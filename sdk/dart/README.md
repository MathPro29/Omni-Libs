# Client สำหรับ Dart ฝั่ง server

เพิ่ม dependency ใน `pubspec.yaml` ของ server project แล้ว pin ref ให้ชัดเจน:

```yaml
dependencies:
  omnilogs:
    git:
      url: https://github.com/MathPro29/Omni-Libs.git
      ref: YOUR_COMMIT_OR_TAG
      path: sdk/dart
```

ตัวอย่างเรียกใช้:

```dart
import 'dart:io';
import 'package:omnilogs/omnilogs.dart' as omni;

final client = omni.OmniLogsClient(
  baseUrl: Platform.environment['OMNILOGS_BASE_URL']!,
  apiKey: Platform.environment['OMNILOGS_API_KEY']!,
  environmentCode: Platform.environment['OMNILOGS_ENVIRONMENT_CODE']!,
);
try {
  final result = await client.send({
    'queue_key': 'orders-api', 'source_type': 'SERVICE',
    'source_platform': 'HTTP', 'idempotency_key': 'orders-api:req-123',
    'logs': [{'sequence_no': 1, 'source_type': 'SERVICE',
      'source_platform': 'HTTP', 'data': {'message': 'order created'}}],
  });
  print(result['batch_id']);
} finally {
  client.close();
}
```

นำเข้า `dart:io` เพื่อใช้ `Platform` package นี้เหมาะกับ Dart process ฝั่ง server ไม่ใช่ Flutter app ที่แจกให้ผู้ใช้

ดูวิธีทดลองใช้งานผ่าน consumer ใน [คู่มือ SDK](../README.md#ลองใช้งานเหมือนเป็นทีมผู้พัฒนา-service)
