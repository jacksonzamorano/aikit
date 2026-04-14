package aikit

import (
	"testing"
)

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
		mutate func(thread *Thread)
	}{
		{
			name: "Text",
			mutate: func(thread *Thread) {
				thread.Text("id", "hello")
			},
		},
		{
			name: "Cite",
			mutate: func(thread *Thread) {
				thread.Cite("id", "citation")
			},
		},
		{
			name: "Thinking",
			mutate: func(thread *Thread) {
				thread.Thinking("id", "thinking")
			},
		},
		{
			name: "ThinkingWithSignature",
			mutate: func(thread *Thread) {
				thread.ThinkingWithSignature("id", "thinking", "sig")
			},
		},
		{
			name: "ThinkingSignature",
			mutate: func(thread *Thread) {
				thread.ThinkingSignature("id", "sig")
			},
		},
		{
			name: "ToolCall_new",
			mutate: func(thread *Thread) {
				thread.ToolCall("id", "tool", "{}")
			},
		},
		{
			name: "ToolCall_append_args",
			mutate: func(thread *Thread) {
				thread.ToolCall("id", "tool", "")
				thread.TakeUpdate() // clear
				thread.ToolCall("id", "", `{"arg": 1}`)
			},
		},
		{
			name: "ToolCallWithThinking",
			mutate: func(thread *Thread) {
				thread.ToolCallWithThinking("id", "tool", "{}", "thinking", "sig")
			},
		},
		{
			name: "ToolResult",
			mutate: func(thread *Thread) {
				thread.ToolCall("id", "tool", "{}")
				thread.TakeUpdate() // clear
				thread.ToolResult(thread.Blocks[0].ToolCall, "result")
			},
		},
		{
			name: "WebSearch",
			mutate: func(thread *Thread) {
				thread.WebSearch("id")
			},
		},
		{
			name: "CompleteWebSearch",
			mutate: func(thread *Thread) {
				thread.WebSearch("id")
				thread.TakeUpdate() // clear
				thread.CompleteWebSearch("id")
			},
		},
		{
			name: "ViewWebpageUrl",
			mutate: func(thread *Thread) {
				thread.ViewWebpage("id")
				thread.TakeUpdate() // clear (if any)
				thread.ViewWebpageUrl("id", "https://example.com")
			},
		},
		{
			name: "Complete_with_UpdateOnFinalize",
			mutate: func(thread *Thread) {
				thread.UpdateOnFinalize = true
				thread.Text("id", "content")
				thread.TakeUpdate() // clear
				thread.Complete("id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &Thread{Blocks: []*ThreadBlock{}}
			tt.mutate(thread)
			if !thread.TakeUpdate() {
				t.Errorf("%s should set updated flag", tt.name)
			}
		})
	}
}

// TestUnit_Thread_TakeUpdate verifies TakeUpdate behavior.
func TestUnit_Thread_TakeUpdate(t *testing.T) {
	thread := &Thread{Blocks: []*ThreadBlock{}}

	// Initially false
	if thread.TakeUpdate() {
		t.Error("TakeUpdate should return false on fresh thread")
	}

	// After mutation, returns true
	thread.Text("id", "hello")
	if !thread.TakeUpdate() {
		t.Error("TakeUpdate should return true after mutation")
	}

	// After taking, returns false
	if thread.TakeUpdate() {
		t.Error("TakeUpdate should return false after being taken")
	}

	// Multiple mutations, single take
	thread.Text("id", " world")
	thread.Thinking("id2", "hmm")
	if !thread.TakeUpdate() {
		t.Error("TakeUpdate should return true after multiple mutations")
	}
	if thread.TakeUpdate() {
		t.Error("TakeUpdate should return false after being taken")
	}
}
