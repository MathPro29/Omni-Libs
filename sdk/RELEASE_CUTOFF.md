# OmniLogs SDK release cutoff

สถานะ: **ยังไม่รับรองสำหรับ Production**

Repository นี้กำหนด protocol กลางและมี client สำหรับ Go, JavaScript และ Dart. ระบบที่ส่ง log ต้องทำตาม `PROTOCOL.md`; บริการ OmniLogs ที่รับ log ต้องรักษา contract เดียวกัน. การตัดสินใจปล่อย SDK ไม่ผูกกับ source code ของ backend ของผู้ใช้ SDK รายใด

## เกณฑ์ก่อนปล่อย

1. ทดสอบ client ทุกภาษากับ mock server ว่าส่ง path, headers, payload และ idempotency key ตรงตาม `PROTOCOL.md`; ตรวจ timeout, non-2xx, malformed response และ redirect
2. ทดสอบกับ OmniLogs staging โดยใช้ข้อมูลสังเคราะห์: ingest ได้ `201` และ `batch_id`, batch ไปถึง `COMPLETED`, ค้นหา event ได้; ส่ง batch เดิมซ้ำพร้อมกันต้องได้ `batch_id` เดิมโดยไม่เพิ่ม log ซ้ำ และ key เดิมที่ใช้กับเนื้อหาต่างกันต้องถูกปฏิเสธ; ตรวจกรณี auth, scope และ payload ผิดรูปแบบ
3. บริการ OmniLogs ที่รับ log ต้องยืนยันว่า payload ดิบใน queue, failure record และ archive ถูกป้องกันตามข้อกำหนดความปลอดภัย; การเปิดดู raw ต้องตรวจสิทธิ์และบันทึก audit
4. ทดสอบ Go middleware ขณะคิวเต็ม, ส่งล้มเหลว และ shutdown; ระบุชัดว่าเป็น best effort และใช้ `Client.Send` สำหรับ event ที่ห้ามหาย
5. Pin commit หรือ tag ที่ทดสอบแล้วใน consumer ก่อนใช้งาน Production

## ผลที่ตรวจใน repository นี้

- Go: `go test -race ./...` ผ่านใน `sdk/go` (2026-10-20); `Middleware()` เก็บเฉพาะ route metadata และการเก็บ payload ต้องเลือก `MiddlewareWithPayload()` ชัดเจน
- JavaScript: `node --test index.test.js`, `node --check index.js` และ `npm pack --dry-run --json` ผ่าน รวม timeout กับ mock server
- Dart: `dart run test/client_check.dart` และ `dart analyze` ผ่านโดยใช้ Dart container
- ทดสอบ Assetwise route จริงกับ OmniLogs local แล้ว; batch ไปถึง `COMPLETED`, ค้นหา synthetic event ได้โดยไม่พบ `password` หรือ `pin` ดิบ, retry พร้อมกัน 8 ครั้งได้ batch ID เดียวและ payload ต่างกันได้ HTTP `409`
- OmniLogs backend `go test ./...`, `go build ./...`, OpenAPI validate/check ผ่าน; คิว PostgreSQL เดิม 1,188 รายการเข้ารหัสครบ และลบเฉพาะ NATS messages ที่ ack แล้วจาก local stream
- ยังไม่มีหลักฐานจาก OmniLogs staging ตามข้อ 2 และ 3; Assetwise ยัง pin Go SDK commit เก่าและทดสอบผ่าน Go workspace ชั่วคราว

ห้ามตีความ HTTP `201` ว่า event ค้นหาได้แล้ว: ผลนี้ยืนยันเฉพาะว่าเข้าคิว. Go Echo middleware เก็บ event ในคิวหน่วยความจำ จึงอาจทิ้ง event เมื่อคิวเต็ม, ส่งไม่สำเร็จ หรือ process หยุดกะทันหัน
