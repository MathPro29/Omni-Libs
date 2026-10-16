package omni

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config contains the server-side credentials and connection settings.
type Config struct {
	// BaseURL is the OmniLogs HTTP(S) origin without an API path.
	BaseURL string
	// APIKey is a Product API key with LOG_INGEST_CREATE permission.
	APIKey string
	// EnvironmentCode is the active Environment code assigned to the API key.
	EnvironmentCode string
	// HTTPClient optionally replaces the default client, which has a five-second timeout.
	HTTPClient *http.Client
}

// Client sends batches to the OmniLogs ingestion API.
type Client struct {
	baseURL         string
	apiKey          string
	environmentCode string
	httpClient      *http.Client
}

// Log is one event in an ingestion batch.
type Log struct {
	// SequenceNo is the one-based position of this event in the batch.
	SequenceNo int `json:"sequence_no"`
	// SourceType identifies the source class, such as SERVICE.
	SourceType string `json:"source_type"`
	// SourcePlatform identifies the source platform, such as HTTP.
	SourcePlatform string `json:"source_platform"`
	// Data is the JSON object to store and process.
	Data map[string]any `json:"data"`
	// CustomFields contains optional classification values.
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

// Batch is a canonical group of log events sent in one API request.
type Batch struct {
	// QueueKey is the logical queue name configured for the sender.
	QueueKey string `json:"queue_key"`
	// SourceType identifies the source class, such as SERVICE.
	SourceType string `json:"source_type"`
	// SourcePlatform identifies the source platform, such as HTTP.
	SourcePlatform string `json:"source_platform"`
	// EnvironmentCode must match the value configured on the Client.
	EnvironmentCode string `json:"environment_code"`
	// IdempotencyKey is a unique key for this logical batch and must be reused only for its retries.
	IdempotencyKey string `json:"idempotency_key"`
	// Logs contains one or more events with ordered sequence numbers starting at one.
	Logs []Log `json:"logs"`
}

// BatchResult describes the server's queue acknowledgement.
type BatchResult struct {
	// BatchID is the identifier assigned by OmniLogs.
	BatchID string `json:"batch_id"`
	// Status is the batch state returned by OmniLogs, usually QUEUED on ingestion.
	Status string `json:"status"`
}

// APIError describes a non-success HTTP response from OmniLogs.
type APIError struct {
	// Status is the HTTP response status code.
	Status int
	// Code is the stable error code returned by OmniLogs.
	Code string
	// Message is the error message returned by OmniLogs.
	Message string
}

// Error formats the API error without including the API key or request payload.
func (e *APIError) Error() string {
	return fmt.Sprintf("omnilogs: HTTP %d: %s: %s", e.Status, e.Code, e.Message)
}

// New validates the configuration and creates an OmniLogs client.
func New(config Config) (*Client, error) {
	u, err := url.Parse(config.BaseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("omnilogs: BaseURL must be an HTTP(S) origin")
	}
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.EnvironmentCode) == "" {
		return nil, errors.New("omnilogs: APIKey and EnvironmentCode are required")
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	copyClient := *httpClient
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("omni: redirects are not allowed")
	}
	return &Client{strings.TrimRight(config.BaseURL, "/"), config.APIKey, config.EnvironmentCode, &copyClient}, nil
}

// Send sends a batch and returns when OmniLogs queues it; indexing happens later.
func (c *Client) Send(ctx context.Context, batch Batch) (BatchResult, error) {
	if batch.QueueKey == "" || batch.SourceType == "" || batch.SourcePlatform == "" || batch.IdempotencyKey == "" || len(batch.Logs) == 0 {
		return BatchResult{}, errors.New("omnilogs: queue key, source type, platform, idempotency key and logs are required")
	}
	for i, log := range batch.Logs {
		if log.SequenceNo != i+1 || log.SourceType == "" || log.SourcePlatform == "" || log.Data == nil {
			return BatchResult{}, fmt.Errorf("omnilogs: invalid log at index %d", i)
		}
	}
	if batch.EnvironmentCode == "" {
		batch.EnvironmentCode = c.environmentCode
	}
	if batch.EnvironmentCode != c.environmentCode {
		return BatchResult{}, errors.New("omnilogs: batch environment differs from client environment")
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return BatchResult{}, fmt.Errorf("omnilogs: encode batch: %w", err)
	}
	var result BatchResult
	err = c.request(ctx, http.MethodPost, "/api/v1/ingest/logs", body, &result)
	return result, err
}

// Status retrieves a batch state for diagnostics; callers should not poll after every Send.
func (c *Client) Status(ctx context.Context, batchID string) (BatchResult, error) {
	if strings.TrimSpace(batchID) == "" {
		return BatchResult{}, errors.New("omnilogs: batch ID is required")
	}
	var result BatchResult
	err := c.request(ctx, http.MethodGet, "/api/v1/ingest/batches/"+url.PathEscape(batchID), nil, &result)
	return result, err
}

func (c *Client) request(ctx context.Context, method, path string, body []byte, result *BatchResult) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Environment-Code", c.environmentCode)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("omnilogs: decode HTTP %d response: %w", resp.StatusCode, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{resp.StatusCode, envelope.Error.Code, envelope.Error.Message}
	}
	if err := json.Unmarshal(envelope.Data, result); err != nil || result.BatchID == "" {
		return errors.New("omnilogs: invalid batch response")
	}
	return nil
}
