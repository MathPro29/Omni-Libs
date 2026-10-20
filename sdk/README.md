# การเชื่อมต่อ OmniLogs ได้ทุกภาษา

OmniLogs รับข้อมูลผ่าน HTTPS และ JSON โปรแกรมฝั่ง server ทุกภาษาที่ส่ง HTTP request และอ่าน JSON ได้จึงเชื่อมต่อโดยตรงตาม[เอกสาร protocol กลาง](PROTOCOL.md) ไม่ต้องติดตั้ง SDK หรือใช้ runtime ของ Go, Node.js หรือ Dart ทั้งสามภาษาเป็นเพียง client สำเร็จรูปสำหรับผู้ที่ต้องการใช้

เมื่อ API ตอบสำเร็จ หมายถึงรับ batch เข้าคิวแล้ว OmniLogs จะประมวลผลและทำให้ค้นหาได้ภายหลัง

## ข้อมูลที่ต้องขอจากผู้ดูแล OmniLogs

- API origin เช่น `https://logs.example.com` โดยไม่เติม `/api`
- Product API key ที่ยังใช้งานได้และมีสิทธิ์ `LOG_INGEST_CREATE`
- รหัส Environment ที่แน่นอนและผูกกับ key

เก็บ key ใน secret store ของ service ฝั่ง server ห้ามฝัง key ใน browser หรือ mobile app

## วิธีเชื่อมต่อจากทุกภาษา

1. ใช้ HTTP client ของภาษานั้นส่ง `POST {origin}/api/v1/ingest/logs` พร้อม JSON และ headers ตาม[คำขอพื้นฐาน](PROTOCOL.md#คำขอพื้นฐาน)
2. ตรวจ HTTP status และอ่าน `data.batch_id` จากผลตอบกลับ `201` ค่านี้หมายถึงระบบรับ batch เข้าคิวแล้ว
3. ตรวจ[วิธีทดสอบการเชื่อมต่อ](PROTOCOL.md#วิธีทดสอบการเชื่อมต่อ) ก่อนใช้กับ service จริง

ไม่มี dependency ที่ต้องติดตั้งจาก repository นี้สำหรับภาษาอื่น และไม่ต้องเชื่อมต่อฐานข้อมูลหรือ NATS ของ OmniLogs โดยตรง

## Client สำเร็จรูปที่มีให้

| วิธี | ติดตั้งหรือเริ่มใช้งาน | ตัวอย่าง |
| --- | --- | --- |
| Go | `go get github.com/MathPro29/Omni-Libs/sdk/go@COMMIT_OR_TAG` | [ตัวอย่าง Go](go/README.md) |
| Node.js | clone repo แล้ว `npm pack` ใน `sdk/js` เพื่อติดตั้ง `.tgz` ในโปรเจกต์ผู้ใช้ | [ตัวอย่าง JS/TS](js/README.md) |
| Dart ฝั่ง server | เพิ่ม Git dependency จาก repo นี้ โดยระบุ `path: sdk/dart` และ ref ที่ pin ไว้ | [ตัวอย่าง Dart](dart/README.md) |

อัปโหลด commit ที่เปลี่ยน Go module path เป็น GitHub ก่อนติดตั้งจาก GitHub แล้วใช้ commit hash หรือ Git tag `sdk/go/v0.1.0` ที่มีอยู่จริง ส่วน Dart ใช้ Git dependency ได้หลัง commit ไฟล์ขึ้น repository

แต่ละ batch ต้องมี `idempotency_key` ที่ไม่ซ้ำกันทั่วระบบ เช่น `product:service:request-id` หาก retry batch เดิมให้ใช้ key เดิม บริการ OmniLogs ต้อง deduplicate แบบ atomic ตาม Product และ Environment ตาม[โปรโตคอลกลาง](PROTOCOL.md) ก่อนเปิด retry อัตโนมัติ ส่วน HTTP header `Idempotency-Key` ไม่ใช้สำหรับ deduplication

Go Echo `Middleware()` เก็บเฉพาะ route metadata; หากเลือกใช้ `MiddlewareWithPayload()` จะส่ง JSON request/response body แบบจำกัดขนาดและ query ไปยัง OmniLogs จึงต้องใช้ HTTPS และตรวจว่าบริการ OmniLogs ป้องกัน payload ดิบใน queue/failure/archive, mask ข้อมูลก่อนแสดง และตรวจสิทธิ์พร้อม audit เมื่อเปิดดู raw ตาม[เกณฑ์ก่อนปล่อย](RELEASE_CUTOFF.md) ตั้ง field ลับให้ครบก่อนใช้กับข้อมูลจริง JavaScript กับ Dart เป็น sender client ที่ผู้ใช้ต้องสร้าง event เอง สำหรับ audit event ที่ห้ามหาย ให้ใช้การส่งแบบรอผลตอบรับ `Client.Send` ตามความเหมาะสม

ทุกภาษาส่งตาม contract เดียวกันได้ ไม่ต้องเชื่อมต่อฐานข้อมูลหรือ NATS ของ OmniLogs

## ลองใช้งานเหมือนเป็นทีมผู้พัฒนา service

มี consumer example ใน `sdk/examples` ที่ติดตั้ง SDK จาก source ในเครื่องและส่ง log ผ่าน client เหมือน service ของทีมจริง ตั้ง `OMNILOGS_BASE_URL`, `OMNILOGS_API_KEY`, `OMNILOGS_ENVIRONMENT_CODE`, `OMNILOGS_QUEUE_KEY` และ `OMNILOGS_SERVICE` ใน environment แล้วรัน `go run .` จาก `examples/go-consumer`, `npm install` และ `node main.mjs` จาก `examples/js-consumer` หรือ `dart pub get` และ `dart run bin/main.dart` จาก `examples/dart-consumer` แต่ละตัวพิมพ์ `batch_id` เมื่อสำเร็จ สำหรับการจำลองทีมอื่นให้ clone [repository บน GitHub](https://github.com/MathPro29/Omni-Libs) ลงคนละโฟลเดอร์ แล้วติดตั้งตามคู่มือของแต่ละภาษา
