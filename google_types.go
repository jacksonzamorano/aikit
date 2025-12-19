package aikit

import "encoding/json"

type VertexRequest struct {
	SystemInstruction *VertexContent  `json:"system_instruction,omitempty"`
	Contents          []VertexContent `json:"contents"`
	Tools             VertexTools     `json:"tools"`
}
type VertexTools struct {
	FunctionDeclarations []map[string]any `json:"functionDeclarations,omitempty"`
}
type VertexGenerateContentResponse struct {
	Candidates []VertexCandidate    `json:"candidates"`
	Usage      VertexUsageMetadata  `json:"usageMetadata"`
}
type VertexErrorResponse struct {
	Error VertexErrorResponseError `json:"error,omitempty"`
}
type VertexErrorResponseError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
type VertexUsageMetadata struct {
	InputTokens    int64 `json:"promptTokenCount"`
	OutputTokens   int64 `json:"candidatesTokenCount"`
	CachedTokens   int64 `json:"cachedContentTokenCount"`
	ThinkingTokens int64 `json:"thinkingTokenCount"`
}
type VertexCandidate struct {
	Content      VertexContent `json:"content"`
	FinishReason *string       `json:"finishReason,omitempty"`
}
type VertexContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []VertexPart `json:"parts,omitempty"`
}
type VertexPart struct {
	Thought          bool                  `json:"thought,omitempty"`
	ThoughtSignature string                `json:"thoughtSignature,omitempty"`
	Text             string                `json:"text,omitempty"`
	FunctionCall     *VertexFunctionCall   `json:"functionCall,omitempty"`
	FunctionResult   *VertexFunctionResult `json:"functionResponse,omitempty"`
}
type VertexFunctionCall struct {
	Name string          `json:"name,omitempty"`
	Args json.RawMessage `json:"args,omitempty"`
}
type VertexFunctionResult struct {
	Id       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}
