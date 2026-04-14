package aikit

// StreamState is the provider-facing view of a thread.
// API request implementations use this to read configuration,
// append streaming content, and update usage statistics.
type StreamState struct {
	data *thread
}

// --- Config read accessors ---

func (s *StreamState) Tools() map[string]ToolDefinition {
	return s.data.tools
}

func (s *StreamState) Reasoning() ReasoningConfig {
	return s.data.reasoning
}

func (s *StreamState) MaxWebSearches() int {
	return s.data.maxWebSearches
}

func (s *StreamState) WebFetchEnabled() bool {
	return s.data.webFetchEnabled
}

func (s *StreamState) Model() string {
	return s.data.model
}

func (s *StreamState) ThreadId() string {
	return s.data.threadId
}

func (s *StreamState) SetThreadId(id string) {
	s.data.threadId = id
}

func (s *StreamState) Blocks() []*ThreadBlock {
	return s.data.blocks
}

func (s *StreamState) CoalesceTextBlocks() bool {
	return s.data.coalesceTextBlocks
}

func (s *StreamState) UpdateOnFinalize() bool {
	return s.data.updateOnFinalize
}

func (s *StreamState) HandleToolFunction() func(name string, args string) string {
	return s.data.handleToolFunction
}

func (s *StreamState) CurrentProvider() string {
	return s.data.currentProvider
}

func (s *StreamState) SetCurrentProvider(name string) {
	s.data.currentProvider = name
}

func (s *StreamState) SetSuccess(val bool) {
	s.data.success = val
}

// --- Usage mutation ---

func (s *StreamState) AddInputTokens(n int64) {
	s.data.result.InputTokens += n
}

func (s *StreamState) AddOutputTokens(n int64) {
	s.data.result.OutputTokens += n
}

func (s *StreamState) AddCacheReadTokens(n int64) {
	s.data.result.CacheReadTokens += n
}

func (s *StreamState) AddCacheWriteTokens(n int64) {
	s.data.result.CacheWriteTokens += n
}

// --- State management ---

// TakeUpdate returns the current update flag and resets it to false.
func (s *StreamState) TakeUpdate() bool {
	if s.data.updated {
		s.data.updated = false
		return true
	}
	return false
}

// SetError sets the error message from an error and marks success as false.
func (s *StreamState) SetError(err error) {
	s.data.setError(err)
}

// IncompleteToolCalls returns the count of tool call blocks that are not yet complete.
func (s *StreamState) IncompleteToolCalls() int {
	count := 0
	for _, b := range s.data.blocks {
		if b.Type == InferenceBlockToolCall && !b.Complete {
			count++
		}
	}
	return count
}

// --- Structured output ---

func (s *StreamState) StructuredOutputStrictValue() bool {
	if s.data.structuredOutputStrict != nil {
		return *s.data.structuredOutputStrict
	}
	return true
}

func (s *StreamState) StructuredOutputSchemaValue() *JsonSchema {
	if s.data.structuredOutputSchema == nil {
		return nil
	}
	return PrepareStructuredOutputSchema(s.data.structuredOutputSchema, s.StructuredOutputStrictValue(), true)
}

func (s *StreamState) StructuredOutputSchemaValueWithoutAdditionalProperties() *JsonSchema {
	if s.data.structuredOutputSchema == nil {
		return nil
	}
	return PrepareStructuredOutputSchema(s.data.structuredOutputSchema, s.StructuredOutputStrictValue(), false)
}

func (s *StreamState) StructuredOutputFormat() *JsonSchemaResponseFormat {
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

func (s *StreamState) StructuredOutputTextFormat() *ResponsesTextFormat {
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

// --- Block management ---

func (s *StreamState) AppendBlock(typ ThreadBlockType) string {
	return s.data.appendBlockId(typ)
}

func (s *StreamState) AppendCoalesce(id string, typ ThreadBlockType) *ThreadBlock {
	return s.data.appendCoalesce(id, typ)
}

func (s *StreamState) CompleteBlock(id string) {
	s.data.completeBlock(id)
}

// --- Append methods ---

func (s *StreamState) AppendText(id string, text string) {
	if text == "" {
		return
	}
	b := s.data.findOrCreateIDBlock(id, InferenceBlockText)
	b.Text += text
	s.data.updated = true
}

func (s *StreamState) AppendCite(id string, citation string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockText)
	b.Citations = append(b.Citations, citation)
	s.data.updated = true
}

func (s *StreamState) AppendThinking(id string, text string) {
	if text == "" {
		return
	}
	b := s.data.findOrCreateIDBlock(id, InferenceBlockThinking)
	b.Text += text
	s.data.updated = true
}

func (s *StreamState) AppendThinkingWithSignature(id string, thinking string, signature string) {
	if thinking == "" && signature == "" {
		return
	}
	b := s.data.findOrCreateIDBlock(id, InferenceBlockThinking)
	b.Text += thinking
	b.Signature += signature
	s.data.updated = true
}

func (s *StreamState) AppendThinkingSignature(id string, signature string) {
	if signature == "" {
		return
	}
	b := s.data.findOrCreateIDBlock(id, InferenceBlockThinking)
	b.Signature += signature
	s.data.updated = true
}

func (s *StreamState) AppendEncryptedThinking(text string) {
	b := s.data.create("", InferenceBlockEncryptedThinking)
	b.Text += text
}

func (s *StreamState) AppendToolCall(id string, name string, arguments string) {
	var b *ThreadBlock
	for blockIdx := range s.data.blocks {
		if s.data.blocks[blockIdx].ID == id && s.data.blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.data.blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.data.create(id, InferenceBlockToolCall)
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.data.updated = true
	} else if b.ToolCall == nil {
		b.ToolCall = &ThreadToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}
		s.data.updated = true
	} else if arguments != "" {
		b.ToolCall.Arguments += arguments
		s.data.updated = true
	}
}

func (s *StreamState) AppendToolCallWithThinking(id string, name string, arguments string, thinkingText string, thinkingSignature string) {
	var b *ThreadBlock
	for blockIdx := range s.data.blocks {
		if s.data.blocks[blockIdx].ID == id && s.data.blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.data.blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.data.create(id, InferenceBlockToolCall)
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
	s.data.updated = true
}

func (s *StreamState) AppendToolResult(toolCall *ThreadToolCall, output string) {
	b := s.data.getType(toolCall.ID, InferenceBlockToolCall)
	if b != nil {
		b.ToolResult = &ThreadToolResult{
			ToolCallID: toolCall.ID,
			Output:     output,
		}
		b.Complete = true
		s.data.updated = true
	}
}

func (s *StreamState) AppendWebSearch(id string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch = &ThreadWebSearch{
		Results: []ThreadWebSearchResult{},
	}
	s.data.updated = true
}

func (s *StreamState) AppendWebSearchQuery(id string, query string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch = &ThreadWebSearch{
		Query: query,
	}
	s.AppendCompleteWebSearch(id)
}

func (s *StreamState) AppendWebSearchResult(id string, result ThreadWebSearchResult) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.WebSearch.Results = append(b.WebSearch.Results, result)
}

func (s *StreamState) AppendCompleteWebSearch(id string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockWebSearch)
	b.Complete = true
	s.data.result.WebSearches++
	s.data.updated = true
}

func (s *StreamState) AppendViewWebpage(id string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockViewWebpage)
	b.Complete = false
}

func (s *StreamState) AppendViewWebpageUrl(id string, url string) {
	b := s.data.findOrCreateIDBlock(id, InferenceBlockViewWebpage)
	b.Text = url
	b.Complete = true
	s.data.result.PageViews++
	s.data.updated = true
}
