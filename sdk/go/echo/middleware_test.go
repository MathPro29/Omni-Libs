package omni

import (
	"context"
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
	batches := make(chan map[string]any, 2)
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
	e.GET("/safe/:id", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"token": "response-secret"})
	}, Middleware())
	e.POST("/orders/:id", func(c echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil || !strings.Contains(string(body), `"name":"facility"`) {
			t.Errorf("handler did not receive original body: %v", err)
		}
		c.Set("logger", map[string]any{"adminsId": 345, "projectsId": 14})
		return c.JSON(http.StatusCreated, map[string]any{"id": 77, "token": "response-secret"})
	}, MiddlewareWithPayload())
	safeRequest := httptest.NewRequest(http.MethodGet, "/safe/path-secret?pin=query-secret", nil)
	safeRequest.Header.Set("Authorization", "Bearer header-secret")
	e.ServeHTTP(httptest.NewRecorder(), safeRequest)
	select {
	case batch := <-batches:
		encoded, _ := json.Marshal(batch)
		if strings.Contains(string(encoded), "path-secret") || strings.Contains(string(encoded), "query-secret") ||
			strings.Contains(string(encoded), "response-secret") || strings.Contains(string(encoded), "header-secret") {
			t.Fatalf("default middleware exposed request data: %s", encoded)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for metadata event")
	}
	request := httptest.NewRequest(http.MethodPost, "/orders/123?source=mobile&pin=1234", strings.NewReader(`{"name":"facility","password":"do-not-log","private_key":"private-secret"}`))
	request.Header.Set("Authorization", "Bearer header-only-secret")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("route returned %d", response.Code)
	}
	select {
	case batch := <-batches:
		encoded, _ := json.Marshal(batch)
		if strings.Contains(string(encoded), "header-only-secret") || batch["queue_key"] != "assetwise" {
			t.Fatalf("unsafe or invalid batch: %s", encoded)
		}
		logs, ok := batch["logs"].([]any)
		if !ok || len(logs) != 1 {
			t.Fatalf("invalid logs: %v", batch["logs"])
		}
		data, ok := logs[0].(map[string]any)["data"].(map[string]any)
		if !ok || data["route"] != "/orders/:id" || data["status_code"] != float64(http.StatusCreated) || data["environment"] != "UAT" {
			t.Fatalf("invalid route metadata: %v", data)
		}
		requestData, ok := data["request"].(map[string]any)
		if !ok || requestData["method"] != http.MethodPost || requestData["path"] != "/orders/123" {
			t.Fatalf("missing request details: %v", data["request"])
		}
		body, ok := requestData["body"].(map[string]any)
		if !ok || body["name"] != "facility" || body["password"] != "do-not-log" || body["private_key"] != "private-secret" {
			t.Fatalf("request body was not preserved")
		}
		if requestData["url"] != "/orders/123" {
			t.Fatalf("raw query leaked into URL")
		}
		query, ok := requestData["query"].(map[string]any)
		if !ok || len(query) != 2 {
			t.Fatalf("query was not preserved")
		}
		responseData, ok := data["response"].(map[string]any)
		if !ok || responseData["status_code"] != float64(http.StatusCreated) {
			t.Fatalf("missing response: %v", data["response"])
		}
		responseBody, ok := responseData["body"].(map[string]any)
		if !ok || responseBody["id"] != float64(77) || responseBody["token"] != "response-secret" {
			t.Fatalf("response body was not preserved")
		}
		if data["user_id"] != float64(345) || data["project_id"] != float64(14) {
			t.Fatalf("missing actor/project metadata: %v", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OmniLogs event")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if pending, dropped, failed := Stats(); pending != 0 || dropped != 0 || failed != 0 {
		t.Fatalf("unexpected delivery stats: pending=%d dropped=%d failed=%d", pending, dropped, failed)
	}
}

func TestBodyCaptureWriterBoundsWithoutChangingResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &bodyCaptureWriter{ResponseWriter: recorder}
	content := strings.Repeat("a", maxCapturedBody+10)
	n, err := writer.Write([]byte(content))
	if err != nil || n != len(content) || recorder.Body.String() != content ||
		len(writer.body) != maxCapturedBody || !writer.truncated {
		t.Fatalf("capture changed response: n=%d err=%v captured=%d truncated=%v", n, err, len(writer.body), writer.truncated)
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
