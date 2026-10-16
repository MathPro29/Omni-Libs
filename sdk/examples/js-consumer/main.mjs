import { randomUUID } from 'node:crypto';
import * as omni from '@nextate/omnilogs';

const client = new omni.OmniLogsClient({
  baseUrl: process.env.OMNILOGS_BASE_URL,
  apiKey: process.env.OMNILOGS_API_KEY,
  environmentCode: process.env.OMNILOGS_ENVIRONMENT_CODE,
});
const id = randomUUID();
const result = await client.send({
  queue_key: process.env.OMNILOGS_QUEUE_KEY,
  source_type: 'SERVICE',
  source_platform: 'HTTP',
  idempotency_key: `orders-service:js:${id}`,
  logs: [{
    sequence_no: 1,
    source_type: 'SERVICE',
    source_platform: 'HTTP',
    data: {
      service: process.env.OMNILOGS_SERVICE,
      level: 'INFO',
      message: 'order created',
      request_id: id,
    },
  }],
});
console.log(`Log queued in OmniLogs: batch_id=${result.batch_id}`);
