// Package omni provides an Echo route middleware for OmniLogs.
package omni

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	core "github.com/MathPro29/Omni-Libs/sdk/go"
	"github.com/labstack/echo/v4"
)

type event struct {
	data map[string]any
}

const maxCapturedBody = 64 << 10

var (
	once   sync.Once
	queue  chan event
	config core.Config
)

// Middleware records safe route metadata without changing the route response.
// Set OMNILOGS_BASE_URL, OMNILOGS_API_KEY, OMNILOGS_ENVIRONMENT_CODE and OMNILOGS_QUEUE_KEY before registering routes.
func Middleware() echo.MiddlewareFunc {
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
		queue = make(chan event, 128)
		go func() {
			for item := range queue {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := core.FnAdd(ctx, config, item.data)
				cancel()
				if err != nil {
					log.Printf("omnilogs: send failed: %v", err)
				}
			}
		}()
	})
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			request := c.Request()
			body, readErr := io.ReadAll(io.LimitReader(request.Body, maxCapturedBody+1))
			request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), request.Body))
			if readErr != nil {
				log.Printf("omnilogs: request body capture failed: %v", readErr)
				body = nil
			}
			truncated := len(body) > maxCapturedBody
			if truncated {
				body = body[:maxCapturedBody]
			}
			err := next(c)
			if queue == nil {
				return err
			}
			status := c.Response().Status
			if err != nil {
				status = http.StatusInternalServerError
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					status = httpErr.Code
				}
			}
			path := request.URL.Path
			reqDetails := map[string]any{
				"method": c.Request().Method, "host": request.Host, "url": request.URL.String(),
				"path": path, "route": c.Path(), "query": safeQuery(request.URL.Query()),
				"headers": map[string]string{"content_type": request.Header.Get("Content-Type"), "user_agent": request.UserAgent()},
				"body":    safeBody(body), "client_ip": c.RealIP(), "user_agent": request.UserAgent(),
			}
			if truncated {
				reqDetails["body_truncated"] = true
			}
			data := map[string]any{
				"service": os.Getenv("OMNILOGS_SERVICE"), "level": logLevel(status), "log_level": logLevel(status),
				"captured_at": time.Now().UTC().Format(time.RFC3339Nano), "request": reqDetails,
				"response": map[string]any{"status_code": status},
				"method":   request.Method, "path": path, "route": c.Path(), "status_code": status,
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
			select {
			case queue <- item:
			default:
				log.Printf("omnilogs: queue full; event dropped")
			}
			return err
		}
	}
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

func safeBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(body, &value) != nil {
		return "[omitted: non-JSON body]"
	}
	return maskSecrets(value)
}

func maskSecrets(value any) any {
	switch item := value.(type) {
	case map[string]any:
		for key, value := range item {
			normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			switch normalized {
			case "password", "token", "accesstoken", "refreshtoken", "sessiontoken", "authorization", "cookie", "secret", "secretkey", "clientsecret", "apikey", "credential":
				item[key] = "[REDACTED]"
			default:
				item[key] = maskSecrets(value)
			}
		}
	case []any:
		for i := range item {
			item[i] = maskSecrets(item[i])
		}
	}
	return value
}

func safeQuery(query map[string][]string) map[string]any {
	result := make(map[string]any, len(query))
	for key, values := range query {
		masked := maskSecrets(map[string]any{key: values})
		result[key] = masked.(map[string]any)[key]
	}
	return result
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
