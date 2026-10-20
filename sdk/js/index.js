export class OmniLogsError extends Error {
  constructor(status, code, message) {
    super(`OmniLogs HTTP ${status}: ${code}: ${message}`);
    this.name = 'OmniLogsError';
    this.status = status;
    this.code = code;
  }
}

export class OmniLogsClient {
  constructor({ baseUrl, apiKey, environmentCode, timeoutMs = 5000 }) {
    const url = new URL(baseUrl);
    if (!['http:', 'https:'].includes(url.protocol) || !url.host || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '')) {
      throw new Error('baseUrl must be an HTTP(S) origin');
    }
    if (!apiKey || !environmentCode || !Number.isFinite(timeoutMs) || timeoutMs <= 0) {
      throw new Error('apiKey, environmentCode and a positive timeoutMs are required');
    }
    this.baseUrl = url.origin;
    this.apiKey = apiKey;
    this.environmentCode = environmentCode;
    this.timeoutMs = timeoutMs;
  }

  async send(batch) {
    if (!batch?.queue_key || !batch?.source_type || !batch?.source_platform || !batch?.idempotency_key || !Array.isArray(batch.logs) || batch.logs.length === 0) {
      throw new Error('queue_key, source_type, source_platform, idempotency_key and logs are required');
    }
    if (batch.logs.some((log, index) => log.sequence_no !== index + 1 || !log.source_type || !log.source_platform || !log.data || Array.isArray(log.data) || typeof log.data !== 'object')) {
      throw new Error('each log needs an ordered sequence_no, source_type, source_platform and object data');
    }
    if (batch.environment_code && batch.environment_code !== this.environmentCode) {
      throw new Error('batch environment differs from client environment');
    }
    return this.#request('POST', '/api/v1/ingest/logs', { ...batch, environment_code: this.environmentCode });
  }

  // Check status for diagnostics; avoid polling for every log.
  async status(batchId) {
    if (!batchId) throw new Error('batchId is required');
    return this.#request('GET', `/api/v1/ingest/batches/${encodeURIComponent(batchId)}`);
  }

  async #request(method, path, body) {
    const response = await fetch(this.baseUrl + path, {
      method,
      redirect: 'error',
      headers: {
        'X-API-Key': this.apiKey,
        'X-Environment-Code': this.environmentCode,
        Accept: 'application/json',
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(this.timeoutMs),
    });
    let payload;
    try {
      payload = await response.json();
    } catch {
      throw new OmniLogsError(response.status, 'INVALID_RESPONSE', 'response is not valid JSON');
    }
    if (!response.ok) throw new OmniLogsError(response.status, payload?.error?.code ?? 'UNKNOWN', payload?.error?.message ?? 'request failed');
    if (typeof payload?.data?.batch_id !== 'string' || !payload.data.batch_id) throw new Error('OmniLogs returned an invalid batch response');
    return payload.data;
  }
}
