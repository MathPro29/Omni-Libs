# Orders Product: ทดลองใช้ OmniLogs จาก GitHub

โฟลเดอร์นี้จำลอง service ของอีกทีมหนึ่ง โดยดึง `Omni-Libs` จาก GitHub ลงใน `product/vendor` แล้วติดตั้ง JavaScript SDK จากสำเนานั้น ไม่อ้าง `../sdk/js` ใน workspace นี้

ต้องมี Git, Node.js 20.6 ขึ้นไป และ npm เปิด PowerShell ในโฟลเดอร์ `product` แล้วรัน:

```powershell
.\setup.ps1
Copy-Item .env.example .env.local
```

แก้ `.env.local` ให้เป็น API origin, Product API key และ Environment สำหรับทดสอบจริง รวมทั้ง queue และชื่อ service แล้วรัน:

```powershell
npm start
```

คำสั่งนี้จำลองเหตุการณ์ `order.created` หนึ่งรายการและแสดง `batch_id` เมื่อ OmniLogs รับเข้าคิว แต่ละครั้งสร้าง `order_id` และ `idempotency_key` ใหม่ อย่า commit `.env.local`; ไฟล์นี้และ `vendor` ถูก Git ignore แล้ว

หากต้องการลอง commit ใหม่จาก GitHub ให้ใช้ clone ใหม่ใน `vendor/Omni-Libs` ก่อนรัน `setup.ps1` อีกครั้ง
