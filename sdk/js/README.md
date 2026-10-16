# Client สำหรับ Node.js และ TypeScript

เผยแพร่โฟลเดอร์นี้ไปยัง private npm registry ก่อน แล้วติดตั้ง `@nextate/omnilogs` package ใช้ `fetch` ที่มีมาให้ใน Node.js 18 ขึ้นไป

ผู้ดูแลที่มีสิทธิ์ publish สามารถเผยแพร่จากโฟลเดอร์ `sdk/js` ไปยัง GitLab package registry ได้ (แทน `PROJECT_ID` ด้วย project ID และตั้ง npm token ใน npm config หรือ CI):

```sh
npm publish --registry=https://gitlab.dev.nextate.tech/api/v4/projects/PROJECT_ID/packages/npm/
```

ทีมที่ติดตั้งให้ตั้งค่า npm scope ให้ชี้ registry เดียวกันใน `.npmrc` และอ่าน token จาก secret store จากนั้นติดตั้ง:

```sh
npm install @nextate/omnilogs
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
