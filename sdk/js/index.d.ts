export interface LogItem {
  sequence_no: number;
  source_type: string;
  source_platform: string;
  data: Record<string, unknown>;
  custom_fields?: Record<string, unknown>;
}

export interface Batch {
  queue_key: string;
  source_type: string;
  source_platform: string;
  idempotency_key: string;
  environment_code?: string;
  logs: LogItem[];
}

export interface BatchResult {
  batch_id: string;
  status: string;
  [key: string]: unknown;
}

export class OmniLogsError extends Error {
  readonly status: number;
  readonly code: string;
}

export class OmniLogsClient {
  constructor(config: { baseUrl: string; apiKey: string; environmentCode: string; timeoutMs?: number });
  send(batch: Batch): Promise<BatchResult>;
  status(batchId: string): Promise<BatchResult>;
}
