package aikit

import (
	"encoding/base64"
	"fmt"
)

type Thread struct {
	ReasoningEffort    string                                `json:"reasoning_effort"`
	Tools              map[string]ToolDefinition             `json:"tools"`
	MaxWebSearches     int                                   `json:"max_web_searches"`
	WebFetchEnabled    bool                                  `json:"web_fetch_enabled"`
	HandleToolFunction func(name string, args string) string `json:"-"`
	UpdateOnFinalize   bool                                  `json:"update_on_finalize"`
	CoalesceTextBlocks bool                                  `json:"coalesce_text_blocks"`

	Success bool        `json:"success"`
	Error   error       `json:"error,omitempty"`
	Result  ThreadUsage `json:"result"`

	Model    string `json:"model,omitempty"`
	ThreadId string `json:"thread_id,omitempty"`

	Blocks []*ThreadBlock `json:"blocks"`

	Updated bool `json:"-"`

	incompleteToolCalls int
}

type ThreadUsage struct {
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	WebSearches      int   `json:"web_searches"`
	PageViews        int   `json:"page_views"`
}

func (state *Thread) Debug() string {
	dbg := fmt.Sprintf("Thread: Success=%v, Error=%q, Model=%q\n", state.Success, state.Error, state.Model)
	for i, b := range state.Blocks {
		dbg += fmt.Sprintf(" Block %d: ID=%q, Type=%q, Complete=%v: '%s'\n", i, b.ID, b.Type, b.Complete, b.Text)
	}
	return dbg
}

func (state *Thread) PrintDescription() {
	for _, b := range state.Blocks {
		println(b.Description())
	}
}

func NewProviderState() *Thread {
	return &Thread{}
}

func (s *Thread) create(id string, typ ThreadBlockType) *ThreadBlock {
	b := &ThreadBlock{
		ID:   id,
		Type: typ,
	}
	s.Blocks = append(s.Blocks, b)
	return b
}

func (s *Thread) NewBlockId(typ ThreadBlockType) string {
	return fmt.Sprintf("%s-%d", typ, len(s.Blocks)+1)
}

func (s *Thread) Complete(id string) {
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id {
			s.Blocks[blockIdx].Complete = true
			if s.UpdateOnFinalize {
				s.Updated = true
			}
		}
	}
}
func (s *Thread) getType(id string, ofType ThreadBlockType) *ThreadBlock {
	blockIdx := len(s.Blocks) - 1
	for blockIdx >= 0 {
		if s.Blocks[blockIdx].Type == ofType && s.Blocks[blockIdx].ID == id {
			return s.Blocks[blockIdx]
		}
		blockIdx--
	}
	return nil
}
func (s *Thread) System(text string) {
	b := s.create("", InferenceBlockSystem)
	b.Text = text
	b.Complete = true
}
func (s *Thread) Input(text string) {
	b := s.create("", InferenceBlockInput)
	b.Text = text
	b.Complete = true
}

// InputImage adds an image to the thread using raw bytes.
// The bytes are immediately encoded to base64 and stored.
// mediaType should be a valid MIME type (e.g., "image/jpeg", "image/png", "image/gif", "image/webp").
func (s *Thread) InputImage(data []byte, mediaType string) {
	b := s.create("", InferenceBlockInputImage)
	b.Image = &ThreadImage{
		Base64:    base64.StdEncoding.EncodeToString(data),
		MediaType: mediaType,
	}
	b.Complete = true
}

// InputImageBase64 adds an image to the thread using a pre-encoded base64 string.
// mediaType should be a valid MIME type (e.g., "image/jpeg", "image/png", "image/gif", "image/webp").
func (s *Thread) InputImageBase64(base64Data string, mediaType string) {
	b := s.create("", InferenceBlockInputImage)
	b.Image = &ThreadImage{
		Base64:    base64Data,
		MediaType: mediaType,
	}
	b.Complete = true
}
func (s *Thread) Text(id string, text string) {
	if text == "" {
		return
	}
	b := s.findOrCreateIDBlock(id, InferenceBlockText)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockText), InferenceBlockText)
	}
	b.Text += text
	s.Updated = true
}
func (s *Thread) Coalesce(id string, typ ThreadBlockType) *ThreadBlock {
	searchIdx := len(s.Blocks) - 1
	if searchIdx < 0 {
		return nil
	}
	if s.Blocks[searchIdx].Type != typ {
		return nil
	}

	og_block := s.Blocks[searchIdx]
	if og_block.AliasFor != nil {
		og_block = og_block.AliasFor
	}

	b := s.create(id, typ)
	b.AliasFor = og_block
	b.AliasId = og_block.ID
	return b
}
func (s *Thread) Cite(id string, citation string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockText)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockText), InferenceBlockText)
	}
	b.Citations = append(b.Citations, citation)
	s.Updated = true
}
func (s *Thread) Thinking(id string, text string) {
	if text == "" {
		return
	}
	b := s.findOrCreateIDBlock(id, InferenceBlockThinking)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockThinking), InferenceBlockThinking)
	}
	b.Text += text
	s.Updated = true
}
func (s *Thread) ThinkingWithSignature(id string, thinking string, signature string) {
	if thinking == "" && signature == "" {
		return
	}
	b := s.findOrCreateIDBlock(id, InferenceBlockThinking)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockThinking), InferenceBlockThinking)
	}
	b.Text += thinking
	b.Signature += signature
	s.Updated = true
}
func (s *Thread) ThinkingSignature(id string, signature string) {
	if signature == "" {
		return
	}
	b := s.findOrCreateIDBlock(id, InferenceBlockThinking)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockThinking), InferenceBlockThinking)
	}
	b.Signature += signature
	s.Updated = true
}
func (s *Thread) EncryptedThinking(text string) {
	b := s.create("", InferenceBlockEncryptedThinking)
	b.Text += text
}
func (s *Thread) ToolCall(id string, name string, arguments string) {
	var b *ThreadBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockToolCall)
		s.incompleteToolCalls++
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.Updated = true
	} else if b.ToolCall == nil {
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.Updated = true
	} else if arguments != "" {
		b.ToolCall.Arguments += arguments
		s.Updated = true
	}
}
func (s *Thread) ToolCallWithThinking(id string, name string, arguments string, thinkingText string, thinkingSignature string) {
	var b *ThreadBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockToolCall)
	}
	if b.ToolCall == nil {
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.incompleteToolCalls++
	} else {
		b.ToolCall.Arguments += arguments
	}
	b.Text = thinkingText
	b.Signature = thinkingSignature
	s.Updated = true
}
func (s *Thread) ToolResult(toolCall *ThreadToolCall, output string) {
	s.incompleteToolCalls--
	b := s.getType(toolCall.ID, InferenceBlockToolCall)
	if b != nil {
		b.ToolResult = &ThreadToolResult{
			ToolCallID: toolCall.ID,
			Output:     output,
		}
		b.Complete = true
		s.Updated = true
	}
}
func (s *Thread) findOrCreateIDBlock(id string, typ ThreadBlockType) *ThreadBlock {
	blockIdx := len(s.Blocks) - 1
	for blockIdx >= 0 {
		if s.Blocks[blockIdx].Type == typ && s.Blocks[blockIdx].ID == id {
			if s.Blocks[blockIdx].AliasFor != nil {
				return s.Blocks[blockIdx].AliasFor
			}
			return s.Blocks[blockIdx]
		}
		blockIdx--
	}
	if s.CoalesceTextBlocks && typ == InferenceBlockText {
		if block := s.Coalesce(id, typ); block != nil {
			return block
		}
	}
	return s.create(id, typ)
}
func (s *Thread) WebSearch(id string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch = &ThreadWebSearch{
		Results: []ThreadWebSearchResult{},
	}
	s.Updated = true
}
func (s *Thread) WebSearchQuery(id string, query string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch = &ThreadWebSearch{
		Query: query,
	}
	s.CompleteWebSearch(id)
}
func (s *Thread) WebSearchResult(id string, result ThreadWebSearchResult) {
	b := s.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch.Results = append(b.WebSearch.Results, result)
}
func (s *Thread) CompleteWebSearch(id string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.Complete = true
	s.Result.WebSearches++
	s.Updated = true
}
func (s *Thread) ViewWebpage(id string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockViewWebpage)
	b.Complete = false
}
func (s *Thread) ViewWebpageUrl(id string, url string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockViewWebpage)
	b.Text = url
	b.Complete = true
	s.Result.PageViews++
	s.Updated = true
}
