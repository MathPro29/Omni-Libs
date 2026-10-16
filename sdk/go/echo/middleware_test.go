package omni

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestMiddlewareSendsRouteMetadataWithoutRequestSecrets(t *testing.T) {
	if os.Getenv("OMNILOGS_LIVE_TEST") == "1" {
		t.Skip("running the live middleware test")
	}
	batches := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/ingest/logs" ||
			r.Header.Get("X-API-Key") != "test-key" || r.Header.Get("X-Environment-Code") != "UAT" {
			t.Errorf("unexpected OmniLogs request: %s %s", r.Method, r.URL.Path)
		}
		var batch map[string]any
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
		}
		batches <- batch
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"batch_id":"mock-batch","status":"QUEUED"}}`))
	}))
	defer server.Close()
	t.Setenv("OMNILOGS_BASE_URL", server.URL)
	t.Setenv("OMNILOGS_API_KEY", "test-key")
	t.Setenv("OMNILOGS_ENVIRONMENT_CODE", "UAT")
	t.Setenv("OMNILOGS_QUEUE_KEY", "assetwise")
	t.Setenv("OMNILOGS_SERVICE", "assetwise-core-api")

	e := echo.New()
	e.POST("/orders/:id", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, Middleware())
	request := httptest.NewRequest(http.MethodPost, "/orders/123", strings.NewReader(`{"password":"do-not-log"}`))
	request.Header.Set("Authorization", "Bearer do-not-log")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("route returned %d", response.Code)
	}
	select {
	case batch := <-batches:
		encoded, _ := json.Marshal(batch)
		if strings.Contains(string(encoded), "do-not-log") || batch["queue_key"] != "assetwise" {
			t.Fatalf("unsafe or invalid batch: %s", encoded)
		}
		logs, ok := batch["logs"].([]any)
		if !ok || len(logs) != 1 {
			t.Fatalf("invalid logs: %v", batch["logs"])
		}
		data, ok := logs[0].(map[string]any)["data"].(map[string]any)
		if !ok || data["route"] != "/orders/:id" || data["status_code"] != float64(http.StatusNoContent) {
			t.Fatalf("invalid route metadata: %v", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OmniLogs event")
	}
}

func TestMiddlewareLive(t *testing.T) {
	if os.Getenv("OMNILOGS_LIVE_TEST") != "1" {
		t.Skip("set OMNILOGS_LIVE_TEST=1 with test credentials")
	}
	target := strings.TrimRight(os.Getenv("OMNILOGS_BASE_URL"), "/")
	if target == "" || os.Getenv("OMNILOGS_API_KEY") == "" {
		t.Fatal("OmniLogs test credentials are required")
	}
	batchIDs := make(chan string, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := http.NewRequestWithContext(r.Context(), r.Method, target+r.URL.Path, r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		request.Header = r.Header.Clone()
		response, err := (&http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if response.StatusCode == http.StatusCreated {
			var result struct {
				Data struct {
					BatchID string `json:"batch_id"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &result); err == nil && result.Data.BatchID != "" {
				batchIDs <- result.Data.BatchID
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(body)
	}))
	defer proxy.Close()
	t.Setenv("OMNILOGS_BASE_URL", proxy.URL)
	e := echo.New()
	e.POST("/verify", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, Middleware())
	response := httptest.NewRecorder()
	e.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/verify", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("route returned %d", response.Code)
	}
	select {
	case id := <-batchIDs:
		t.Logf("OmniLogs queued batch %s", id)
	case <-time.After(10 * time.Second):
		t.Fatal("no batch acknowledgement from OmniLogs")
	}
}
