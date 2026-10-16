import { randomUUID } from 'node:crypto';
import { OmniLogsClient } from '@nextate/omnilogs';

const required = ['OMNILOGS_BASE_URL', 'OMNILOGS_API_KEY', 'OMNILOGS_ENVIRONMENT_CODE', 'OMNILOGS_QUEUE_KEY', 'OMNILOGS_SERVICE'];
for (const name of required) {
  if (!process.env[name] || process.env[name] === 'replace-with-test-key') {
    throw new Error(`Set ${name} in .env.local before starting the product`);
  }
}

const client = new OmniLogsClient({
  baseUrl: process.env.OMNILOGS_BASE_URL,
  apiKey: process.env.OMNILOGS_API_KEY,
  environmentCode: process.env.OMNILOGS_ENVIRONMENT_CODE,
});

const orderId = randomUUID();
const result = await client.send({
  queue_key: process.env.OMNILOGS_QUEUE_KEY,
  source_type: 'SERVICE',
  source_platform: 'HTTP',
  idempotency_key: `orders-product:order-created:${orderId}`,
  logs: [{
    sequence_no: 1,
    source_type: 'SERVICE',
    source_platform: 'HTTP',
    data: {
      service: process.env.OMNILOGS_SERVICE,
      level: 'INFO',
      event: 'order.created',
      order_id: orderId,
      message: 'Sample order created',
    },
  }],
});

console.log(`Order ${orderId}: OmniLogs batch ${result.batch_id} (${result.status})`);
