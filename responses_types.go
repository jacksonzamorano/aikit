package aikit

import "encoding/json"

type ResponsesRequest struct {
	Model              string              `json:"model"`
	Inputs             []ResponsesInput    `json:"input"`
	Tools              []ResponsesTool     `json:"tools,omitempty"`
	Stream             bool                `json:"stream,omitempty"`
	Store              bool                `json:"store,omitempty"`
	Include            []string            `json:"include,omitempty"`
	Instructions       string              `json:"instructions,omitempty"`
	PreviousResponseID string              `json:"previous_response_id,omitempty"`
	Reasoning          *ResponsesReasoning `json:"reasoning,omitempty"`
	Text               *ResponsesText      `json:"text,omitempty"`
}

type ResponsesText struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

type ResponsesTextFormat struct {
	Type   string      `json:"type"`
	Name   string      `json:"name"`
	Schema *JsonSchema `json:"schema"`
	Strict bool        `json:"strict"`
}

type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}
type ResponsesTool struct {
	Type        string      `json:"type,omitempty"`
	Name        string      `json:"name,omitempty"`
	Parameters  *JsonSchema `json:"parameters,omitempty"`
	Description string      `json:"description,omitempty"`
}

type ResponsesResult struct {
	Id     string            `json:"id"`
	Output []ResponsesOutput `json:"output"`
	Usage  ResponsesUsage    `json:"usage"`
	Error  *any              `json:"error"`
}
type ResponsesUsage struct {
	InputTokens      int64               `json:"input_tokens"`
	PromptTokens     int64               `json:"prompt_tokens"`
	InputDetails     ResponsesInputUsage `json:"input_tokens_details"`
	OutputTokens     int64               `json:"output_tokens"`
	CompletionTokens int64               `json:"completion_tokens"`
}
type ResponsesInputUsage struct {
	CachedTokens int64 `json:"cached_tokens"`
}
type ResponsesOutput struct {
	Action    *ResponsesWebSearchAction `json:"action,omitempty"`
	Role      string                    `json:"role,omitempty"`
	Content   []ResponsesContent        `json:"content,omitempty"`
	Type      string                    `json:"type,omitempty"`
	Id        string                    `json:"id,omitempty"`
	Name      string                    `json:"name,omitempty"`
	CallId    string                    `json:"call_id,omitempty"`
	Arguments string                    `json:"arguments,omitempty"`
}
type ResponsesOutputToolCall struct {
	Id        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
type ResponsesContent struct {
	Typ         string                       `json:"type"`
	Text        string                       `json:"text,omitempty"`
	ImageUrl    string                       `json:"image_url,omitempty"`
	Annotations []ResponsesContentAnnotation `json:"annotations,omitempty"`
}
type ResponsesContentAnnotation struct {
	Typ string `json:"type"`
	Url string `json:"url"`
}

type ResponsesInput struct {
	Id         string             `json:"id,omitempty"`
	Type       string             `json:"type,omitempty"`
	Role       string             `json:"role,omitempty"`
	Name       string             `json:"name,omitempty"`
	Arguments  string             `json:"arguments,omitempty"`
	Status     string             `json:"status,omitempty"`
	Content    []ResponsesContent `json:"content,omitempty"`
	ToolCallId string             `json:"call_id,omitempty"`
	Output     any                `json:"output,omitempty"`
}
type ResponsesInputMessage struct {
	Role    string             `json:"role"`
	Content []ResponsesContent `json:"content"`
}
type ResponsesToolCallResult struct {
	ToolCallId string `json:"call_id,omitempty"`
	Output     any    `json:"output,omitempty"`
	Type       string `json:"type"`
}

type ResponsesStreamError struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}

// ResponsesStreamEvent is a partial representation of streaming SSE payloads.
// Fields are intentionally sparse; handlers should switch on Type.
type ResponsesStreamEvent struct {
	Type string `json:"type"`

	Annotation ResponsesContentAnnotation `json:"annotation"`
	Delta      string                     `json:"delta,omitempty"`
	CallID     string                     `json:"call_id,omitempty"`
	Item       *ResponsesOutput           `json:"item,omitempty"`
	ItemId     string                     `json:"item_id,omitempty"`
	Summary    []ResponsesSummary         `json:"summary,omitempty"`
	Part       ResponsesPartEvent         `json:"part"`
	Text       string                     `json:"text,omitempty"`

	ResponseID string           `json:"response_id,omitempty"`
	ID         string           `json:"id,omitempty"`
	Response   *ResponsesResult `json:"response,omitempty"`

	Error *ResponsesStreamError `json:"error,omitempty"`

	Raw json.RawMessage `json:"-"`
}

type ResponsesWebSearchAction struct {
	Type  string `json:"type"`
	Query string `json:"query"`
	Url   string `json:"url"`
}

type ResponsesPartEvent struct {
	Type        string                       `json:"type"`
	Text        string                       `json:"text"`
	Annotations []ResponsesContentAnnotation `json:"annotations,omitempty"`
}

type ResponsesSummary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
