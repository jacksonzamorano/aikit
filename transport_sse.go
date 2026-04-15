package aikit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// SSETransport implements Transport for Server-Sent Events over HTTP.
// Each Start() call makes a new HTTP POST request; the connection is
// not reused between cycles.
type SSETransport struct {
	providerName string
	url          string
	headers      http.Header
	parseError   func(int, []byte) *AIError

	payload []byte
	body    io.ReadCloser

	// Channel bridge from readSSE goroutine to pull-based ReadChunk.
	chunks <-chan sseChunkOrErr
}

// NewSSETransport creates a Transport that sends HTTP POST requests and
// reads SSE streams. The parseError function handles provider-specific
// HTTP error responses.
func NewSSETransport(
	providerName string,
	url string,
	headers http.Header,
	parseError func(int, []byte) *AIError,
) *SSETransport {
	return &SSETransport{
		providerName: providerName,
		url:          url,
		headers:      headers,
		parseError:   parseError,
	}
}

func (t *SSETransport) Configure(data []byte) error {
	t.payload = data
	return nil
}

func (t *SSETransport) Start(ctx context.Context) error {
	// Close any previous response body.
	if t.body != nil {
		t.body.Close()
		t.body = nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(t.payload))
	if err != nil {
		return &AIError{
			Category: AIErrorCategoryStreamingError,
			Provider: t.providerName,
			Message:  fmt.Sprintf("failed to create request: %s", err.Error()),
		}
	}
	for k, vals := range t.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &AIError{
			Category: AIErrorCategoryStreamingError,
			Provider: t.providerName,
			Message:  err.Error(),
		}
	}

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if t.parseError != nil {
			if parsed := t.parseError(resp.StatusCode, body); parsed != nil {
				return parsed
			}
		}
		return &AIError{
			Category: AIErrorCategoryHTTPStatus,
			Provider: t.providerName,
			Message:  fmt.Sprintf("Unhandled error. Received status code %d with body %s", resp.StatusCode, string(body)),
		}
	}

	t.body = resp.Body
	t.chunks = readSSE(t.providerName, resp.Body)
	return nil
}

func (t *SSETransport) ReadChunk() ([]byte, error) {
	chunk, ok := <-t.chunks
	if !ok {
		return nil, io.EOF
	}
	if chunk.err != nil {
		return nil, chunk.err
	}
	return chunk.data, nil
}

func (t *SSETransport) Close() error {
	if t.body != nil {
		t.body.Close()
		t.body = nil
	}
	return nil
}
