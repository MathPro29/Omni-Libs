// Package omni provides an Echo route middleware for OmniLogs.
package omni

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	core "github.com/MathPro29/Omni-Libs/sdk/go"
	"github.com/labstack/echo/v4"
)

type event struct {
	data map[string]any
}

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
			item := event{data: map[string]any{
				"service":     os.Getenv("OMNILOGS_SERVICE"),
				"level":       logLevel(status),
				"method":      c.Request().Method,
				"route":       c.Path(),
				"status_code": status,
				"duration_ms": time.Since(start).Milliseconds(),
			}}
			select {
			case queue <- item:
			default:
				log.Printf("omnilogs: queue full; event dropped")
			}
			return err
		}
	}
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
