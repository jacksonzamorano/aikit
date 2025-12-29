package aikit

import (
	"fmt"
	"strings"
)

type ThreadBlockType string

const (
	InferenceBlockSystem            ThreadBlockType = "system"
	InferenceBlockInput             ThreadBlockType = "input"
	InferenceBlockInputImage        ThreadBlockType = "input_image"
	InferenceBlockThinking          ThreadBlockType = "thinking"
	InferenceBlockEncryptedThinking ThreadBlockType = "encrypted_thinking"
	InferenceBlockText              ThreadBlockType = "text"
	InferenceBlockToolCall          ThreadBlockType = "tool_call"
	InferenceBlockWebSearch         ThreadBlockType = "web_search"
	InferenceBlockViewWebpage       ThreadBlockType = "view_webpage"
)

type ThreadToolCall struct {
	ID        string `json:"id" xml:"id,attr"`
	Name      string `json:"name" xml:"name,attr"`
	Arguments string `json:"arguments" xml:"arguments"`
}

type ThreadToolResult struct {
	ToolCallID string `json:"tool_call_id" xml:"tool_call_id,attr"`
	Output     string `json:"output" xml:"output"`
}

type ThreadWebSearch struct {
	Query   string                  `json:"query" xml:"query,attr"`
	Results []ThreadWebSearchResult `json:"results" xml:"results>result"`
}

type ThreadWebSearchResult struct {
	Title string `json:"title" xml:"title,attr"`
	URL   string `json:"url" xml:"url,attr"`
}

// ThreadImage represents image data for vision input.
// Images are always stored as base64-encoded strings.
type ThreadImage struct {
	Base64    string `json:"base64" xml:"base64"`
	MediaType string `json:"media_type" xml:"media_type,attr"`
}

// GetBase64 returns the base64-encoded image data.
func (img *ThreadImage) GetBase64() string {
	return img.Base64
}

// GetDataURL returns a data URL suitable for OpenAI-style APIs.
// Format: "data:image/jpeg;base64,<base64data>"
func (img *ThreadImage) GetDataURL() string {
	return fmt.Sprintf("data:%s;base64,%s", img.MediaType, img.GetBase64())
}

type ThreadBlock struct {
	ID   string          `json:"id,omitempty" xml:"id,attr,omitempty"`
	Type ThreadBlockType `json:"type" xml:"type,attr"`

	Text       string            `json:"text,omitempty" xml:"text,omitempty"`
	Signature  string            `json:"signature,omitempty" xml:"signature,omitempty"`
	ToolCall   *ThreadToolCall   `json:"tool_call,omitempty" xml:"tool_call,omitempty"`
	ToolResult *ThreadToolResult `json:"tool_result,omitempty" xml:"tool_result,omitempty"`
	WebSearch  *ThreadWebSearch  `json:"web_search,omitempty" xml:"web_search,omitempty"`
	Image      *ThreadImage      `json:"image,omitempty" xml:"image,omitempty"`
	Complete   bool              `json:"complete" xml:"complete,attr"`
	Continued  bool              `json:"continued,omitempty" xml:"continued,attr,omitempty"`
	Citations  []string          `json:"citations,omitempty" xml:"citations>citation,omitempty"`
	ProviderID string            `json:"provider_id,omitempty" xml:"provider_id,attr,omitempty"`
}

func (b *ThreadBlock) Description() string {
	switch b.Type {
	case InferenceBlockSystem:
		return "| System: " + strings.ReplaceAll(b.Text, "\n", "\n|\t")
	case InferenceBlockInput:
		return "\n> " + strings.ReplaceAll(b.Text, "\n", "\n|\t")
	case InferenceBlockInputImage:
		if b.Image != nil {
			size := len(b.Image.Base64) * 3 / 4 // Approximate decoded size
			return fmt.Sprintf("| Image: [%s, ~%d bytes]", b.Image.MediaType, size)
		}
		return "| Image: [embedded]"
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
