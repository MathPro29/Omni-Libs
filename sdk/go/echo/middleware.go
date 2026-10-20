// Package omni provides an Echo route middleware for OmniLogs.
package omni

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/MathPro29/Omni-Libs/sdk/go"
	"github.com/labstack/echo/v4"
)

type event struct {
	data map[string]any
}

const maxCapturedBody = 64 << 10

var (
	once         sync.Once
	queue        chan event
	config       core.Config
	queueMu      sync.Mutex
	closed       bool
	workers      sync.WaitGroup
	dropped      atomic.Uint64
	sendFailures atomic.Uint64
	pending      atomic.Int64
)

// Middleware records route metadata without changing the route response.
// Set OMNILOGS_BASE_URL, OMNILOGS_API_KEY, OMNILOGS_ENVIRONMENT_CODE and OMNILOGS_QUEUE_KEY before registering routes.
func Middleware() echo.MiddlewareFunc {
	return middleware(false)
}

// MiddlewareWithPayload also records JSON bodies and query values. Use only when the receiving OmniLogs service protects raw payloads.
func MiddlewareWithPayload() echo.MiddlewareFunc {
	return middleware(true)
}

func middleware(includePayload bool) echo.MiddlewareFunc {
	once.Do(func() {
		config = core.Config{
			BaseURL:         os.Getenv("OMNILOGS_BASE_URL"),
			APIKey:          os.Getenv("OMNILOGS_API_KEY"),
			EnvironmentCode: os.Getenv("OMNILOGS_ENVIRONMENT_CODE"),
			QueueKey:        os.Getenv("OMNILOGS_QUEUE_KEY"),
		}
		if _, err := core.New(config); err != nil || config.QueueKey == "" {
			log.Printf("omnilogs: middleware disabled: invalid configuration")
			return
		}
		queue = make(chan event, 1024)
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range queue {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := core.FnAdd(ctx, config, item.data)
				cancel()
				pending.Add(-1)
				if err != nil {
					sendFailures.Add(1)
					log.Printf("omnilogs: send failed: %v", err)
				}
			}
		}()
	})
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if queue == nil {
				return next(c)
			}
			start := time.Now()
			request := c.Request()
			response := c.Response()
			var capture *bodyCaptureWriter
			var body []byte
			var truncated bool
			if includePayload {
				capture = &bodyCaptureWriter{ResponseWriter: response.Writer}
				response.Writer = capture
				var readErr error
				body, readErr = io.ReadAll(io.LimitReader(request.Body, maxCapturedBody+1))
				originalBody := request.Body
				request.Body = &replayBody{Reader: io.MultiReader(bytes.NewReader(body), originalBody), Closer: originalBody}
				if readErr != nil {
					log.Printf("omnilogs: request body capture failed: %v", readErr)
					body = nil
				}
				truncated = len(body) > maxCapturedBody
				if truncated {
					body = body[:maxCapturedBody]
				}
			}
			err := next(c)
			status := c.Response().Status
			if err != nil {
				status = http.StatusInternalServerError
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					status = httpErr.Code
				}
			}
			path := c.Path()
			reqDetails := map[string]any{
				"method": request.Method, "path": path, "route": path,
			}
			responseDetails := map[string]any{"status_code": status}
			if includePayload {
				path = request.URL.Path
				reqDetails["host"] = request.Host
				reqDetails["url"] = path
				reqDetails["path"] = path
				reqDetails["query"] = request.URL.Query()
				reqDetails["headers"] = map[string]string{"content_type": request.Header.Get("Content-Type"), "user_agent": request.UserAgent()}
				reqDetails["body"] = jsonBody(body)
				reqDetails["client_ip"] = c.RealIP()
				reqDetails["user_agent"] = request.UserAgent()
				if truncated {
					reqDetails["body_truncated"] = true
				}
				responseDetails["headers"] = map[string]string{"content_type": response.Header().Get("Content-Type")}
				responseDetails["body"] = jsonBody(capture.body)
				responseDetails["body_truncated"] = capture.truncated
			}
			data := map[string]any{
				"service": os.Getenv("OMNILOGS_SERVICE"), "level": logLevel(status), "log_level": logLevel(status),
				"captured_at": time.Now().UTC().Format(time.RFC3339Nano), "request": reqDetails,
				"environment": config.EnvironmentCode,
				"response":    responseDetails,
				"method":      request.Method, "path": path, "route": c.Path(), "status_code": status,
				"duration_ms": time.Since(start).Milliseconds(),
			}
			if claims := c.Get("claims"); claims != nil {
				if id := structField(claims, "Uuid"); id != nil {
					data["user_id"] = id
				}
				if id := structField(claims, "ProjectsId"); id != nil {
					data["project_id"] = id
				}
			}
			if logger, ok := c.Get("logger").(map[string]any); ok {
				if id := logger["adminsId"]; id != nil {
					data["user_id"] = id
				}
				if id := logger["projectsId"]; id != nil {
					data["project_id"] = id
				}
			}
			item := event{data: data}
			queueMu.Lock()
			if !closed {
				pending.Add(1)
				select {
				case queue <- item:
				default:
					pending.Add(-1)
					dropped.Add(1)
					log.Printf("omnilogs: queue full; event dropped")
				}
			} else {
				dropped.Add(1)
			}
			queueMu.Unlock()
			return err
		}
	}
}

// Shutdown drains queued events after the HTTP server has stopped accepting requests.
func Shutdown(ctx context.Context) error {
	queueMu.Lock()
	if queue != nil && !closed {
		closed = true
		close(queue)
	}
	queueMu.Unlock()
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats reports pending events, events dropped before sending and failed sends.
func Stats() (pendingEvents int64, droppedEvents, failedSends uint64) {
	return pending.Load(), dropped.Load(), sendFailures.Load()
}

type replayBody struct {
	io.Reader
	io.Closer
}

type bodyCaptureWriter struct {
	http.ResponseWriter
	body      []byte
	truncated bool
}

func (w *bodyCaptureWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	remaining := maxCapturedBody - len(w.body)
	if n > remaining {
		w.truncated = true
	}
	if remaining > 0 && n > 0 {
		captureLen := n
		if captureLen > remaining {
			captureLen = remaining
		}
		w.body = append(w.body, p[:captureLen]...)
	}
	return n, err
}

func (w *bodyCaptureWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *bodyCaptureWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *bodyCaptureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
func (w *bodyCaptureWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func structField(value any, name string) any {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.IsValid() && v.Kind() == reflect.Struct {
		field := v.FieldByName(name)
		if field.IsValid() && field.CanInterface() {
			if field.Kind() == reflect.Pointer && !field.IsNil() {
				return field.Elem().Interface()
			}
			if field.Kind() != reflect.Pointer {
				return field.Interface()
			}
		}
	}
	return nil
}

func jsonBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(body, &value) != nil {
		return "[omitted: non-JSON body]"
	}
	return value
}

func logLevel(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "ERROR"
	case status >= http.StatusBadRequest:
		return "WARN"
	default:
		return "INFO"
	}
}
