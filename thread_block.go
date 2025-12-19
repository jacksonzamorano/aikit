package aikit

import "encoding/json"

type ThreadBlockType string

const (
	InferenceBlockSystem            ThreadBlockType = "system"
	InferenceBlockInput             ThreadBlockType = "input"
	InferenceBlockThinking          ThreadBlockType = "thinking"
	InferenceBlockEncryptedThinking ThreadBlockType = "encrypted_thinking"
	InferenceBlockText              ThreadBlockType = "text"
	InferenceBlockToolCall          ThreadBlockType = "tool_call"
	InferenceBlockToolResult        ThreadBlockType = "tool_result"
	InferenceBlockWebSearch         ThreadBlockType = "web_search"
	InferenceBlockViewWebpage       ThreadBlockType = "view_webpage"
)

type ThreadToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"-"`
}

type ThreadToolResult struct {
	ToolCallID string          `json:"tool_call_id"`
	Output     json.RawMessage `json:"-"`
}

type ThreadWebSearch struct {
	Query   string                  `json:"query"`
	Results []ThreadWebSearchResult `json:"results"`
}

type ThreadWebSearchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ThreadBlock struct {
	ID         string            `json:"id"`
	Type       ThreadBlockType   `json:"type"`
	Text       string            `json:"text,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	ToolCall   *ThreadToolCall   `json:"tool_call,omitempty"`
	ToolResult *ThreadToolResult `json:"tool_result,omitempty"`
	WebSearch  *ThreadWebSearch  `json:"web_search,omitempty"`
	Complete   bool              `json:"complete"`
	Citations  []string          `json:"citations,omitempty"`
}
