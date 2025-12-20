package aikit

import (
	"net/http"
)

type APIRequest interface {
	Name() string
	Transport() GatewayTransport
	InitSession(state *Thread)
	PrepareForUpdates()
	ParseHttpError(code int, body []byte) *AIError
	Update(block *ThreadBlock)
	MakeRequest(state *Thread) *http.Request
	OnChunk(data []byte, state *Thread) ChunkResult
}
