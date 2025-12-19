package aikit

import (
	"encoding/json"
	"net/http"
)

type InferenceBlockType string

const (
	InferenceBlockSystem            InferenceBlockType = "system"
	InferenceBlockInput             InferenceBlockType = "input"
	InferenceBlockThinking          InferenceBlockType = "thinking"
	InferenceBlockEncryptedThinking InferenceBlockType = "encrypted_thinking"
	InferenceBlockText              InferenceBlockType = "text"
	InferenceBlockToolCall          InferenceBlockType = "tool_call"
	InferenceBlockToolResult        InferenceBlockType = "tool_result"
)

type InferenceToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"-"`
}

type InferenceToolResult struct {
	ToolCallID string          `json:"tool_call_id"`
	Output     json.RawMessage `json:"-"`
}

type InferenceBlock struct {
	ID         string               `json:"id"`
	Type       InferenceBlockType   `json:"type"`
	Text       string               `json:"text,omitempty"`
	Signature  string               `json:"signature,omitempty"`
	ToolCall   *InferenceToolCall   `json:"tool_call,omitempty"`
	ToolResult *InferenceToolResult `json:"tool_result,omitempty"`
	Complete   bool                 `json:"complete"`
	Citations  []string             `json:"citations,omitempty"`
}

type InferenceProvider interface {
	Name() string
	Transport() InferenceTransport
	InitSession(state *ProviderState)
	PrepareForUpdates()
	ParseHttpError(code int, body []byte) *AIError
	Update(block *InferenceBlock)
	MakeRequest(state *ProviderState) *http.Request
	OnChunk(data []byte, state *ProviderState) ChunkResult
}
