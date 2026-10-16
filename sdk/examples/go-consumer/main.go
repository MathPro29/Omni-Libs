package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	omni "gitlab.dev.nextate.tech/activity-logs/omni-logs/sdk/go"
)

func main() {
	client, err := omni.New(omni.Config{
		BaseURL:         os.Getenv("OMNILOGS_BASE_URL"),
		APIKey:          os.Getenv("OMNILOGS_API_KEY"),
		EnvironmentCode: os.Getenv("OMNILOGS_ENVIRONMENT_CODE"),
	})
	if err != nil {
		log.Fatal(err)
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := client.Send(ctx, omni.Batch{
		QueueKey:        os.Getenv("OMNILOGS_QUEUE_KEY"),
		SourceType:      "SERVICE",
		SourcePlatform:  "HTTP",
		IdempotencyKey:  "orders-service:go:" + id,
		Logs: []omni.Log{{SequenceNo: 1, SourceType: "SERVICE", SourcePlatform: "HTTP", Data: map[string]any{
			"service": os.Getenv("OMNILOGS_SERVICE"), "level": "INFO",
			"message": "order created", "request_id": id,
		}}},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Log queued in OmniLogs: batch_id=%s\n", result.BatchID)
}
