// Package omni sends log batches to an OmniLogs server.
//
// The client uses the public HTTP ingestion API and has no dependency on the
// OmniLogs database, queue, or worker implementation. A successful [Client.Send]
// means the server queued the batch; indexing happens asynchronously.
//
// Create a client with [New], then send a batch with [Client.Send]. Keep the API
// key in a server-side secret store and pass a request-scoped context so callers
// can cancel or time out network requests.
package omni
