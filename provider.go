package aikit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// InferenceClient is the default HTTP client used by providers when they are not
// configured with a custom *http.Client.
//
// This is intentionally a value (not a pointer) so callers can override fields
// like Transport/Timeout without additional allocation.
var InferenceClient http.Client

type InferenceRequest struct {
	Model           string
	ReasoningEffort *string
	SystemPrompt    string
	Prompt          string
	Tools           map[string]ToolDefinition
	MaxWebSearches  int
	ToolHandler     func(string, json.RawMessage) any
}

// NewInferenceRequest returns an InferenceRequest with all optional fields left
// unset (nil), including MaxWebSearches.
func NewInferenceRequest() *InferenceRequest {
	return &InferenceRequest{}
}

type InferenceResult struct {
	Success      bool
	Output       string
	Citations    []string
	Data         []byte
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	ExtraUsage   float64
}

func sanitizeInput(input string) string {
	return strings.ReplaceAll(input, "\r\n", "\n")
}
func (ir *InferenceRequest) PrepareTemplate() string {
	return sanitizeInput(ir.Prompt)
}

func reasoningEffortValue(reasoningEffort *string) (string, bool) {
	if reasoningEffort == nil {
		return "", false
	}
	v := strings.TrimSpace(*reasoningEffort)
	if v == "" || strings.EqualFold(v, "disabled") {
		return "", false
	}
	return v, true
}

type InferenceBlockType string

const (
	InferenceBlockSystem            InferenceBlockType = "system"
	InferenceBlockInput             InferenceBlockType = "input"
	InferenceBlockThinking          InferenceBlockType = "thinking"
	InferenceBlockEncryptedThinking InferenceBlockType = "encrypted_thinking"
	InferenceBlockText              InferenceBlockType = "text"
	InferenceBlockToolCall          InferenceBlockType = "tool_call"
	InferenceBlockToolResult        InferenceBlockType = "tool_result"
)

type InferenceToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type InferenceToolResult struct {
	ToolCallID string
	Output     json.RawMessage
}

type InferenceBlock struct {
	ID         string
	Type       InferenceBlockType
	Text       string
	Signature  string
	ToolCall   *InferenceToolCall
	ToolResult *InferenceToolResult
	Complete   bool
	Citations  []string
}

type ProviderState struct {
	WebSearchEnabled bool             `json:"web_search"`
	ReasoningEffort  string           `json:"reasoning_effort"`
	Tools            []ToolDefinition `json:"tools"`

	Success bool                `json:"success"`
	Error   *ProviderError      `json:"error,omitempty"`
	Result  ProviderStateResult `json:"result"`

	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	ResponseID string `json:"response_id,omitempty"`

	Blocks []*InferenceBlock `json:"blocks"`
}

type ProviderStateResult struct {
	CacheReadTokens  int64
	CacheWriteTokens int64
	InputTokens      int64
	OutputTokens     int64
}

func (state *ProviderState) Debug() string {
	dbg := fmt.Sprintf("ProviderState: Success=%v, Error=%q, Provider=%q, Model=%q\n", state.Success, state.Error, state.Provider, state.Model)
	for i, b := range state.Blocks {
		dbg += fmt.Sprintf(" Block %d: ID=%q, Type=%q, Complete=%v: '%s'\n", i, b.ID, b.Type, b.Complete, b.Text)
	}
	return dbg
}

func NewProviderState() *ProviderState {
	return &ProviderState{}
}

func NewProviderRequestState() *ProviderState {
	return NewProviderState()
}

func (s *ProviderState) create(id string, typ InferenceBlockType) *InferenceBlock {
	b := &InferenceBlock{
		ID:   id,
		Type: typ,
	}
	s.Blocks = append(s.Blocks, b)
	return b
}

func (s *ProviderState) Get(id string) *InferenceBlock {
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id {
			return s.Blocks[blockIdx]
		}
	}
	return nil
}
func (s *ProviderState) System(text string) {
	b := s.create("", InferenceBlockSystem)
	b.Text = text
}
func (s *ProviderState) Input(text string) {
	b := s.create("", InferenceBlockInput)
	b.Text = text
}
func (s *ProviderState) Text(id string, text string) {
	var b *InferenceBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockText {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockText)
	}
	b.Text += text
}
func (s *ProviderState) Cite(id string, citation string) {
	var b *InferenceBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockText {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockText)
	}
	b.Citations = append(b.Citations, citation)
}
func (s *ProviderState) Thinking(id string, text string, signature string) {
	var b *InferenceBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockThinking {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockThinking)
	}
	b.Text += text
	b.Signature += signature
}
func (s *ProviderState) EncryptedThinking(text string) {
	b := s.create("", InferenceBlockEncryptedThinking)
	b.Text += text
}
func (s *ProviderState) ToolCall(id string, toolCallId string, name string, arguments json.RawMessage) {
	var b *InferenceBlock
	for blockIdx := range s.Blocks {
		if s.Blocks[blockIdx].ID == id && s.Blocks[blockIdx].Type == InferenceBlockToolCall {
			b = s.Blocks[blockIdx]
		}
	}
	if b == nil {
		b = s.create(id, InferenceBlockToolCall)
	}
	if b.ToolCall == nil {
		b.ToolCall = &InferenceToolCall{
			ID:        toolCallId,
			Name:      name,
			Arguments: arguments,
		}
	} else {
		b.ToolCall.Arguments = append(b.ToolCall.Arguments, arguments...)
	}
}
func (s *ProviderState) ToolResult(toolCallID string, output json.RawMessage) {
	b := s.create("", InferenceBlockToolResult)
	b.ToolResult = &InferenceToolResult{
		ToolCallID: toolCallID,
		Output:     output,
	}
}

type InferenceProvider interface {
	// Infer runs an inference request and always returns a complete transcript
	// state (blocks).
	//
	// If onPartial is non-nil, the provider may stream partial updates through it.
	Infer(request *InferenceRequest, onPartial func(*ProviderState)) *ProviderState
}
