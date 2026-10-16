# Protocol สำหรับส่ง log เข้า OmniLogs

เอกสารนี้ใช้ได้กับโปรแกรมฝั่ง server ทุกภาษาที่ส่ง HTTPS และ JSON ได้ ส่วน SDK สำหรับ Go, JS และ Dart เป็นตัวช่วยเรียก protocol เดียวกัน

## ข้อมูลที่ต้องได้รับจากผู้ดูแล OmniLogs

- API origin เช่น `https://logs.example.com` โดยไม่เติม `/api`
- Product API key ที่มีสิทธิ์ `LOG_INGEST_CREATE`
- รหัส Environment ที่แน่นอน เช่น `UAT` และต้องอยู่ใน scope ของ key

## คำขอพื้นฐาน

ส่ง `POST {origin}/api/v1/ingest/logs` พร้อม header และ JSON body ตามตัวอย่าง ค่า `idempotency_key` ต้องอยู่ใน JSON body สร้างให้ไม่ซ้ำกันทั่วระบบต่อ batch และใส่ชื่อ product/service นำหน้า หาก retry batch เดิมให้ใช้ค่าเดิม ปัจจุบัน server ยังไม่ใช้ HTTP header `Idempotency-Key` เพื่อป้องกัน batch ซ้ำ

```http
POST /api/v1/ingest/logs HTTP/1.1
Content-Type: application/json
Accept: application/json
X-API-Key: <product-api-key>
X-Environment-Code: UAT
```

```json
{
  "queue_key": "orders-api",
  "source_type": "SERVICE",
  "source_platform": "HTTP",
  "environment_code": "UAT",
  "idempotency_key": "orders:orders-api:req-123",
  "logs": [
    {
      "sequence_no": 1,
      "source_type": "SERVICE",
      "source_platform": "HTTP",
      "data": {
        "timestamp": "2026-10-15T00:00:00Z",
        "level": "INFO",
        "service": "orders-api",
        "message": "order created",
        "request_id": "req-123"
      }
    }
  ]
}
```

ตัวอย่างส่งด้วย `curl` ตั้ง environment variables ใน shell ก่อนเรียก:

```sh
curl --fail-with-body --request POST "$OMNILOGS_BASE_URL/api/v1/ingest/logs" \
  --header "Content-Type: application/json" \
  --header "Accept: application/json" \
  --header "X-API-Key: $OMNILOGS_API_KEY" \
  --header "X-Environment-Code: $OMNILOGS_ENVIRONMENT_CODE" \
  --data "{\"queue_key\":\"orders-api\",\"source_type\":\"SERVICE\",\"source_platform\":\"HTTP\",\"environment_code\":\"$OMNILOGS_ENVIRONMENT_CODE\",\"idempotency_key\":\"orders:orders-api:req-123\",\"logs\":[{\"sequence_no\":1,\"source_type\":\"SERVICE\",\"source_platform\":\"HTTP\",\"data\":{\"level\":\"INFO\",\"message\":\"order created\"}}]}"
```

ค่า environment ใน header, JSON และ API key ต้องตรงกัน `data` ต้องเป็น JSON object หากส่งหลายรายการ ให้เรียง `sequence_no` ตั้งแต่ 1 ถึงจำนวนรายการ ใช้ HTTP timeout ที่สั้นและอย่าใส่ API key, credential, cookie หรือข้อมูลส่วนบุคคลลงใน log

## การอ่านผลตอบกลับ

HTTP `201` พร้อม `data.batch_id` หมายถึง batch ถูกเพิ่มเข้าคิวแล้ว:

```json
{"success":true,"data":{"batch_id":"<uuid>","status":"QUEUED"}}
```

การ index เกิดภายหลัง หากต้องตรวจสอบระหว่างวิเคราะห์ปัญหา ให้เรียก `GET /api/v1/ingest/batches/{batch_id}` ด้วย API key เดิม ไม่ควร poll หลังการส่งทุกครั้ง เมื่อได้ non-2xx ให้อ่าน `error.code` และ `error.message` ซึ่งเป็นข้อความสำหรับ client ส่วน `400`, `401`, `403` และ `413` ต้องแก้ config หรือ payload ก่อน ส่วน `429`, `5xx` และ network timeout อาจเป็นปัญหาชั่วคราว แต่ยังไม่ควร retry อัตโนมัติจนกว่า server จะบังคับ idempotency ที่ป้องกัน race condition และแยกขอบเขตตาม Product

## วิธีทดสอบการเชื่อมต่อ

ทดสอบ 3 ขั้น: ตรวจรูปแบบ request ในเครื่อง, ส่ง smoke test ไป local หรือ staging แล้วตรวจการประมวลผล ห้ามใช้ API key หรือ Environment จริงสำหรับสองขั้นแรก

### 1. ตรวจการสร้าง request โดยไม่ต่อ OmniLogs

ตั้ง local mock HTTP server แล้วให้ client เรียก server นั้น จากนั้นตรวจว่า request มี:

- `POST /api/v1/ingest/logs`
- `X-API-Key`, `X-Environment-Code`, `Accept: application/json` และ JSON content type
- ชื่อ field ตามตัวอย่าง และ `logs` ที่มีอย่างน้อยหนึ่งรายการ
- `idempotency_key` ค่าเดิมเมื่อส่งซ้ำ batch เดิม

ใช้ HTTP test server/mock มาตรฐานของภาษา ขั้นตอนนี้ไม่ต้องใช้ credential ของ OmniLogs

### 2. ส่ง smoke test ไป local หรือ staging

สร้าง Product API key สำหรับ Environment ทดสอบ โดย key ต้องมีสิทธิ์ `LOG_INGEST_CREATE` ตั้งตัวแปรด้านล่างจาก secret store หรือ shell แล้วส่ง event ตัวอย่างที่ไม่มีข้อมูลจริง รหัส Environment ใน JSON ต้องตรงกับ header และ scope ของ key

```sh
curl --fail-with-body --request POST "$OMNILOGS_BASE_URL/api/v1/ingest/logs" \
  --header "Content-Type: application/json" \
  --header "Accept: application/json" \
  --header "X-API-Key: $OMNILOGS_API_KEY" \
  --header "X-Environment-Code: $OMNILOGS_ENVIRONMENT_CODE" \
  --data "{\"queue_key\":\"sdk-smoke\",\"source_type\":\"SERVICE\",\"source_platform\":\"HTTP\",\"environment_code\":\"$OMNILOGS_ENVIRONMENT_CODE\",\"idempotency_key\":\"sdk-smoke:$(date +%s)\",\"logs\":[{\"sequence_no\":1,\"source_type\":\"SERVICE\",\"source_platform\":\"HTTP\",\"data\":{\"level\":\"INFO\",\"message\":\"SDK smoke test\"}}]}"
```

ผลที่คาดหวังคือ HTTP `201`, `success: true` และ `data.batch_id` ที่ไม่ว่าง ผลนี้ยืนยันเฉพาะว่า API รับ batch เข้าคิว

ตัวอย่าง consumer ใน `sdk/examples` ใช้ SDK ผ่าน dependency ของแต่ละภาษา และส่ง event จริงหนึ่งรายการเมื่อกำหนด environment variables ข้างต้น รวมทั้ง `OMNILOGS_QUEUE_KEY` และ `OMNILOGS_SERVICE` ใช้ API key ของ local/staging เท่านั้น

### 3. ตรวจการประมวลผล

นำ ID จากผลตอบกลับไปตรวจ status หนึ่งครั้ง:

```sh
curl --fail-with-body "$OMNILOGS_BASE_URL/api/v1/ingest/batches/$BATCH_ID" \
  --header "Accept: application/json" \
  --header "X-API-Key: $OMNILOGS_API_KEY" \
  --header "X-Environment-Code: $OMNILOGS_ENVIRONMENT_CODE"
```

ตรวจว่า batch ไปถึงสถานะ `COMPLETED` และค้นหา event ได้ใน Log Explorer ของ Product และ Environment นั้น หากยังค้างหรือไม่สำเร็จ ให้ตรวจ API/worker logs โดยใช้ `X-Request-ID` (ถ้าส่งไป) และ `batch_id` ไม่ควร poll ทุกครั้งที่ service ส่ง log

### กรณีตรวจ error

บน Environment ทดสอบ ตรวจว่าไม่มี key หรือ key ไม่ถูกต้องตอบ `401`, key ที่ไม่มี `LOG_INGEST_CREATE` ตอบ `403` และ payload ผิดรูปแบบหรือใหญ่เกินกำหนดตอบ `400`/`413` Environment ที่ไม่ตรงกันอาจได้ scope error `400` หรือ `403` ตามจุดที่ตรวจพบ หลีกเลี่ยงการส่ง request ที่ล้มเหลวซ้ำ ๆ เพราะอาจติด rate limit

## ข้อกำหนดสำหรับทุกภาษา

การเชื่อมต่อถือว่าตรงตาม contract เมื่อทำตามข้อกำหนดนี้:

1. ส่ง method, path, headers และชื่อ field ตามตัวอย่าง
2. ใช้ API key และ Environment ของ Product เดียวกัน
3. สร้าง `idempotency_key` ใน body ให้ไม่ซ้ำกันทั่วระบบและคงเดิมเมื่อ retry batch เดิม
4. ถือว่า `201` คือเข้าคิวแล้ว และอ่าน `data.batch_id`
5. ตั้ง timeout และจัดการ network/HTTP errors โดยไม่พิมพ์ key หรือ payload ลับลง log
6. ปกปิดข้อมูลอ่อนไหวก่อน serialize และส่งผ่าน background worker หรือ bounded queue เมื่อ capture จาก request หลัก

ทุกภาษาสามารถสร้าง event ด้วย framework middleware ของตัวเองได้ ส่วน API, queue, policy การปกปิดข้อมูล และ worker ยังคงอยู่ในการดูแลของ OmniLogs
