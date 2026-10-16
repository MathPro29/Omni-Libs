import 'dart:io';
import 'dart:math';
import 'package:omnilogs/omnilogs.dart' as omni;

Future<void> main() async {
  final client = omni.OmniLogsClient(
    baseUrl: Platform.environment['OMNILOGS_BASE_URL']!,
    apiKey: Platform.environment['OMNILOGS_API_KEY']!,
    environmentCode: Platform.environment['OMNILOGS_ENVIRONMENT_CODE']!,
  );
  final random = Random.secure();
  final id = List.generate(16, (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0')).join();
  try {
    final result = await client.send({
      'queue_key': Platform.environment['OMNILOGS_QUEUE_KEY'],
      'source_type': 'SERVICE',
      'source_platform': 'HTTP',
      'idempotency_key': 'orders-service:dart:$id',
      'logs': [{
        'sequence_no': 1,
        'source_type': 'SERVICE',
        'source_platform': 'HTTP',
        'data': {
          'service': Platform.environment['OMNILOGS_SERVICE'],
          'level': 'INFO',
          'message': 'order created',
          'request_id': id,
        },
      }],
    });
    print('Log queued in OmniLogs: batch_id=${result['batch_id']}');
  } finally {
    client.close();
  }
}
