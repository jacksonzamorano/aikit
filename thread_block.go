package aikit

import (
	"fmt"
	"strings"
)

type ThreadBlockType string

const (
	InferenceBlockSystem            ThreadBlockType = "system"
	InferenceBlockInput             ThreadBlockType = "input"
	InferenceBlockThinking          ThreadBlockType = "thinking"
	InferenceBlockEncryptedThinking ThreadBlockType = "encrypted_thinking"
	InferenceBlockText              ThreadBlockType = "text"
	InferenceBlockToolCall          ThreadBlockType = "tool_call"
	InferenceBlockWebSearch         ThreadBlockType = "web_search"
	InferenceBlockViewWebpage       ThreadBlockType = "view_webpage"
)

type ThreadToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ThreadToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
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
	ID   string          `json:"id,omitempty"`
	Type ThreadBlockType `json:"type"`

	AliasId  string       `json:"alias_id,omitempty"`
	AliasFor *ThreadBlock `json:"-"`

	Text       string            `json:"text,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	ToolCall   *ThreadToolCall   `json:"tool_call,omitempty"`
	ToolResult *ThreadToolResult `json:"tool_result,omitempty"`
	WebSearch  *ThreadWebSearch  `json:"web_search,omitempty"`
	Complete   bool              `json:"complete"`
	Citations  []string          `json:"citations,omitempty"`
}

func (b *ThreadBlock) Description() string {
	switch b.Type {
	case InferenceBlockSystem:
		return "| System: " + strings.ReplaceAll(b.Text, "\n", "\n|\t")
	case InferenceBlockInput:
		return "\n> " + strings.ReplaceAll(b.Text, "\n", "\n|\t")
	case InferenceBlockThinking:
		return "| Thinking: " + strings.ReplaceAll(b.Text, "\n", "\n|\t")
	case InferenceBlockEncryptedThinking:
		return "| Thinking: [redacted]"
	case InferenceBlockText:
		return "\n\n" + b.Text
	case InferenceBlockToolCall:
		return fmt.Sprintf("-> %s\n<- %s", b.ToolCall.Name, string(b.ToolResult.Output))
	case InferenceBlockWebSearch:
		return fmt.Sprintf("| Searched for '%s'", b.WebSearch.Query)
	case InferenceBlockViewWebpage:
		return fmt.Sprintf("| Viewed '%s'", b.Text)
	default:
		return ""
	}
}
