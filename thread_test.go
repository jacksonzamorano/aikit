package aikit

import (
	"testing"
)

func newTestStreamState() *StreamState {
	return &StreamState{data: &thread{blocks: []*ThreadBlock{}}}
}

func newTestStreamStateWithCoalesce() *StreamState {
	return &StreamState{data: &thread{blocks: []*ThreadBlock{}, coalesceTextBlocks: true}}
}

func TestUnit_Thread_IncompleteToolCallsCounter(t *testing.T) {
	state := newTestStreamState()

	if state.IncompleteToolCalls() != 0 {
		t.Errorf("Initial IncompleteToolCalls should be 0, got %d", state.IncompleteToolCalls())
	}

	state.AppendToolCall("call_1", "tool_a", "")
	if state.IncompleteToolCalls() != 1 {
		t.Errorf("After 1st ToolCall, expected 1, got %d", state.IncompleteToolCalls())
	}

	state.AppendToolCall("call_2", "tool_b", "")
	if state.IncompleteToolCalls() != 2 {
		t.Errorf("After 2nd ToolCall, expected 2, got %d", state.IncompleteToolCalls())
	}

	// Appending to existing tool call should NOT increment counter
	state.AppendToolCall("call_1", "", `{"more": "args"}`)
	if state.IncompleteToolCalls() != 2 {
		t.Errorf("After appending args, expected 2, got %d", state.IncompleteToolCalls())
	}

	state.AppendToolResult(state.Blocks()[0].ToolCall, "result_1")
	if state.IncompleteToolCalls() != 1 {
		t.Errorf("After 1st ToolResult, expected 1, got %d", state.IncompleteToolCalls())
	}

	state.AppendToolResult(state.Blocks()[1].ToolCall, "result_2")
	if state.IncompleteToolCalls() != 0 {
		t.Errorf("After 2nd ToolResult, expected 0, got %d", state.IncompleteToolCalls())
	}
}

func TestUnit_Thread_CoalesceTextBlocks(t *testing.T) {
	state := newTestStreamStateWithCoalesce()
	state.AppendText("text_1", "First")
	state.AppendText("text_2", " Second")

	if len(state.Blocks()) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(state.Blocks()))
	}
	if !state.Blocks()[0].Continued {
		t.Error("First block should have Continued=true when coalescing")
	}
}

// =============================================================================
// TOOL ARGUMENT ACCUMULATION TESTS
// =============================================================================

func TestUnit_Thread_ToolArgumentAccumulation(t *testing.T) {
	state := newTestStreamState()
	state.AppendToolCall("call_123", "search", "")

	chunks := []string{`{"`, `query":`, ` "hello`, ` world`, `"}`}
	for _, chunk := range chunks {
		state.AppendToolCall("call_123", "", chunk)
	}

	if len(state.Blocks()) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(state.Blocks()))
	}

	block := state.Blocks()[0]
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
	state := newTestStreamState()

	state.AppendToolCall("call_1", "tool_a", "")
	state.AppendToolCall("call_2", "tool_b", "")
	state.AppendToolCall("call_1", "", `{"a": `)
	state.AppendToolCall("call_2", "", `{"b": `)
	state.AppendToolCall("call_1", "", `1}`)
	state.AppendToolCall("call_2", "", `2}`)

	if len(state.Blocks()) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(state.Blocks()))
	}

	var block1, block2 *ThreadBlock
	for _, b := range state.Blocks() {
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

func TestUnit_Thread_CoalesceMultipleTextBlocks(t *testing.T) {
	state := newTestStreamStateWithCoalesce()

	// Create first block
	state.AppendText("text_1", "Hello")
	// Create second block - should coalesce to first since last block is text
	state.AppendText("text_2", " World")
	// Create third block - should coalesce (second block marked as continued)
	state.AppendText("text_3", "!")

	if len(state.Blocks()) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(state.Blocks()))
	}

	// First and second should be marked as continued
	if !state.Blocks()[0].Continued {
		t.Error("First block should have Continued=true")
	}
	if !state.Blocks()[1].Continued {
		t.Error("Second block should have Continued=true")
	}
	// Third should NOT be continued (it's the last one)
	if state.Blocks()[2].Continued {
		t.Error("Third block should NOT have Continued=true")
	}

	// Each block keeps its own text
	if state.Blocks()[0].Text != "Hello" {
		t.Errorf("First block should have 'Hello', got %q", state.Blocks()[0].Text)
	}
	if state.Blocks()[1].Text != " World" {
		t.Errorf("Second block should have ' World', got %q", state.Blocks()[1].Text)
	}
	if state.Blocks()[2].Text != "!" {
		t.Errorf("Third block should have '!', got %q", state.Blocks()[2].Text)
	}
}

func TestUnit_Thread_CoalesceBreaksOnDifferentBlockType(t *testing.T) {
	state := newTestStreamStateWithCoalesce()

	state.AppendText("text_1", "Hello")
	state.AppendToolCall("call_1", "search", "{}")
	state.AppendText("text_2", " World") // Last block is ToolCall, NOT Text - no coalescing

	// Coalescing should NOT happen since last block was a tool call
	// First text block should NOT be marked as continued (no following text block)
	if state.Blocks()[0].Text != "Hello" {
		t.Errorf("First block should have 'Hello', got %q", state.Blocks()[0].Text)
	}
	if state.Blocks()[2].Text != " World" {
		t.Errorf("Third block should have ' World', got %q", state.Blocks()[2].Text)
	}
	// First block should NOT be continued since the following block is a tool call
	if state.Blocks()[0].Continued {
		t.Error("First text block should NOT have Continued=true when followed by tool call")
	}
}

// =============================================================================
// UPDATE FLAG TESTS
// =============================================================================

// TestUnit_Thread_MutationMethodsSetUpdated verifies that all mutation methods
// that should trigger a streaming update properly set the updated flag.
//
// IMPORTANT: When adding new mutation methods that modify thread state during
// streaming, add them to this test to ensure the updated flag is set.
func TestUnit_Thread_MutationMethodsSetUpdated(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(state *StreamState)
	}{
		{
			name: "Text",
			mutate: func(state *StreamState) {
				state.AppendText("id", "hello")
			},
		},
		{
			name: "Cite",
			mutate: func(state *StreamState) {
				state.AppendCite("id", "citation")
			},
		},
		{
			name: "Thinking",
			mutate: func(state *StreamState) {
				state.AppendThinking("id", "thinking")
			},
		},
		{
			name: "ThinkingWithSignature",
			mutate: func(state *StreamState) {
				state.AppendThinkingWithSignature("id", "thinking", "sig")
			},
		},
		{
			name: "ThinkingSignature",
			mutate: func(state *StreamState) {
				state.AppendThinkingSignature("id", "sig")
			},
		},
		{
			name: "ToolCall_new",
			mutate: func(state *StreamState) {
				state.AppendToolCall("id", "tool", "{}")
			},
		},
		{
			name: "ToolCall_append_args",
			mutate: func(state *StreamState) {
				state.AppendToolCall("id", "tool", "")
				state.TakeUpdate() // clear
				state.AppendToolCall("id", "", `{"arg": 1}`)
			},
		},
		{
			name: "ToolCallWithThinking",
			mutate: func(state *StreamState) {
				state.AppendToolCallWithThinking("id", "tool", "{}", "thinking", "sig")
			},
		},
		{
			name: "ToolResult",
			mutate: func(state *StreamState) {
				state.AppendToolCall("id", "tool", "{}")
				state.TakeUpdate() // clear
				state.AppendToolResult(state.Blocks()[0].ToolCall, "result")
			},
		},
		{
			name: "WebSearch",
			mutate: func(state *StreamState) {
				state.AppendWebSearch("id")
			},
		},
		{
			name: "CompleteWebSearch",
			mutate: func(state *StreamState) {
				state.AppendWebSearch("id")
				state.TakeUpdate() // clear
				state.AppendCompleteWebSearch("id")
			},
		},
		{
			name: "ViewWebpageUrl",
			mutate: func(state *StreamState) {
				state.AppendViewWebpage("id")
				state.TakeUpdate() // clear (if any)
				state.AppendViewWebpageUrl("id", "https://example.com")
			},
		},
		{
			name: "Complete_with_UpdateOnFinalize",
			mutate: func(state *StreamState) {
				state.data.updateOnFinalize = true
				state.AppendText("id", "content")
				state.TakeUpdate() // clear
				state.CompleteBlock("id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newTestStreamState()
			tt.mutate(state)
			if !state.TakeUpdate() {
				t.Errorf("%s should set updated flag", tt.name)
			}
		})
	}
}

// TestUnit_Thread_TakeUpdate verifies TakeUpdate behavior.
func TestUnit_Thread_TakeUpdate(t *testing.T) {
	state := newTestStreamState()

	// Initially false
	if state.TakeUpdate() {
		t.Error("TakeUpdate should return false on fresh thread")
	}

	// After mutation, returns true
	state.AppendText("id", "hello")
	if !state.TakeUpdate() {
		t.Error("TakeUpdate should return true after mutation")
	}

	// After taking, returns false
	if state.TakeUpdate() {
		t.Error("TakeUpdate should return false after being taken")
	}

	// Multiple mutations, single take
	state.AppendText("id", " world")
	state.AppendThinking("id2", "hmm")
	if !state.TakeUpdate() {
		t.Error("TakeUpdate should return true after multiple mutations")
	}
	if state.TakeUpdate() {
		t.Error("TakeUpdate should return false after being taken")
	}
}
