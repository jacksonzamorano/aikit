package aikit

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// =============================================================================
// THREAD STATE TESTS
// =============================================================================

func TestUnit_Thread_GetTypeFindsBlockAtIndexZero(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.ToolCall("call_1", "test_tool", `{"arg": "value"}`)

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	found := thread.getType("call_1", InferenceBlockToolCall)
	if found == nil {
		t.Fatal("getType failed to find block at index 0")
	}
	if found.ID != "call_1" {
		t.Errorf("Expected block ID 'call_1', got %q", found.ID)
	}
}

func TestUnit_Thread_ToolResultWithSingleToolCall(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.ToolCall("call_1", "test_tool", `{"query": "test"}`)

	if thread.IncompleteToolCalls() != 1 {
		t.Errorf("Expected 1 incomplete tool call, got %d", thread.IncompleteToolCalls())
	}

	block := thread.Blocks[0]
	thread.ToolResult(block.ToolCall, "result output")

	if block.ToolResult == nil {
		t.Fatal("ToolResult was not attached to the block at index 0")
	}
	if block.ToolResult.Output != "result output" {
		t.Errorf("Expected output 'result output', got %q", block.ToolResult.Output)
	}
	if !block.Complete {
		t.Error("Block should be marked complete after ToolResult")
	}
	if thread.IncompleteToolCalls() != 0 {
		t.Errorf("Expected 0 incomplete tool calls, got %d", thread.IncompleteToolCalls())
	}
}

func TestUnit_Thread_IncompleteToolCallsCounter(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}

	if thread.IncompleteToolCalls() != 0 {
		t.Errorf("Initial IncompleteToolCalls should be 0, got %d", thread.IncompleteToolCalls())
	}

	thread.ToolCall("call_1", "tool_a", "")
	if thread.IncompleteToolCalls() != 1 {
		t.Errorf("After 1st ToolCall, expected 1, got %d", thread.IncompleteToolCalls())
	}

	thread.ToolCall("call_2", "tool_b", "")
	if thread.IncompleteToolCalls() != 2 {
		t.Errorf("After 2nd ToolCall, expected 2, got %d", thread.IncompleteToolCalls())
	}

	// Appending to existing tool call should NOT increment counter
	thread.ToolCall("call_1", "", `{"more": "args"}`)
	if thread.IncompleteToolCalls() != 2 {
		t.Errorf("After appending args, expected 2, got %d", thread.IncompleteToolCalls())
	}

	thread.ToolResult(thread.Blocks[0].ToolCall, "result_1")
	if thread.IncompleteToolCalls() != 1 {
		t.Errorf("After 1st ToolResult, expected 1, got %d", thread.IncompleteToolCalls())
	}

	thread.ToolResult(thread.Blocks[1].ToolCall, "result_2")
	if thread.IncompleteToolCalls() != 0 {
		t.Errorf("After 2nd ToolResult, expected 0, got %d", thread.IncompleteToolCalls())
	}
}

func TestUnit_Thread_FindOrCreateIDBlockAtIndexZero(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.Text("text_1", "Hello")

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	thread.Text("text_1", " World")

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block after append, got %d (duplicate created)", len(thread.Blocks))
	}
	if thread.Blocks[0].Text != "Hello World" {
		t.Errorf("Expected text 'Hello World', got %q", thread.Blocks[0].Text)
	}
}

func TestUnit_Thread_CoalesceTextBlocks(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}, CoalesceTextBlocks: true}
	thread.Text("text_1", "First")
	thread.Text("text_2", " Second")

	if len(thread.Blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(thread.Blocks))
	}
	if !thread.Blocks[0].Continued {
		t.Error("First block should have Continued=true when coalescing")
	}
}

func TestUnit_Thread_NewBlockIdGeneration(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	id1 := thread.NewBlockId(InferenceBlockText)
	thread.Text(id1, "First")
	id2 := thread.NewBlockId(InferenceBlockText)

	if id1 == id2 {
		t.Errorf("NewBlockId should generate unique IDs, got %q twice", id1)
	}
}

func TestUnit_Thread_CompleteBlock(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}, UpdateOnFinalize: true}
	thread.Text("block_1", "content")
	thread.Updated = false

	if thread.Blocks[0].Complete {
		t.Error("Block should not be complete initially")
	}

	thread.Complete("block_1")

	if !thread.Blocks[0].Complete {
		t.Error("Block should be complete after Complete()")
	}
	if !thread.Updated {
		t.Error("Updated flag should be set when UpdateOnFinalize is true")
	}
}

func TestUnit_Thread_WebSearchFlow(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.WebSearch("search_1")

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	block := thread.Blocks[0]
	if block.WebSearch == nil {
		t.Fatal("WebSearch should be initialized")
	}

	thread.WebSearchQuery("search_1", "test query")

	if block.WebSearch.Query != "test query" {
		t.Errorf("Expected query 'test query', got %q", block.WebSearch.Query)
	}
	if !block.Complete {
		t.Error("Block should be complete after WebSearchQuery")
	}
	if thread.Result.WebSearches != 1 {
		t.Errorf("Expected 1 web search in results, got %d", thread.Result.WebSearches)
	}
}

func TestUnit_Thread_ThinkingWithSignature(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.ThinkingWithSignature("think_1", "thinking content", "sig123")

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	block := thread.Blocks[0]
	if block.Text != "thinking content" {
		t.Errorf("Expected text 'thinking content', got %q", block.Text)
	}
	if block.Signature != "sig123" {
		t.Errorf("Expected signature 'sig123', got %q", block.Signature)
	}

	thread.ThinkingWithSignature("think_1", " more", "456")

	if block.Text != "thinking content more" {
		t.Errorf("Expected appended text, got %q", block.Text)
	}
	if block.Signature != "sig123456" {
		t.Errorf("Expected appended signature, got %q", block.Signature)
	}
}

// =============================================================================
// TOOL ARGUMENT ACCUMULATION TESTS
// =============================================================================

func TestUnit_Thread_ToolArgumentAccumulation(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}
	thread.ToolCall("call_123", "search", "")

	chunks := []string{`{"`, `query":`, ` "hello`, ` world`, `"}`}
	for _, chunk := range chunks {
		thread.ToolCall("call_123", "", chunk)
	}

	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	block := thread.Blocks[0]
	if block.ToolCall == nil {
		t.Fatal("ToolCall is nil")
	}

	expectedArgs := `{"query": "hello world"}`
	if block.ToolCall.Arguments != expectedArgs {
		t.Errorf("Expected arguments %q, got %q", expectedArgs, block.ToolCall.Arguments)
	}
	if block.ToolCall.Name != "search" {
		t.Errorf("Expected tool name 'search', got %q", block.ToolCall.Name)
	}
}

func TestUnit_Thread_MultipleToolCallsAccumulation(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}

	thread.ToolCall("call_1", "tool_a", "")
	thread.ToolCall("call_2", "tool_b", "")
	thread.ToolCall("call_1", "", `{"a": `)
	thread.ToolCall("call_2", "", `{"b": `)
	thread.ToolCall("call_1", "", `1}`)
	thread.ToolCall("call_2", "", `2}`)

	if len(thread.Blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(thread.Blocks))
	}

	var block1, block2 *ThreadBlock
	for _, b := range thread.Blocks {
		if b.ID == "call_1" {
			block1 = b
		} else if b.ID == "call_2" {
			block2 = b
		}
	}

	if block1 == nil || block1.ToolCall == nil {
		t.Fatal("Tool call 1 not found or has nil ToolCall")
	}
	if block2 == nil || block2.ToolCall == nil {
		t.Fatal("Tool call 2 not found or has nil ToolCall")
	}
	if block1.ToolCall.Arguments != `{"a": 1}` {
		t.Errorf("Tool 1: expected arguments %q, got %q", `{"a": 1}`, block1.ToolCall.Arguments)
	}
	if block2.ToolCall.Arguments != `{"b": 2}` {
		t.Errorf("Tool 2: expected arguments %q, got %q", `{"b": 2}`, block2.ToolCall.Arguments)
	}
}

// =============================================================================
// SSE PARSING TESTS
// =============================================================================

func TestUnit_SSE_BasicEvent(t *testing.T) {
	input := "event: message\ndata: hello world\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "message" {
		t.Errorf("Expected event 'message', got %q", received[0].event)
	}
	if string(received[0].data) != "hello world" {
		t.Errorf("Expected data 'hello world', got %q", string(received[0].data))
	}
}

func TestUnit_SSE_MultipleEvents(t *testing.T) {
	input := "event: first\ndata: one\n\nevent: second\ndata: two\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(received))
	}
	if received[0].event != "first" {
		t.Errorf("First event: expected 'first', got %q", received[0].event)
	}
	if received[1].event != "second" {
		t.Errorf("Second event: expected 'second', got %q", received[1].event)
	}
}

func TestUnit_SSE_MultilineData(t *testing.T) {
	input := "event: test\ndata: line one\ndata: line two\ndata: line three\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}

	expected := "line one\nline two\nline three"
	if string(received[0].data) != expected {
		t.Errorf("Expected data %q, got %q", expected, string(received[0].data))
	}
}

func TestUnit_SSE_CommentLines(t *testing.T) {
	input := ": this is a comment\nevent: test\n: another comment\ndata: content\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "test" {
		t.Errorf("Expected event 'test', got %q", received[0].event)
	}
	if string(received[0].data) != "content" {
		t.Errorf("Expected data 'content', got %q", string(received[0].data))
	}
}

func TestUnit_SSE_CRLFLineEndings(t *testing.T) {
	input := "event: test\r\ndata: content\r\n\r\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "test" {
		t.Errorf("Expected event 'test', got %q", received[0].event)
	}
}

func TestUnit_SSE_DataWithoutEvent(t *testing.T) {
	input := "data: just data\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "" {
		t.Errorf("Expected empty event type, got %q", received[0].event)
	}
	if string(received[0].data) != "just data" {
		t.Errorf("Expected data 'just data', got %q", string(received[0].data))
	}
}

func TestUnit_SSE_EventWithoutData(t *testing.T) {
	input := "event: ping\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "ping" {
		t.Errorf("Expected event 'ping', got %q", received[0].event)
	}
	if len(received[0].data) != 0 {
		t.Errorf("Expected empty data, got %q", string(received[0].data))
	}
}

func TestUnit_SSE_HandlerStop(t *testing.T) {
	input := "event: first\ndata: one\n\nevent: second\ndata: two\n\n"
	count := 0

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		count++
		return false, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected handler called once, got %d", count)
	}
}

func TestUnit_SSE_HandlerError(t *testing.T) {
	input := "event: test\ndata: content\n\n"
	handlerErr := errors.New("handler error")

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		return false, handlerErr
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	aiErr, ok := err.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", err)
	}
	if aiErr.Category != AIErrorCategoryStreamingError {
		t.Errorf("Expected category StreamingError, got %v", aiErr.Category)
	}
}

func TestUnit_SSE_HandlerAIErrorPreserved(t *testing.T) {
	input := "event: test\ndata: content\n\n"
	handlerErr := RateLimitError("test", "rate limited")

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		return false, handlerErr
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	aiErr, ok := err.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", err)
	}
	if aiErr.Category != AIErrorCategoryRateLimit {
		t.Errorf("Expected category RateLimit (preserved), got %v", aiErr.Category)
	}
}

func TestUnit_SSE_EOFMidEvent(t *testing.T) {
	input := "event: test\ndata: content"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event (flushed at EOF), got %d", len(received))
	}
	if received[0].event != "test" {
		t.Errorf("Expected event 'test', got %q", received[0].event)
	}
}

func TestUnit_SSE_EmptyInput(t *testing.T) {
	var received []sseEvent

	err := readSSE("test", strings.NewReader(""), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 0 {
		t.Errorf("Expected 0 events for empty input, got %d", len(received))
	}
}

func TestUnit_SSE_OnlyComments(t *testing.T) {
	input := ": comment 1\n: comment 2\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 0 {
		t.Errorf("Expected 0 events for comment-only input, got %d", len(received))
	}
}

func TestUnit_SSE_KeepalivePattern(t *testing.T) {
	input := ":\n\nevent: real\ndata: data\n\n"
	var received []sseEvent

	err := readSSE("test", strings.NewReader(input), func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
	if received[0].event != "real" {
		t.Errorf("Expected event 'real', got %q", received[0].event)
	}
}

type errorReader struct{ err error }

func (e *errorReader) Read(p []byte) (n int, err error) { return 0, e.err }

func TestUnit_SSE_ReaderError(t *testing.T) {
	readerErr := errors.New("read error")
	reader := &errorReader{err: readerErr}

	err := readSSE("test", reader, func(ev sseEvent) (bool, error) {
		return true, nil
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	aiErr, ok := err.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", err)
	}
	if aiErr.Category != AIErrorCategoryStreamingError {
		t.Errorf("Expected StreamingError category, got %v", aiErr.Category)
	}
}

type partialReader struct {
	data string
	pos  int
	err  error
}

func (p *partialReader) Read(b []byte) (n int, err error) {
	if p.pos >= len(p.data) {
		return 0, p.err
	}
	n = copy(b, p.data[p.pos:])
	p.pos += n
	if p.pos >= len(p.data) {
		return n, p.err
	}
	return n, nil
}

func TestUnit_SSE_ReaderErrorAfterData(t *testing.T) {
	reader := &partialReader{data: "event: test\ndata: content\n\n", err: io.EOF}
	var received []sseEvent

	err := readSSE("test", reader, func(ev sseEvent) (bool, error) {
		received = append(received, ev)
		return true, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(received))
	}
}

// =============================================================================
// CONFIG TESTS
// =============================================================================

func TestUnit_Config_ResolveEndpointWithExplicitEndpoint(t *testing.T) {
	config := ProviderConfig{
		BaseURL:  "https://api.example.com/v1",
		Endpoint: "https://custom.endpoint.com/v2/messages",
	}

	result := config.resolveEndpoint("/v1/messages")

	if result != "https://custom.endpoint.com/v2/messages" {
		t.Errorf("Expected explicit endpoint, got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointWithBaseURL(t *testing.T) {
	config := ProviderConfig{BaseURL: "https://api.example.com/v1"}
	result := config.resolveEndpoint("/v1/messages")

	if result != "https://api.example.com/v1/v1/messages" {
		t.Errorf("Expected %q, got %q", "https://api.example.com/v1/v1/messages", result)
	}
}

func TestUnit_Config_ResolveEndpointTrimsWhitespace(t *testing.T) {
	config := ProviderConfig{Endpoint: "  https://api.example.com/messages  "}
	result := config.resolveEndpoint("/default")

	if result != "https://api.example.com/messages" {
		t.Errorf("Expected trimmed endpoint, got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointTrailingSlash(t *testing.T) {
	config := ProviderConfig{BaseURL: "https://api.example.com/v1/"}
	result := config.resolveEndpoint("/messages")

	if result != "https://api.example.com/v1/messages" {
		t.Errorf("Expected %q, got %q", "https://api.example.com/v1/messages", result)
	}
}

func TestUnit_Config_ResolveEndpointEmptyBaseURL(t *testing.T) {
	config := ProviderConfig{BaseURL: "", Endpoint: ""}
	result := config.resolveEndpoint("/messages")

	if result != "messages" {
		t.Errorf("Expected 'messages', got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointMalformedPanics(t *testing.T) {
	config := ProviderConfig{Endpoint: "://invalid-url"}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for malformed Endpoint URL")
		}
	}()

	config.resolveEndpoint("/messages")
}

func TestUnit_Config_SessionCallsFactory(t *testing.T) {
	called := false
	config := &ProviderConfig{
		Name: "test",
		MakeSessionFunction: func(c *ProviderConfig) *Session {
			called = true
			if c.Name != "test" {
				t.Errorf("Expected config name 'test', got %q", c.Name)
			}
			return &Session{Thread: NewProviderState()}
		},
	}

	session := config.Session()

	if !called {
		t.Error("MakeSessionFunction was not called")
	}
	if session == nil {
		t.Error("Session should not be nil")
	}
}

// =============================================================================
// ERROR CONSTRUCTOR TESTS
// =============================================================================

func TestUnit_Error_ConstructorCategories(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, string) *AIError
		expected AIErrorCategory
	}{
		{"DecodingError", DecodingError, AIErrorCategoryDecodingError},
		{"AuthenticationError", AuthenticationError, AIErrorCategoryAuthentication},
		{"RateLimitError", RateLimitError, AIErrorCategoryRateLimit},
		{"UnknownError", UnknownError, AIErrorCategoryUnknown},
		{"ConfigurationError", ConfigurationError, AIErrorCategoryConfiguration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("provider", "message")
			if err.Category != tt.expected {
				t.Errorf("got %v, want %v", err.Category, tt.expected)
			}
			if err.Provider != "provider" {
				t.Errorf("got provider %q, want 'provider'", err.Provider)
			}
		})
	}
}

// =============================================================================
// HTTP ERROR PARSING TESTS - MESSAGES API
// =============================================================================

func TestUnit_Messages_ParseHttpError401(t *testing.T) {
	config := AnthropicProvider("test-key")
	req := &MessagesAPIRequest{Config: &config}

	body := []byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`)
	err := req.ParseHttpError(401, body)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err.Category != AIErrorCategoryAuthentication {
		t.Errorf("Expected Authentication category, got %v", err.Category)
	}
	if err.Message != "Invalid API key" {
		t.Errorf("Expected message 'Invalid API key', got %q", err.Message)
	}
}

func TestUnit_Messages_ParseHttpError429(t *testing.T) {
	config := AnthropicProvider("test-key")
	req := &MessagesAPIRequest{Config: &config}

	body := []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`)
	err := req.ParseHttpError(429, body)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err.Category != AIErrorCategoryRateLimit {
		t.Errorf("Expected RateLimit category, got %v", err.Category)
	}
}

func TestUnit_Messages_ParseHttpErrorMalformedJSON(t *testing.T) {
	config := AnthropicProvider("test-key")
	req := &MessagesAPIRequest{Config: &config}

	err := req.ParseHttpError(500, []byte(`not json at all`))

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err.Category != AIErrorCategoryUnknown {
		t.Errorf("Expected Unknown category for malformed JSON, got %v", err.Category)
	}
}

// =============================================================================
// HTTP ERROR PARSING TESTS - COMPLETIONS API
// =============================================================================

func TestUnit_Completions_ParseHttpErrorByType(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		wantCategory AIErrorCategory
	}{
		{"invalid_request_error", "invalid_request_error", AIErrorCategoryConfiguration},
		{"authentication_error", "authentication_error", AIErrorCategoryAuthentication},
		{"rate_limit_error", "rate_limit_error", AIErrorCategoryRateLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GroqProvider("test-key")
			req := &CompletionsAPIRequest{Config: &config}

			body := []byte(`{"error":{"type":"` + tt.errorType + `","message":"Test message"}}`)
			err := req.ParseHttpError(400, body)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if err.Category != tt.wantCategory {
				t.Errorf("Expected %v category, got %v", tt.wantCategory, err.Category)
			}
		})
	}
}

func TestUnit_Completions_ParseHttpErrorByStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		wantCategory AIErrorCategory
	}{
		{"401", 401, AIErrorCategoryAuthentication},
		{"403", 403, AIErrorCategoryAuthentication},
		{"404", 404, AIErrorCategoryConfiguration},
		{"429", 429, AIErrorCategoryRateLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GroqProvider("test-key")
			req := &CompletionsAPIRequest{Config: &config}

			body := []byte(`{"error":{"message":"Error message"}}`)
			err := req.ParseHttpError(tt.statusCode, body)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if err.Category != tt.wantCategory {
				t.Errorf("Expected %v category, got %v", tt.wantCategory, err.Category)
			}
		})
	}
}

// =============================================================================
// HTTP ERROR PARSING TESTS - RESPONSES API
// =============================================================================

func TestUnit_Responses_ParseHttpErrorReturnsNil(t *testing.T) {
	config := OpenAIProvider("test-key")
	req := &ResponsesAPIRequest{Config: &config}

	err := req.ParseHttpError(401, []byte(`{"error":"test"}`))

	if err != nil {
		t.Errorf("ResponsesAPIRequest.ParseHttpError should return nil, got %v", err)
	}
}

// =============================================================================
// ONCHUNK ERROR HANDLING TESTS
// =============================================================================

func TestUnit_Messages_OnChunkMalformedJSON(t *testing.T) {
	config := AnthropicProvider("test-key")
	session := config.Session()
	session.Thread.Model = "claude-3"

	req := session.Provider.(*MessagesAPIRequest)
	req.InitSession(session.Thread)

	result := req.OnChunk([]byte(`{not valid json`), session.Thread)

	if result.Error == nil {
		t.Fatal("Expected error for malformed JSON")
	}
	aiErr, ok := result.Error.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", result.Error)
	}
	if aiErr.Category != AIErrorCategoryDecodingError {
		t.Errorf("Expected DecodingError category, got %v", aiErr.Category)
	}
}

func TestUnit_Messages_OnChunkUnknownEventType(t *testing.T) {
	config := AnthropicProvider("test-key")
	session := config.Session()
	session.Thread.Model = "claude-3"

	req := session.Provider.(*MessagesAPIRequest)
	req.InitSession(session.Thread)

	result := req.OnChunk([]byte(`{"type":"unknown_event_type"}`), session.Thread)

	if result.Error != nil {
		t.Errorf("Unknown event type should not cause error, got %v", result.Error)
	}
}

func TestUnit_Messages_OnChunkErrorEvent(t *testing.T) {
	config := AnthropicProvider("test-key")
	session := config.Session()
	session.Thread.Model = "claude-3"

	req := session.Provider.(*MessagesAPIRequest)
	req.InitSession(session.Thread)

	chunk := []byte(`{"type":"error","error":{"type":"rate_limit_exceeded","message":"Too many requests"}}`)
	result := req.OnChunk(chunk, session.Thread)

	if result.Error == nil {
		t.Fatal("Expected error for error event")
	}
	aiErr, ok := result.Error.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", result.Error)
	}
	if aiErr.Category != AIErrorCategoryRateLimit {
		t.Errorf("Expected RateLimit category, got %v", aiErr.Category)
	}
}

func TestUnit_Messages_OnChunkToolCallStreaming(t *testing.T) {
	config := AnthropicProvider("test-key")
	session := config.Session()
	session.Thread.Model = "claude-3-opus"
	session.Thread.Tools = map[string]ToolDefinition{
		"search": {
			Description: "Search for information",
			Parameters:  &ToolJsonSchema{Type: "object"},
		},
	}

	req := session.Provider.(*MessagesAPIRequest)
	req.InitSession(session.Thread)

	startChunk := []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_abc","name":"search","input":{}}}`)
	result := req.OnChunk(startChunk, session.Thread)
	if result.Error != nil {
		t.Fatalf("OnChunk failed on content_block_start: %v", result.Error)
	}

	deltaChunks := []string{
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":" \"test"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":" query\"}"}}`,
	}

	for _, chunk := range deltaChunks {
		result = req.OnChunk([]byte(chunk), session.Thread)
		if result.Error != nil {
			t.Fatalf("OnChunk failed on content_block_delta: %v", result.Error)
		}
	}

	var toolBlock *ThreadBlock
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockToolCall {
			toolBlock = b
			break
		}
	}

	if toolBlock == nil {
		t.Fatal("No tool call block found")
	}
	if toolBlock.ToolCall == nil {
		t.Fatal("ToolCall is nil")
	}
	if toolBlock.ToolCall.Arguments != `{"query": "test query"}` {
		t.Errorf("Expected arguments %q, got %q", `{"query": "test query"}`, toolBlock.ToolCall.Arguments)
	}
}

func TestUnit_Completions_OnChunkMalformedJSON(t *testing.T) {
	config := GroqProvider("test-key")
	session := config.Session()
	session.Thread.Model = "llama-3"

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	result := req.OnChunk([]byte(`{not valid json`), session.Thread)

	if result.Error == nil {
		t.Fatal("Expected error for malformed JSON")
	}
	aiErr, ok := result.Error.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", result.Error)
	}
	if aiErr.Category != AIErrorCategoryDecodingError {
		t.Errorf("Expected DecodingError category, got %v", aiErr.Category)
	}
}

func TestUnit_Completions_OnChunkToolCallStreaming(t *testing.T) {
	config := GroqProvider("test-key")
	session := config.Session()
	session.Thread.Model = "llama-3"
	session.Thread.Tools = map[string]ToolDefinition{
		"get_weather": {
			Description: "Get weather information",
			Parameters:  &ToolJsonSchema{Type: "object"},
		},
	}

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	chunks := []string{
		`{"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_xyz","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
		`{"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":"}}]}}]}`,
		`{"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":" \"Paris\"}"}}]}}]}`,
	}

	for _, chunk := range chunks {
		result := req.OnChunk([]byte(chunk), session.Thread)
		if result.Error != nil {
			t.Fatalf("OnChunk failed: %v", result.Error)
		}
	}

	var toolBlock *ThreadBlock
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockToolCall {
			toolBlock = b
			break
		}
	}

	if toolBlock == nil {
		t.Fatal("No tool call block found")
	}
	if toolBlock.ToolCall == nil {
		t.Fatal("ToolCall is nil")
	}
	if toolBlock.ToolCall.Arguments != `{"location": "Paris"}` {
		t.Errorf("Expected arguments %q, got %q", `{"location": "Paris"}`, toolBlock.ToolCall.Arguments)
	}
}

func TestUnit_Responses_OnChunkMalformedJSON(t *testing.T) {
	config := OpenAIProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gpt-4"

	req := session.Provider.(*ResponsesAPIRequest)
	req.InitSession(session.Thread)

	result := req.OnChunk([]byte(`{not valid json`), session.Thread)

	if result.Error == nil {
		t.Fatal("Expected error for malformed JSON")
	}
	aiErr, ok := result.Error.(*AIError)
	if !ok {
		t.Fatalf("Expected *AIError, got %T", result.Error)
	}
	if aiErr.Category != AIErrorCategoryDecodingError {
		t.Errorf("Expected DecodingError category, got %v", aiErr.Category)
	}
}

func TestUnit_Responses_OnChunkErrorEvent(t *testing.T) {
	config := OpenAIProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gpt-4"

	req := session.Provider.(*ResponsesAPIRequest)
	req.InitSession(session.Thread)

	chunk := []byte(`{"type":"error","error":{"message":"Something went wrong"}}`)
	result := req.OnChunk(chunk, session.Thread)

	if result.Error == nil {
		t.Fatal("Expected error for error event")
	}
}

// =============================================================================
// REQUEST SERIALIZATION TESTS
// =============================================================================

func TestUnit_Messages_RequestSerialization(t *testing.T) {
	config := AnthropicProvider("test-key")
	session := config.Session()
	session.Thread.Model = "claude-3-opus"
	session.Thread.Tools = map[string]ToolDefinition{
		"search": {
			Description: "Search for information",
			Parameters: &ToolJsonSchema{
				Type: "object",
				Properties: &map[string]*ToolJsonSchema{
					"query": {Type: "string"},
				},
			},
		},
	}

	req := session.Provider.(*MessagesAPIRequest)
	req.InitSession(session.Thread)

	if len(req.request.Tools) == 0 {
		t.Fatal("Tools not initialized")
	}
	if req.request.Model != "claude-3-opus" {
		t.Errorf("Expected model 'claude-3-opus', got %q", req.request.Model)
	}
}

func TestUnit_Completions_RequestSerialization(t *testing.T) {
	config := GroqProvider("test-key")
	session := config.Session()
	session.Thread.Model = "llama-3"
	session.Thread.Tools = map[string]ToolDefinition{
		"search": {
			Description: "Search for information",
			Parameters: &ToolJsonSchema{
				Type: "object",
				Properties: &map[string]*ToolJsonSchema{
					"query": {Type: "string"},
				},
			},
		},
	}

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	if len(req.request.Tools) == 0 {
		t.Fatal("Tools not initialized")
	}
	if req.request.Model != "llama-3" {
		t.Errorf("Expected model 'llama-3', got %q", req.request.Model)
	}
}

func TestUnit_Responses_RequestSerialization(t *testing.T) {
	config := OpenAIProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gpt-4"
	session.Thread.Tools = map[string]ToolDefinition{
		"search": {
			Description: "Search for information",
			Parameters: &ToolJsonSchema{
				Type: "object",
				Properties: &map[string]*ToolJsonSchema{
					"query": {Type: "string"},
				},
			},
		},
	}

	req := session.Provider.(*ResponsesAPIRequest)
	req.InitSession(session.Thread)

	if len(req.Request.Tools) == 0 {
		t.Fatal("Tools not initialized")
	}
	if req.Request.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %q", req.Request.Model)
	}
}

// =============================================================================
// TOOL DEFINITION SERIALIZATION TESTS
// =============================================================================

func TestUnit_Tool_DefinitionSerialization(t *testing.T) {
	toolDef := ToolDefinition{
		Description: "Search for information on a topic",
		Parameters: &ToolJsonSchema{
			Type: "object",
			Properties: &map[string]*ToolJsonSchema{
				"query": {Type: "string", Description: "The search query"},
				"limit": {Type: "integer", Description: "Maximum number of results"},
			},
			Required: []string{"query"},
		},
	}

	if toolDef.Description != "Search for information on a topic" {
		t.Errorf("Unexpected description: %q", toolDef.Description)
	}
	if toolDef.Parameters.Type != "object" {
		t.Errorf("Expected parameters type 'object', got %q", toolDef.Parameters.Type)
	}
	if len(*toolDef.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(*toolDef.Parameters.Properties))
	}
}

// =============================================================================
// AISTUDIO ONCHUNK TESTS
// =============================================================================

func TestUnit_AIStudio_EmptyCandidates(t *testing.T) {
	config := GoogleProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gemini-pro"

	req := session.Provider.(*AIStudioAPIRequest)
	req.InitSession(session.Thread)

	// Empty candidates array should not panic - just accept and continue
	chunk := []byte(`{"candidates":[],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OnChunk panicked with empty candidates: %v", r)
		}
	}()

	result := req.OnChunk(chunk, session.Thread)

	// Should handle gracefully - accept the chunk
	if result.Error != nil {
		t.Errorf("Unexpected error for empty candidates: %v", result.Error)
	}

	// Token counts should still be updated
	if session.Thread.Result.InputTokens != 10 {
		t.Errorf("Expected 10 input tokens, got %d", session.Thread.Result.InputTokens)
	}
}

func TestUnit_AIStudio_MultiplePartsInCandidate(t *testing.T) {
	config := GoogleProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gemini-pro"
	session.Thread.Tools = map[string]ToolDefinition{
		"get_weather": {Description: "Get weather"},
	}

	req := session.Provider.(*AIStudioAPIRequest)
	req.InitSession(session.Thread)

	// A chunk with multiple parts - text and function call
	chunk := []byte(`{
		"responseId": "resp_123",
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Let me check the weather."},
					{"functionCall": {"name": "get_weather", "args": {"location": "Paris"}}}
				]
			}
		}],
		"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 20}
	}`)

	result := req.OnChunk(chunk, session.Thread)

	if result.Error != nil {
		t.Fatalf("OnChunk failed: %v", result.Error)
	}

	// Should have created both text and tool call blocks
	var hasText, hasToolCall bool
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockText && b.Text != "" {
			hasText = true
		}
		if b.Type == InferenceBlockToolCall && b.ToolCall != nil {
			hasToolCall = true
		}
	}

	if !hasText {
		t.Error("Expected text block from first part")
	}
	if !hasToolCall {
		t.Error("Expected tool call block from second part")
	}
}

func TestUnit_AIStudio_ThinkingPart(t *testing.T) {
	config := GoogleProvider("test-key")
	session := config.Session()
	session.Thread.Model = "gemini-pro"

	req := session.Provider.(*AIStudioAPIRequest)
	req.InitSession(session.Thread)

	chunk := []byte(`{
		"responseId": "resp_123",
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Internal reasoning...", "thought": true, "thoughtSignature": "sig_abc"}
				]
			}
		}],
		"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 20}
	}`)

	result := req.OnChunk(chunk, session.Thread)

	if result.Error != nil {
		t.Fatalf("OnChunk failed: %v", result.Error)
	}

	var thinkingBlock *ThreadBlock
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockThinking {
			thinkingBlock = b
			break
		}
	}

	if thinkingBlock == nil {
		t.Fatal("Expected thinking block")
	}
	if thinkingBlock.Signature != "sig_abc" {
		t.Errorf("Expected signature 'sig_abc', got %q", thinkingBlock.Signature)
	}
}

// =============================================================================
// COMPLETIONS MULTI-TOOL TESTS
// =============================================================================

func TestUnit_Completions_MultipleToolCallsInSameDelta(t *testing.T) {
	config := GroqProvider("test-key")
	session := config.Session()
	session.Thread.Model = "llama-3"
	session.Thread.Tools = map[string]ToolDefinition{
		"tool_a": {Description: "Tool A"},
		"tool_b": {Description: "Tool B"},
	}

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	// Single delta with two tool calls (both have IDs)
	chunk := []byte(`{
		"id": "chatcmpl-123",
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [
					{"index": 0, "id": "call_aaa", "type": "function", "function": {"name": "tool_a", "arguments": "{\"x\":1}"}},
					{"index": 1, "id": "call_bbb", "type": "function", "function": {"name": "tool_b", "arguments": "{\"y\":2}"}}
				]
			}
		}]
	}`)

	result := req.OnChunk(chunk, session.Thread)
	if result.Error != nil {
		t.Fatalf("OnChunk failed: %v", result.Error)
	}

	// Should have created two distinct tool call blocks
	var blockA, blockB *ThreadBlock
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockToolCall && b.ToolCall != nil {
			if b.ToolCall.Name == "tool_a" {
				blockA = b
			} else if b.ToolCall.Name == "tool_b" {
				blockB = b
			}
		}
	}

	if blockA == nil {
		t.Error("Expected block for tool_a")
	} else if blockA.ID != "call_aaa" {
		t.Errorf("Tool A: expected ID 'call_aaa', got %q", blockA.ID)
	}

	if blockB == nil {
		t.Error("Expected block for tool_b")
	} else if blockB.ID != "call_bbb" {
		t.Errorf("Tool B: expected ID 'call_bbb', got %q", blockB.ID)
	}
}

func TestUnit_Completions_ToolCallWithoutId(t *testing.T) {
	config := GroqProvider("test-key")
	session := config.Session()
	session.Thread.Model = "llama-3"
	session.Thread.Tools = map[string]ToolDefinition{
		"search": {Description: "Search"},
	}

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	// First chunk with ID
	chunk1 := []byte(`{
		"id": "chatcmpl-123",
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [{"index": 0, "id": "call_xyz", "type": "function", "function": {"name": "search", "arguments": ""}}]
			}
		}]
	}`)
	req.OnChunk(chunk1, session.Thread)

	// Second chunk without ID (continuation)
	chunk2 := []byte(`{
		"id": "chatcmpl-123",
		"choices": [{
			"index": 0,
			"delta": {
				"tool_calls": [{"index": 0, "function": {"arguments": "{\"q\": \"test\"}"}}]
			}
		}]
	}`)
	req.OnChunk(chunk2, session.Thread)

	// Should have appended to the same tool call
	var toolBlock *ThreadBlock
	for _, b := range session.Thread.Blocks {
		if b.Type == InferenceBlockToolCall {
			toolBlock = b
			break
		}
	}

	if toolBlock == nil {
		t.Fatal("Expected tool call block")
	}
	if toolBlock.ToolCall.Arguments != `{"q": "test"}` {
		t.Errorf("Expected arguments %q, got %q", `{"q": "test"}`, toolBlock.ToolCall.Arguments)
	}
}

// =============================================================================
// THREAD ALIAS EDGE CASE TESTS
// =============================================================================

func TestUnit_Thread_CoalesceMultipleTextBlocks(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}, CoalesceTextBlocks: true}

	// Create first block
	thread.Text("text_1", "Hello")
	// Create second block - should coalesce to first since last block is text
	thread.Text("text_2", " World")
	// Create third block - should coalesce (second block marked as continued)
	thread.Text("text_3", "!")

	if len(thread.Blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(thread.Blocks))
	}

	// First and second should be marked as continued
	if !thread.Blocks[0].Continued {
		t.Error("First block should have Continued=true")
	}
	if !thread.Blocks[1].Continued {
		t.Error("Second block should have Continued=true")
	}
	// Third should NOT be continued (it's the last one)
	if thread.Blocks[2].Continued {
		t.Error("Third block should NOT have Continued=true")
	}

	// Each block keeps its own text
	if thread.Blocks[0].Text != "Hello" {
		t.Errorf("First block should have 'Hello', got %q", thread.Blocks[0].Text)
	}
	if thread.Blocks[1].Text != " World" {
		t.Errorf("Second block should have ' World', got %q", thread.Blocks[1].Text)
	}
	if thread.Blocks[2].Text != "!" {
		t.Errorf("Third block should have '!', got %q", thread.Blocks[2].Text)
	}
}

func TestUnit_Thread_CoalesceBreaksOnDifferentBlockType(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}, CoalesceTextBlocks: true}

	thread.Text("text_1", "Hello")
	thread.ToolCall("call_1", "search", "{}")
	thread.Text("text_2", " World") // Last block is ToolCall, NOT Text - no coalescing

	// Coalescing should NOT happen since last block was a tool call
	// First text block should NOT be marked as continued (no following text block)
	if thread.Blocks[0].Text != "Hello" {
		t.Errorf("First block should have 'Hello', got %q", thread.Blocks[0].Text)
	}
	if thread.Blocks[2].Text != " World" {
		t.Errorf("Third block should have ' World', got %q", thread.Blocks[2].Text)
	}
	// First block should NOT be continued since the following block is a tool call
	if thread.Blocks[0].Continued {
		t.Error("First text block should NOT have Continued=true when followed by tool call")
	}
}
