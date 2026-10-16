# Client สำหรับ Go

## ใช้กับ Echo แบบใส่ใน route

ตั้ง `OMNILOGS_BASE_URL`, `OMNILOGS_API_KEY`, `OMNILOGS_ENVIRONMENT_CODE`, `OMNILOGS_QUEUE_KEY` และ `OMNILOGS_SERVICE` ใน environment ก่อนลงทะเบียน routes แล้วเพิ่ม middleware ใน route ที่ต้องการ:

```go
import omni "github.com/MathPro29/Omni-Libs/sdk/go/echo"

api.GET("/getOne", banks.FnGetOne, omni.Middleware())
```

Middleware ส่งเฉพาะ method, route template, status, duration และชื่อ service โดยไม่ส่ง request/response body หรือ headers การส่งทำใน worker ที่มีคิวจำกัดเพื่อไม่ให้ route รอ OmniLogs; หากคิวเต็มจะข้าม event และบันทึกข้อผิดพลาดใน application log

## ส่ง event เอง

หลังอัปโหลด commit ที่ประกาศ module path นี้แล้ว ติดตั้งด้วย commit hash หรือ Git tag `sdk/go/v0.1.0`:

```sh
go get github.com/MathPro29/Omni-Libs/sdk/go@COMMIT_OR_TAG
```

ตัวอย่างเรียกใช้:

```go
import omni "github.com/MathPro29/Omni-Libs/sdk/go"

result, err := omni.FnAdd(ctx, omni.Config{
    BaseURL: os.Getenv("OMNILOGS_BASE_URL"),
    APIKey: os.Getenv("OMNILOGS_API_KEY"),
    EnvironmentCode: os.Getenv("OMNILOGS_ENVIRONMENT_CODE"),
    QueueKey: "orders-api",
}, map[string]any{"message": "order created", "level": "INFO"})
if err != nil { return err }
_ = result.BatchID
```

`FnAdd` ส่ง event เดียวและสร้าง `idempotency_key` ใหม่ทุกครั้ง หากต้อง retry batch เดิมด้วย key เดิมหรือส่งหลาย event ให้ใช้ `Client.Send` แทน ใช้ context ที่มี timeout สั้นสำหรับส่งผ่าน background worker และเก็บ API key ใน secret store

ดูวิธีทดลองใช้งานผ่าน consumer ใน [คู่มือ SDK](../README.md#ลองใช้งานเหมือนเป็นทีมผู้พัฒนา-service)
