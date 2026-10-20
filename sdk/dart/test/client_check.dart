import 'dart:convert';
import 'dart:io';

import 'package:omnilogs/omnilogs.dart';

Future<void> main() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final client = OmniLogsClient(
    baseUrl: 'http://127.0.0.1:${server.port}',
    apiKey: 'test-key',
    environmentCode: 'UAT',
  );
  var responseStatus = HttpStatus.created;
  var responseBody = '{"data":{"batch_id":"batch-1","status":"QUEUED"}}';
  final requests = <Future<void>>[];
  server.listen((request) {
    requests.add(() async {
      final body = await utf8.decoder.bind(request).join();
      if (request.method != 'POST' ||
          request.uri.path != '/api/v1/ingest/logs' ||
          request.headers.value('X-API-Key') != 'test-key' ||
          request.headers.value('X-Environment-Code') != 'UAT' ||
          (jsonDecode(body) as Map<String, dynamic>)['idempotency_key'] !=
              'orders:1') {
        throw StateError('Client request does not match the protocol');
      }
      request.response.statusCode = responseStatus;
      request.response.write(responseBody);
      await request.response.close();
    }());
  });
  try {
    final result = await client.send({
      'queue_key': 'orders',
      'source_type': 'SERVICE',
      'source_platform': 'HTTP',
      'idempotency_key': 'orders:1',
      'logs': [
        {
          'sequence_no': 1,
          'source_type': 'SERVICE',
          'source_platform': 'HTTP',
          'data': {'message': 'created'},
        }
      ],
    });
    if (result['batch_id'] != 'batch-1') {
      throw StateError('Missing batch acknowledgement');
    }
    responseStatus = HttpStatus.badGateway;
    responseBody = '<html>bad gateway</html>';
    try {
      await client.send({
        'queue_key': 'orders',
        'source_type': 'SERVICE',
        'source_platform': 'HTTP',
        'idempotency_key': 'orders:1',
        'logs': [
          {
            'sequence_no': 1,
            'source_type': 'SERVICE',
            'source_platform': 'HTTP',
            'data': {'message': 'created'},
          }
        ],
      });
      throw StateError('Expected an OmniLogsException');
    } on OmniLogsException catch (error) {
      if (error.status != HttpStatus.badGateway ||
          error.code != 'INVALID_RESPONSE') {
        rethrow;
      }
    }
    await Future.wait(requests);
  } finally {
    client.close();
    await server.close(force: true);
  }
}
