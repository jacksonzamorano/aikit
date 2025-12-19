package aikit

import "encoding/json"

type MessagesErrorResponse struct {
	Error MessagesError `json:"error"`
}
type MessagesError struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
}

type MessagesRequest struct {
	Model     string            `json:"model"`
	Messages  []MessagesMessage `json:"messages"`
	Tools     []map[string]any  `json:"tools"`
	System    string            `json:"system"`
	MaxTokens int64             `json:"max_tokens"`
	Thinking  *MessagesThinking `json:"thinking"`
	Stream    bool              `json:"stream"`
}
type MessagesMessage struct {
	Role    string            `json:"role"`
	Content []MessagesContent `json:"content"`
}
type MessagesCacheControl struct {
	Type string `json:"type"`
}
type MessagesThinking struct {
	BudgetTokens int64  `json:"budget_tokens"`
	Type         string `json:"type"`
}

type MessagesResult struct {
	Type       string            `json:"type"`
	Content    []MessagesContent `json:"content"`
	Usage      MessagesUsage     `json:"usage"`
	StopReason string            `json:"stop_reason"`
}
type MessagesContent struct {
	Type         string                    `json:"type"`
	Thinking     string                    `json:"thinking,omitempty"`
	Signature    string                    `json:"signature,omitempty"`
	Data         string                    `json:"data,omitempty"`
	Text         string                    `json:"text,omitempty"`
	Name         string                    `json:"name,omitempty"`
	Id           string                    `json:"id,omitempty"`
	Input        any                       `json:"input,omitempty"`
	ToolUseId    string                    `json:"tool_use_id,omitempty"`
	Content      any                       `json:"content,omitempty"`
	CacheControl *MessagesCacheControl     `json:"cache_control,omitempty"`
	Citations    []MessagesContentCitation `json:"citations,omitempty"`
}
type MessagesContentCitation struct {
	Url            string `json:"url"`
	EncryptedIndex string `json:"encrypted_index"`
}
type MessagesUsage struct {
	InputTokens              int64                    `json:"input_tokens"`
	CacheReadInputTokens     int64                    `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64                    `json:"cache_creation_input_tokens"`
	OutputTokens             int64                    `json:"output_tokens"`
	ServerToolUse            *MessagesServerToolUsage `json:"server_tool_use,omitempty"`
}
type MessagesServerToolUsage struct {
	WebSearchRequests int64 `json:"web_search_requests,omitempty"`
}

type MessagesStreamEnvelope struct {
	Type string `json:"type"`
}

type MessagesStreamErrorEvent struct {
	Type  string         `json:"type"`
	Error *MessagesError `json:"error,omitempty"`
}

type MessagesStreamMessage struct {
	ID    string        `json:"id,omitempty"`
	Usage MessagesUsage `json:"usage"`
}

type MessagesStreamMessageStart struct {
	Type    string                `json:"type"`
	Message MessagesStreamMessage `json:"message"`
}

type MessagesStreamUsageDelta struct {
	OutputTokens int64 `json:"output_tokens,omitempty"`
}

type MessagesStreamMessageDeltaData struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type MessagesStreamMessageDelta struct {
	Type  string                         `json:"type"`
	Delta MessagesStreamMessageDeltaData `json:"delta"`
	Usage MessagesUsage                  `json:"usage"`
}

type MessagesStreamContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Data      string          `json:"data,omitempty"`
	Name      string          `json:"name,omitempty"`
	ID        string          `json:"id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

type MessagesStreamContentBlockStart struct {
	Type         string                     `json:"type"`
	Index        int                        `json:"index"`
	ContentBlock MessagesStreamContentBlock `json:"content_block"`
}

type MessagesStreamContentDelta struct {
	Type  string                         `json:"type"`
	Delta MessagesStreamContentDeltaData `json:"delta"`
}
type MessagesStreamContentDeltaData struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Data        string `json:"data,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type MessagesStreamContentBlockDelta struct {
	Type  string                     `json:"type"`
	Index int                        `json:"index"`
	Delta MessagesStreamContentDelta `json:"delta"`
}

type MessagesStreamContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type MessagesBuilderBlock struct {
	Index     int    `json:"index"`
	Type      string `json:"type"`
	Thinking  string `json:"thinking,omitempty"`
	Data      string `json:"data,omitempty"`
	Signature string `json:"signature,omitempty"`
	Text      string `json:"text,omitempty"`
	ToolId    string `json:"id,omitempty"`
	ToolName  string `json:"name,omitempty"`
	ToolInput string `json:"input,omitempty"`
}
