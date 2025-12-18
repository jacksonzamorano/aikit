package aikit

import "encoding/json"

type ResponsesResult struct {
	Id     string            `json:"id"`
	Output []ResponsesOutput `json:"output"`
	Usage  ResponsesUsage    `json:"usage"`
	Error  *any              `json:"error"`
}
type ResponsesUsage struct {
	InputTokens  int64               `json:"input_tokens"`
	InputDetails ResponsesInputUsage `json:"input_tokens_details"`
	OutputTokens int64               `json:"output_tokens"`
}
type ResponsesInputUsage struct {
	CachedTokens int64 `json:"cached_tokens"`
}
type ResponsesOutput struct {
	Action    *ResponsesOutputAction `json:"action,omitempty"`
	Role      string                 `json:"role,omitempty"`
	Content   []ResponsesContent     `json:"content,omitempty"`
	Type      string                 `json:"type,omitempty"`
	Id        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	CallId    string                 `json:"call_id,omitempty"`
	Arguments json.RawMessage        `json:"arguments,omitempty"`
}
type ResponsesOutputAction struct {
	Type string `json:"type,omitempty"`
}
type ResponsesOutputToolCall struct {
	Id        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
type ResponsesContent struct {
	Typ         string                       `json:"type"`
	Text        string                       `json:"text"`
	Annotations []ResponsesContentAnnotation `json:"annotations,omitempty"`
}
type ResponsesContentAnnotation struct {
	Typ string `json:"type"`
	Url string `json:"url"`
}

type ResponsesInput struct {
	Type    string             `json:"type,omitempty"`
	Role    string             `json:"role"`
	Content []ResponsesContent `json:"content"`
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

	Delta   string             `json:"delta,omitempty"`
	CallID  string             `json:"call_id,omitempty"`
	Item    *ResponsesOutput   `json:"item,omitempty"`
	ItemId  string             `json:"item_id,omitempty"`
	Summary []ResponsesSummary `json:"summary,omitempty"`
	Part    ResponsesPartEvent `json:"part,omitempty"`

	ResponseID string           `json:"response_id,omitempty"`
	ID         string           `json:"id,omitempty"`
	Response   *ResponsesResult `json:"response,omitempty"`

	Error *ResponsesStreamError `json:"error,omitempty"`

	Raw json.RawMessage `json:"-"`
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
