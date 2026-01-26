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
	// Configuration
	Reasoning              ReasoningConfig                       `json:"reasoning"`
	Tools                  map[string]ToolDefinition             `json:"tools"`
	StructuredOutputSchema *JsonSchema                           `json:"schema"`
	StructuredOutputStrict *bool                                 `json:"schema_strict"`
	MaxWebSearches         int                                   `json:"max_web_searches"`
	WebFetchEnabled        bool                                  `json:"web_fetch_enabled"`
	HandleToolFunction     func(name string, args string) string `json:"-"`
	UpdateOnFinalize       bool                                  `json:"update_on_finalize"`
	CoalesceTextBlocks     bool                                  `json:"coalesce_text_blocks"`

	Success bool
	Error   string
	Result  ThreadUsage

	Model    string `json:"model"`
	ThreadId string `json:"thread_id"`

	Blocks []*ThreadBlock `json:"blocks"`

	updated         bool
	CurrentProvider string
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
// This is the primary serialization mechanism for persisting conversation history.
//
// Usage:
//
//	// Save conversation
//	snapshot := thread.Snapshot()
//	data, _ := json.Marshal(snapshot)
//
//	// Restore conversation
//	var restored Snapshot
//	json.Unmarshal(data, &restored)
//	newThread.Restore(&restored)
//	// Re-configure: newThread.Model, newThread.Tools, etc.
type Snapshot struct {
	Blocks []*ThreadBlock `json:"blocks" xml:"blocks>block"`
}

// ThreadUsage tracks token and resource usage from inference calls.
type ThreadUsage struct {
	CacheReadTokens  int64
	CacheWriteTokens int64
	InputTokens      int64
	OutputTokens     int64
	WebSearches      int
	PageViews        int
}

func NewProviderState() *Thread {
	return &Thread{}
}

// SetError sets the error message from an error and marks success as false.
func (s *Thread) SetError(err error) {
	s.Error = err.Error()
	s.Success = false
}

func (s *Thread) StructuredOutputStrictValue() bool {
	if s.StructuredOutputStrict != nil {
		return *s.StructuredOutputStrict
	}
	return true
}

func (s *Thread) StructuredOutputSchemaValue() *JsonSchema {
	if s.StructuredOutputSchema == nil {
		return nil
	}
	return PrepareStructuredOutputSchema(s.StructuredOutputSchema, s.StructuredOutputStrictValue(), true)
}

func (s *Thread) StructuredOutputSchemaValueWithoutAdditionalProperties() *JsonSchema {
	if s.StructuredOutputSchema == nil {
		return nil
	}
	return PrepareStructuredOutputSchema(s.StructuredOutputSchema, s.StructuredOutputStrictValue(), false)
}

func (s *Thread) StructuredOutputFormat() *JsonSchemaResponseFormat {
	schema := s.StructuredOutputSchemaValue()
	if schema == nil {
		return nil
	}
	return &JsonSchemaResponseFormat{
		Type: "json_schema",
		JsonSchema: JsonSchemaDescription{
			Name:   "response",
			Schema: schema,
			Strict: s.StructuredOutputStrictValue(),
		},
	}
}

func (s *Thread) StructuredOutputTextFormat() *ResponsesTextFormat {
	schema := s.StructuredOutputSchemaValue()
	if schema == nil {
		return nil
	}
	return &ResponsesTextFormat{
		Type:   "json_schema",
		Name:   "response",
		Schema: schema,
		Strict: s.StructuredOutputStrictValue(),
	}
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
