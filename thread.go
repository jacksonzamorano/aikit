package aikit

import (
	"encoding/base64"
	"fmt"
)

// ReasoningConfig configures reasoning behavior for the thread.
// Effort is for string-based providers (e.g., OpenAI's "low"/"medium"/"high").
// Budget is for integer-based token budgets.
type ReasoningConfig struct {
	Effort string `json:"effort,omitempty" xml:"effort,attr,omitempty"`
	Budget int    `json:"budget,omitempty" xml:"budget,attr,omitempty"`
}

type Thread struct {
	Reasoning          ReasoningConfig                       `json:"reasoning" xml:"reasoning"`
	Tools              map[string]ToolDefinition             `json:"tools" xml:"tools"`
	MaxWebSearches     int                                   `json:"max_web_searches" xml:"max_web_searches,attr"`
	WebFetchEnabled    bool                                  `json:"web_fetch_enabled" xml:"web_fetch_enabled,attr"`
	HandleToolFunction func(name string, args string) string `json:"-" xml:"-"`
	UpdateOnFinalize   bool                                  `json:"update_on_finalize" xml:"update_on_finalize,attr"`
	CoalesceTextBlocks bool                                  `json:"coalesce_text_blocks" xml:"coalesce_text_blocks,attr"`

	Success bool        `json:"success" xml:"success,attr"`
	Error   string      `json:"error,omitempty" xml:"error,omitempty"`
	Result  ThreadUsage `json:"result" xml:"result"`

	Model    string `json:"model,omitempty" xml:"model,attr,omitempty"`
	ThreadId string `json:"thread_id,omitempty" xml:"thread_id,attr,omitempty"`

	Blocks []*ThreadBlock `json:"blocks" xml:"blocks>block"`

	updated         bool   `json:"-" xml:"-"`
	CurrentProvider string `json:"-" xml:"-"`
}

// TakeUpdate returns the current update flag and resets it to false.
// This is used to check if the thread was modified since the last check.
func (s *Thread) TakeUpdate() bool {
	if s.updated {
		s.updated = false
		return true
	}
	return false
}

// Snapshot represents a serializable snapshot of a Thread's conversation blocks.
type Snapshot struct {
	Blocks []*ThreadBlock `json:"blocks" xml:"blocks>block"`
}

type ThreadUsage struct {
	CacheReadTokens  int64 `json:"cache_read_tokens" xml:"cache_read_tokens,attr"`
	CacheWriteTokens int64 `json:"cache_write_tokens" xml:"cache_write_tokens,attr"`
	InputTokens      int64 `json:"input_tokens" xml:"input_tokens,attr"`
	OutputTokens     int64 `json:"output_tokens" xml:"output_tokens,attr"`
	WebSearches      int   `json:"web_searches" xml:"web_searches,attr"`
	PageViews        int   `json:"page_views" xml:"page_views,attr"`
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

// SetError sets the error message from an error and marks success as false.
func (s *Thread) SetError(err error) {
	s.Error = err.Error()
	s.Success = false
}

// IncompleteToolCalls returns the count of tool call blocks that are not yet complete.
func (s *Thread) IncompleteToolCalls() int {
	count := 0
	for _, b := range s.Blocks {
		if b.Type == InferenceBlockToolCall && !b.Complete {
			count++
		}
	}
	return count
}

// Snapshot creates a serializable snapshot of the Thread's conversation blocks.
func (s *Thread) Snapshot() *Snapshot {
	return &Snapshot{
		Blocks: s.Blocks,
	}
}

// Restore restores the Thread's blocks from a snapshot.
func (s *Thread) Restore(snapshot *Snapshot) {
	s.Blocks = make([]*ThreadBlock, len(snapshot.Blocks))
	copy(s.Blocks, snapshot.Blocks)
}

func (s *Thread) create(id string, typ ThreadBlockType) *ThreadBlock {
	b := &ThreadBlock{
		ID:         id,
		Type:       typ,
		ProviderID: s.CurrentProvider,
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
				s.updated = true
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
	s.updated = true
}
func (s *Thread) Coalesce(id string, typ ThreadBlockType) *ThreadBlock {
	searchIdx := len(s.Blocks) - 1
	if searchIdx < 0 {
		return nil
	}
	if s.Blocks[searchIdx].Type != typ {
		return nil
	}

	// Mark the previous block as continued
	s.Blocks[searchIdx].Continued = true

	// Create the new block
	b := s.create(id, typ)
	return b
}
func (s *Thread) Cite(id string, citation string) {
	b := s.findOrCreateIDBlock(id, InferenceBlockText)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockText), InferenceBlockText)
	}
	b.Citations = append(b.Citations, citation)
	s.updated = true
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
	s.updated = true
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
	s.updated = true
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
	s.updated = true
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
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.updated = true
	} else if b.ToolCall == nil {
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.updated = true
	} else if arguments != "" {
		b.ToolCall.Arguments += arguments
		s.updated = true
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
	} else {
		b.ToolCall.Arguments += arguments
	}
	b.Text = thinkingText
	b.Signature = thinkingSignature
	s.updated = true
}
func (s *Thread) ToolResult(toolCall *ThreadToolCall, output string) {
	b := s.getType(toolCall.ID, InferenceBlockToolCall)
	if b != nil {
		b.ToolResult = &ThreadToolResult{
			ToolCallID: toolCall.ID,
			Output:     output,
		}
		b.Complete = true
		s.updated = true
	}
}
func (s *Thread) findOrCreateIDBlock(id string, typ ThreadBlockType) *ThreadBlock {
	blockIdx := len(s.Blocks) - 1
	for blockIdx >= 0 {
		if s.Blocks[blockIdx].Type == typ && s.Blocks[blockIdx].ID == id {
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
	s.updated = true
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
	s.updated = true
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
	s.updated = true
}
