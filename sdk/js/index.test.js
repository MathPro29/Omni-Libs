import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { after, before, test } from 'node:test';
import { OmniLogsClient, OmniLogsError } from './index.js';

let server;
let baseUrl;
let reply = { status: 201, body: { data: { batch_id: 'batch-1', status: 'QUEUED' } } };
let received;

before(async () => {
  server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    received = { method: request.method, path: request.url, headers: request.headers, body: Buffer.concat(chunks).toString() };
    if (reply.delayMs) await new Promise((resolve) => setTimeout(resolve, reply.delayMs));
    response.writeHead(reply.status, { 'Content-Type': 'application/json', ...(reply.status === 302 ? { Location: '/redirected' } : {}) });
    response.end(typeof reply.body === 'string' ? reply.body : JSON.stringify(reply.body));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  baseUrl = `http://127.0.0.1:${server.address().port}`;
});

after(() => server?.close());

test('send follows the OmniLogs contract', async () => {
  const client = new OmniLogsClient({ baseUrl, apiKey: 'test-key', environmentCode: 'UAT' });
  const result = await client.send({
    queue_key: 'orders', source_type: 'SERVICE', source_platform: 'HTTP', idempotency_key: 'orders:1',
    logs: [{ sequence_no: 1, source_type: 'SERVICE', source_platform: 'HTTP', data: { message: 'created' } }],
  });
  assert.equal(result.batch_id, 'batch-1');
  assert.equal(received.method, 'POST');
  assert.equal(received.path, '/api/v1/ingest/logs');
  assert.equal(received.headers['x-api-key'], 'test-key');
  assert.equal(received.headers['x-environment-code'], 'UAT');
  assert.equal(JSON.parse(received.body).environment_code, 'UAT');
  assert.equal(JSON.parse(received.body).idempotency_key, 'orders:1');
});

test('non-JSON HTTP errors retain their status', async () => {
  reply = { status: 502, body: '<html>bad gateway</html>' };
  const client = new OmniLogsClient({ baseUrl, apiKey: 'test-key', environmentCode: 'UAT' });
  await assert.rejects(client.status('batch-1'), (error) => error instanceof OmniLogsError && error.status === 502 && error.code === 'INVALID_RESPONSE');
});

test('invalid acknowledgements are rejected', async () => {
  reply = { status: 201, body: { data: { batch_id: '' } } };
  const client = new OmniLogsClient({ baseUrl, apiKey: 'test-key', environmentCode: 'UAT' });
  await assert.rejects(client.status('batch-1'), /invalid batch response/);
});

test('redirects are not followed', async () => {
  reply = { status: 302, body: '' };
  const client = new OmniLogsClient({ baseUrl, apiKey: 'test-key', environmentCode: 'UAT' });
  await assert.rejects(client.status('batch-1'));
  assert.equal(received.path, '/api/v1/ingest/batches/batch-1');
});

test('request timeout aborts the call', async () => {
  reply = { status: 201, body: { data: { batch_id: 'late' } }, delayMs: 100 };
  const client = new OmniLogsClient({ baseUrl, apiKey: 'test-key', environmentCode: 'UAT', timeoutMs: 10 });
  await assert.rejects(client.status('batch-1'), (error) => error.name === 'TimeoutError');
});
