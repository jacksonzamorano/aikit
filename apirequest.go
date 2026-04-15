package aikit

type APIRequest interface {
	Name() string
	InitSession(state *StreamState)
	PrepareForUpdates()
	Update(block *ThreadBlock)
	OnChunk(data []byte, state *StreamState) ChunkResult
	EncodeRequest(state *StreamState) []byte
	MakeTransport() Transport
}
