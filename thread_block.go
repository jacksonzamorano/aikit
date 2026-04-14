package aikit

import "fmt"

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

// ThreadImage represents image data for vision input.
// Images are always stored as base64-encoded strings.
type ThreadImage struct {
	Base64    string `json:"base64"`
	MediaType string `json:"media_type"`
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
	ID   string          `json:"id,omitempty"`
	Type ThreadBlockType `json:"type"`

	Text       string            `json:"text,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	ToolCall   *ThreadToolCall   `json:"tool_call,omitempty"`
	ToolResult *ThreadToolResult `json:"tool_result,omitempty"`
	WebSearch  *ThreadWebSearch  `json:"web_search,omitempty"`
	Image      *ThreadImage      `json:"image,omitempty"`
	Complete   bool              `json:"complete"`
	Continued  bool              `json:"continued,omitempty"`
	Citations  []string          `json:"citations,omitempty"`
	ProviderID string            `json:"provider_id,omitempty"`
}
