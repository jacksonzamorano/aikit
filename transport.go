package aikit

import "context"

// Transport is a pure I/O interface for sending requests and reading
// streamed response chunks. Implementations handle protocol details
// (SSE, WebSocket, etc.) while remaining agnostic to API formats.
//
// All transports are long-lived: created once per session via
// APIRequest.MakeTransport() and reused across multiple request cycles
// (e.g. tool call loops).
type Transport interface {
	// Configure sets the request payload for the next Start call.
	// The payload is pre-encoded bytes (e.g. JSON request body).
	Configure(data []byte) error

	// Start sends the configured payload and prepares to read response chunks.
	// The context controls cancellation of the underlying request.
	Start(ctx context.Context) error

	// ReadChunk returns the next chunk of response data.
	// Returns io.EOF when the response is complete.
	// Protocol-level details (empty events, sentinels) are pre-filtered.
	ReadChunk() ([]byte, error)

	// Close cleans up transport resources.
	Close() error
}
