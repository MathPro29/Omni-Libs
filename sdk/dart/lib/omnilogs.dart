import 'dart:async';
import 'dart:convert';
import 'dart:io';

class OmniLogsException implements Exception {
  final int status;
  final String code;
  final String message;

  OmniLogsException(this.status, this.code, this.message);

  @override
  String toString() => 'OmniLogs HTTP $status: $code: $message';
}

class OmniLogsClient {
  final Uri baseUrl;
  final String apiKey;
  final String environmentCode;
  final Duration timeout;
  final HttpClient _httpClient;

  OmniLogsClient({
    required String baseUrl,
    required this.apiKey,
    required this.environmentCode,
    this.timeout = const Duration(seconds: 5),
    HttpClient? httpClient,
  })  : baseUrl = Uri.parse(baseUrl),
        _httpClient = httpClient ?? HttpClient() {
    if (!['http', 'https'].contains(this.baseUrl.scheme) ||
        this.baseUrl.host.isEmpty ||
        this.baseUrl.userInfo.isNotEmpty ||
        this.baseUrl.hasQuery ||
        this.baseUrl.hasFragment ||
        (this.baseUrl.path.isNotEmpty && this.baseUrl.path != '/') ||
        apiKey.trim().isEmpty ||
        environmentCode.trim().isEmpty ||
        timeout <= Duration.zero) {
      throw ArgumentError('Use an HTTP(S) origin, API key, environment code and positive timeout');
    }
  }

  /// Returns when the batch is queued; indexing happens later.
  Future<Map<String, dynamic>> send(Map<String, dynamic> batch) async {
    final logs = batch['logs'];
    if (batch['queue_key'] is! String || (batch['queue_key'] as String).isEmpty ||
        batch['source_type'] is! String || (batch['source_type'] as String).isEmpty ||
        batch['source_platform'] is! String || (batch['source_platform'] as String).isEmpty ||
        batch['idempotency_key'] is! String || (batch['idempotency_key'] as String).isEmpty ||
        logs is! List || logs.isEmpty) {
      throw ArgumentError('queue_key, source_type, source_platform, idempotency_key and logs are required');
    }
    for (var i = 0; i < logs.length; i++) {
      final log = logs[i];
      if (log is! Map || log['sequence_no'] != i + 1 ||
          log['source_type'] is! String || (log['source_type'] as String).isEmpty ||
          log['source_platform'] is! String || (log['source_platform'] as String).isEmpty ||
          log['data'] is! Map) {
        throw ArgumentError('Invalid log at index $i');
      }
    }
    if (batch['environment_code'] != null &&
        batch['environment_code'] != environmentCode) {
      throw ArgumentError('batch environment differs from client environment');
    }
    return _request('POST', '/api/v1/ingest/logs', {
      ...batch,
      'environment_code': environmentCode,
    });
  }

  /// Intended for diagnostics, not polling after every send.
  Future<Map<String, dynamic>> status(String batchId) {
    if (batchId.trim().isEmpty) throw ArgumentError('batchId is required');
    return _request('GET', '/api/v1/ingest/batches/${Uri.encodeComponent(batchId)}');
  }

  Future<Map<String, dynamic>> _request(String method, String path,
      [Map<String, dynamic>? body]) async {
    final uri = baseUrl.replace(path: path);
    final request = await _httpClient.openUrl(method, uri).timeout(timeout);
    request.followRedirects = false;
    request.headers.set('X-API-Key', apiKey);
    request.headers.set('X-Environment-Code', environmentCode);
    request.headers.set('Accept', 'application/json');
    if (body != null) {
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(body));
    }
    final response = await request.close().timeout(timeout);
    final text = await utf8.decoder.bind(response).join().timeout(timeout);
    final envelope = jsonDecode(text) as Map<String, dynamic>;
    if (response.statusCode < 200 || response.statusCode >= 300) {
      final error = envelope['error'] as Map<String, dynamic>?;
      throw OmniLogsException(response.statusCode,
          error?['code']?.toString() ?? 'UNKNOWN',
          error?['message']?.toString() ?? 'request failed');
    }
    final data = envelope['data'];
    if (data is! Map<String, dynamic> || data['batch_id'] is! String) {
      throw const FormatException('OmniLogs returned an invalid batch response');
    }
    return data;
  }

  void close() => _httpClient.close();
}
