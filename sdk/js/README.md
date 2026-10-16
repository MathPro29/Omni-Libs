# Client สำหรับ Node.js และ TypeScript

package ใช้ `fetch` ที่มีมาให้ใน Node.js 18 ขึ้นไป สำหรับการทดลองจาก GitHub ให้ clone repository และสร้าง package archive จาก `sdk/js`:

```powershell
git clone https://github.com/MathPro29/Omni-Libs.git
cd Omni-Libs\sdk\js
npm pack
```

ในโปรเจกต์ผู้ใช้แยก ติดตั้งไฟล์ `nextate-omnilogs-0.1.0.tgz` ที่สร้างขึ้น (ระบุ path ไปยังไฟล์จริง):

```powershell
npm install C:\path\to\nextate-omnilogs-0.1.0.tgz
```

ตัวอย่างเรียกใช้:

```js
import * as omni from '@nextate/omnilogs';

const client = new omni.OmniLogsClient({
  baseUrl: process.env.OMNILOGS_BASE_URL,
  apiKey: process.env.OMNILOGS_API_KEY,
  environmentCode: process.env.OMNILOGS_ENVIRONMENT_CODE,
});

const result = await client.send({
  queue_key: 'orders-api',
  source_type: 'SERVICE',
  source_platform: 'HTTP',
  idempotency_key: 'orders-api:req-123',
  logs: [{ sequence_no: 1, source_type: 'SERVICE', source_platform: 'HTTP',
    data: { message: 'order created', level: 'INFO' } }],
});
console.log(result.batch_id);
```

เรียกใช้ใน process ฝั่ง server ที่เชื่อถือได้เท่านั้น ห้าม bundle API key ไว้ใน browser

ดูวิธีทดลองใช้งานผ่าน consumer ใน [คู่มือ SDK](../README.md#ลองใช้งานเหมือนเป็นทีมผู้พัฒนา-service)
