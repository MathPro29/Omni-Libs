package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	omni "github.com/MathPro29/Omni-Libs/sdk/go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := omni.FnAdd(ctx, omni.Config{
		BaseURL:         os.Getenv("OMNILOGS_BASE_URL"),
		APIKey:          os.Getenv("OMNILOGS_API_KEY"),
		EnvironmentCode: os.Getenv("OMNILOGS_ENVIRONMENT_CODE"),
		QueueKey:        os.Getenv("OMNILOGS_QUEUE_KEY"),
	}, map[string]any{
		"service": os.Getenv("OMNILOGS_SERVICE"),
		"level":   "INFO",
		"message": "order created",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Log queued in OmniLogs: batch_id=%s\n", result.BatchID)
}
