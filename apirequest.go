package aikit

import (
	"net/http"
)

type APIRequest interface {
	Name() string
	Transport() GatewayTransport
	InitSession(state *StreamState)
	PrepareForUpdates()
	ParseHttpError(code int, body []byte) *AIError
	Update(block *ThreadBlock)
	MakeRequest(state *StreamState) *http.Request
	OnChunk(data []byte, state *StreamState) ChunkResult
}
