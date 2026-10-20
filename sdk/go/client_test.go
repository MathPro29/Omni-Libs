package omni

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendFollowsProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/ingest/logs" ||
			r.Header.Get("X-API-Key") != "test-key" || r.Header.Get("X-Environment-Code") != "UAT" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var batch Batch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
		}
		if batch.EnvironmentCode != "UAT" || batch.IdempotencyKey != "orders:1" || len(batch.Logs) != 1 {
			t.Errorf("unexpected batch: %+v", batch)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"batch_id":"batch-1","status":"QUEUED"}}`))
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, APIKey: "test-key", EnvironmentCode: "UAT"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Send(context.Background(), Batch{
		QueueKey: "orders", SourceType: "SERVICE", SourcePlatform: "HTTP", IdempotencyKey: "orders:1",
		Logs: []Log{{SequenceNo: 1, SourceType: "SERVICE", SourcePlatform: "HTTP", Data: map[string]any{"message": "created"}}},
	})
	if err != nil || result.BatchID != "batch-1" {
		t.Fatalf("send result: %+v, %v", result, err)
	}
}
