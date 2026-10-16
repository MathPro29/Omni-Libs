# Client สำหรับ Go

ติดตั้งหลังสร้าง Git tag `sdk/go/v0.1.0`:

```sh
go env -w GOPRIVATE=gitlab.dev.nextate.tech
go get gitlab.dev.nextate.tech/activity-logs/omni-logs/sdk/go@v0.1.0
```

ตัวอย่างเรียกใช้:

```go
import omni "gitlab.dev.nextate.tech/activity-logs/omni-logs/sdk/go"

client, err := omni.New(omni.Config{
    BaseURL: os.Getenv("OMNILOGS_BASE_URL"),
    APIKey: os.Getenv("OMNILOGS_API_KEY"),
    EnvironmentCode: os.Getenv("OMNILOGS_ENVIRONMENT_CODE"),
})
if err != nil { return err }
result, err := client.Send(ctx, omni.Batch{
    QueueKey: "orders-api", SourceType: "SERVICE", SourcePlatform: "HTTP",
    IdempotencyKey: "orders-api:req-123",
    Logs: []omni.Log{{SequenceNo: 1, SourceType: "SERVICE", SourcePlatform: "HTTP",
        Data: map[string]any{"message": "order created", "level": "INFO"}}},
})
if err != nil { return err }
_ = result.BatchID
```

ใช้ context ที่มี timeout สั้นสำหรับส่งผ่าน background worker เก็บ API key ใน secret store

ดูวิธีทดลองใช้งานผ่าน consumer ใน [คู่มือ SDK](../README.md#ลองใช้งานเหมือนเป็นทีมผู้พัฒนา-service)
