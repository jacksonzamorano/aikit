package aikit

import "encoding/json"

type AIStudioRequest struct {
	SystemInstruction *AIStudioContent  `json:"system_instruction,omitempty"`
	Contents          []AIStudioContent `json:"contents"`
	Tools             AIStudioTools     `json:"tools"`
}
type AIStudioTools struct {
	FunctionDeclarations []map[string]any `json:"functionDeclarations,omitempty"`
}
type AIStudioGenerateContentResponse struct {
	Candidates []AIStudioCandidate    `json:"candidates"`
	Usage      AIStudioUsageMetadata  `json:"usageMetadata"`
}
type AIStudioErrorResponse struct {
	Error AIStudioErrorResponseError `json:"error,omitempty"`
}
type AIStudioErrorResponseError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
type AIStudioUsageMetadata struct {
	InputTokens    int64 `json:"promptTokenCount"`
	OutputTokens   int64 `json:"candidatesTokenCount"`
	CachedTokens   int64 `json:"cachedContentTokenCount"`
	ThinkingTokens int64 `json:"thinkingTokenCount"`
}
type AIStudioCandidate struct {
	Content      AIStudioContent `json:"content"`
	FinishReason *string       `json:"finishReason,omitempty"`
}
type AIStudioContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []AIStudioPart `json:"parts,omitempty"`
}
type AIStudioPart struct {
	Thought          bool                  `json:"thought,omitempty"`
	ThoughtSignature string                `json:"thoughtSignature,omitempty"`
	Text             string                `json:"text,omitempty"`
	FunctionCall     *AIStudioFunctionCall   `json:"functionCall,omitempty"`
	FunctionResult   *AIStudioFunctionResult `json:"functionResponse,omitempty"`
}
type AIStudioFunctionCall struct {
	Name string          `json:"name,omitempty"`
	Args json.RawMessage `json:"args,omitempty"`
}
type AIStudioFunctionResult struct {
	Id       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}
