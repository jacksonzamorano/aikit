package aikit

import "encoding/json"

type CompletionsRequest struct {
	Model    string               `json:"model"`
	Messages []CompletionsMessage `json:"messages"`
	Tools    []map[string]any     `json:"tools,omitempty"`
}
type CompletionsMessage struct {
	Id         string                `json:"id,omitempty"`
	Role       string                `json:"role,omitempty"`
	Content    []CompletionTextBlock `json:"content,omitempty"`
	ToolCalls  []CompletionsToolCall `json:"tool_calls,omitempty"`
	ToolCallId string                `json:"tool_call_id,omitempty"`
}
type CompletionTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type CompletionsUsage struct {
	PromptTokens       int64                         `json:"prompt_tokens"`
	CompletionTokens   int64                         `json:"completion_tokens"`
	PromptTokenDetails CompletionsPromptTokensDetail `json:"prompt_tokens_details"`
}
type CompletionsPromptTokensDetail struct {
	CachedTokens int64 `json:"cached_tokens"`
}

type CompletionsResponse struct {
	Choices []CompletionsChoice `json:"choices"`
	Usage   CompletionsUsage    `json:"usage"`
}
type CompletionsChoice struct {
	Message CompletionsMessage `json:"message"`
}
type CompletionsToolCall struct {
	Id       string                       `json:"id"`
	Type     string                       `json:"type"`
	Function *CompletionsToolCallFunction `json:"function,omitempty"`
}
type CompletionsToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}
type CompletionsStreamChunk struct {
	Choices []CompletionsStreamChoice `json:"choices"`
	Usage   *CompletionsUsage         `json:"usage,omitempty"`
}

type CompletionsStreamChoice struct {
	Delta        CompletionsStreamDelta `json:"delta"`
	FinishReason *string                `json:"finish_reason,omitempty"`
}

type CompletionsStreamDelta struct {
	Role             string                     `json:"role,omitempty"`
	Content          string                     `json:"content,omitempty"`
	ReasoningContent string                     `json:"reasoning_content,omitempty"`
	ToolCalls        []CompletionsToolCallDelta `json:"tool_calls,omitempty"`
}

type CompletionsToolCallDelta struct {
	Index    int                               `json:"index,omitempty"`
	Id       string                            `json:"id,omitempty"`
	Type     string                            `json:"type,omitempty"`
	Function *CompletionsToolCallFunctionDelta `json:"function,omitempty"`
}

type CompletionsToolCallFunctionDelta struct {
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}
