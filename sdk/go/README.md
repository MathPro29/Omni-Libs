# Client สำหรับ Go

## ใช้กับ Echo แบบใส่ใน route

ตั้ง `OMNILOGS_BASE_URL`, `OMNILOGS_API_KEY`, `OMNILOGS_ENVIRONMENT_CODE`, `OMNILOGS_QUEUE_KEY` และ `OMNILOGS_SERVICE` ใน environment ก่อนลงทะเบียน routes แล้วเพิ่ม middleware ใน route ที่ต้องการ:

```go
import omni "github.com/MathPro29/Omni-Libs/sdk/go/echo"

api.GET("/getOne", banks.FnGetOne, omni.Middleware())
```

`Middleware()` เก็บ method, route template, status, duration และผู้ใช้/โปรเจกต์จาก Echo context โดยไม่ส่ง path จริง, query, headers หรือ body หากต้องเก็บ JSON request/response body และ query ให้ใช้ `omni.MiddlewareWithPayload()` แทน ซึ่งจำกัด body สูงสุดด้านละ 64 KiB และไม่ส่ง Authorization/Cookie headers การเปิดใช้แบบนี้ต้องใช้ HTTPS และยืนยันว่าบริการ OmniLogs ป้องกัน payload ดิบและ mask เอกสารที่ใช้ค้นหาตาม[เกณฑ์ก่อนปล่อย](../RELEASE_CUTOFF.md) ตั้ง field ที่เป็นข้อมูลลับให้ครบก่อนเปิดใช้กับ route จริง

Middleware ส่งแบบ asynchronous ด้วย worker เดียวและคิวในหน่วยความจำ 1,024 รายการ หากคิวเต็มหรือส่งไม่สำเร็จ event อาจหายได้ ใช้ `omni.Stats()` ดูจำนวนค้างคิว ทิ้ง และส่งไม่สำเร็จ หลังหยุดรับ HTTP request ให้เรียก `omni.Shutdown(ctx)` เพื่อรอระบายคิวก่อนปิด process:

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
if err := omni.Shutdown(ctx); err != nil { log.Printf("omnilogs drain: %v", err) }
pending, dropped, failed := omni.Stats()
log.Printf("omnilogs delivery: pending=%d dropped=%d failed=%d", pending, dropped, failed)
```

หากต้องการ log ที่ต้องยืนยันการรับจริง ให้ใช้ `Client.Send` และตรวจผลตอบรับตาม business transaction นั้น คิวของ middleware เป็น best effort

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
