package aikit

import (
	"encoding/json"
	"fmt"
)

type Thread struct {
	WebSearchEnabled   bool                                        `json:"web_search"`
	ReasoningEffort    string                                      `json:"reasoning_effort"`
	Tools              map[string]ToolDefinition                   `json:"tools"`
	HandleToolFunction func(name string, args json.RawMessage) any `json:"-"`

	Success bool        `json:"success"`
	Error   error       `json:"error,omitempty"`
	Result  ThreadUsage `json:"result"`

	Model      string `json:"model,omitempty"`
	ResponseID string `json:"response_id,omitempty"`

	Blocks []*ThreadBlock `json:"blocks"`

	incompleteToolCalls int
}

type ThreadUsage struct {
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
}

func (state *Thread) Debug() string {
	dbg := fmt.Sprintf("Thread: Success=%v, Error=%q, Model=%q\n", state.Success, state.Error, state.Model)
	for i, b := range state.Blocks {
		dbg += fmt.Sprintf(" Block %d: ID=%q, Type=%q, Complete=%v: '%s'\n", i, b.ID, b.Type, b.Complete, b.Text)
	}
	return dbg
}

func NewProviderState() *Thread {
	return &Thread{}
}

func NewProviderRequestState() *Thread {
	return NewProviderState()
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

func (s *Thread) Get(id string) *ThreadBlock {
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id {
			return s.Blocks[blockIdx]
		}
	}
	return nil
}
func (s *Thread) System(text string) {
	b := s.create("", InferenceBlockSystem)
	b.Text = text
}
func (s *Thread) Input(text string) {
	b := s.create("", InferenceBlockInput)
	b.Text = text
}
func (s *Thread) latestBlock(ofType ThreadBlockType) *ThreadBlock {
	blockIdx := len(s.Blocks) - 1
	for blockIdx > 0 {
		if s.Blocks[blockIdx].Type == ofType {
			return s.Blocks[blockIdx]
		}
		blockIdx--
	}
	return nil
}
func (s *Thread) Text(text string) {
	b := s.latestBlock(InferenceBlockText)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockText), InferenceBlockText)
	}
	b.Text += text
}
func (s *Thread) Cite(id string, citation string) {
	b := s.latestBlock(InferenceBlockText)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockText), InferenceBlockText)
	}
	b.Citations = append(b.Citations, citation)
}
func (s *Thread) Thinking(text string, signature string) {
	b := s.latestBlock(InferenceBlockThinking)
	if b == nil {
		b = s.create(s.NewBlockId(InferenceBlockThinking), InferenceBlockThinking)
	}
	b.Text += text
	b.Signature += signature
}
func (s *Thread) EncryptedThinking(text string) {
	b := s.create("", InferenceBlockEncryptedThinking)
	b.Text += text
}
func (s *Thread) ToolCall(id string, toolCallId string, name string, arguments json.RawMessage) {
	var b *ThreadBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockToolCall)
		s.incompleteToolCalls++
	}
	if b.ToolCall == nil {
		b.ToolCall = &ThreadToolCall{
			ID:        toolCallId,
			Name:      name,
			Arguments: arguments,
		}
	} else {
		b.ToolCall.Arguments = append(b.ToolCall.Arguments, arguments...)
	}
}
func (s *Thread) ToolCallWithThinking(id string, toolCallId string, name string, arguments json.RawMessage, thinkingText string, thinkingSignature string) {
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
			ID:        toolCallId,
			Name:      name,
			Arguments: arguments,
		}
		s.incompleteToolCalls++
	} else {
		b.ToolCall.Arguments = append(b.ToolCall.Arguments, arguments...)
	}
	b.Text = thinkingText
	b.Signature = thinkingSignature
}
func (s *Thread) ToolResult(toolCall *ThreadToolCall, output json.RawMessage) {
	s.incompleteToolCalls--
	b := s.create(s.NewBlockId(InferenceBlockToolResult), InferenceBlockToolResult)
	b.ToolResult = &ThreadToolResult{
		ToolCallID: toolCall.ID,
		Output:     output,
	}
	b.ToolCall = toolCall
}
