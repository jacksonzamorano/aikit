package aikit

type CompletionsErrorResponse struct {
	Error CompletionsErrorDetail `json:"error"`
}
type CompletionsErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
}
type CompletionsRequest struct {
	Model           string               `json:"model"`
	Messages        []CompletionsMessage `json:"messages"`
	Tools           []map[string]any     `json:"tools,omitempty"`
	Stream          bool                 `json:"stream,omitempty"`
	StreamOptions   map[string]any       `json:"stream_options,omitempty"`
	ReasoningEffort string               `json:"reasoning_effort,omitempty"`
}
type CompletionsMessage struct {
	Id               string                `json:"id,omitempty"`
	Role             string                `json:"role,omitempty"`
	Content          any                   `json:"content,omitempty"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	ToolCalls        []CompletionsToolCall `json:"tool_calls,omitempty"`
	ToolCallId       string                `json:"tool_call_id,omitempty"`
	Name             string                `json:"name,omitempty"`
}
type CompletionTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CompletionImageUrlBlock represents an image content block for Chat Completions API.
type CompletionImageUrlBlock struct {
	Type     string                   `json:"type"` // "image_url"
	ImageUrl CompletionImageUrlDetail `json:"image_url"`
}

// CompletionImageUrlDetail contains the image URL details.
type CompletionImageUrlDetail struct {
	Url string `json:"url"` // data URL or https URL
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
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type CompletionsStreamChunk struct {
	Id      string                    `json:"id"`
	Choices []CompletionsStreamChoice `json:"choices"`
	Usage   *CompletionsUsage         `json:"usage,omitempty"`
}

type CompletionsStreamChoice struct {
	Index        int                    `json:"index"`
	Delta        CompletionsStreamDelta `json:"delta"`
	FinishReason *string                `json:"finish_reason,omitempty"`
}

type CompletionsStreamDelta struct {
	Role             string                     `json:"role,omitempty"`
	Content          string                     `json:"content,omitempty"`
	ReasoningContent string                     `json:"reasoning,omitempty"`
	ToolCalls        []CompletionsToolCallDelta `json:"tool_calls,omitempty"`
}

type CompletionsToolCallDelta struct {
	Index    int                               `json:"index,omitempty"`
	Id       string                            `json:"id,omitempty"`
	Type     string                            `json:"type,omitempty"`
	Function *CompletionsToolCallFunctionDelta `json:"function,omitempty"`
}

type CompletionsToolCallFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
