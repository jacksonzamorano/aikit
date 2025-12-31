package aikit

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

func TestSnapshot_FullRoundTrip(t *testing.T) {
	thread := &Thread{
		Model:    "claude-3-opus",
		ThreadId: "thread_123",
		Blocks:   []*ThreadBlock{},
	}
	thread.System("You are a helpful assistant.")
	thread.Input("Hello")
	thread.Text("text_1", "Hi there!")
	thread.Complete("text_1")

	// Create snapshot and serialize
	snapshot := thread.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Deserialize
	var restoredSnapshot Snapshot
	if err := json.Unmarshal(data, &restoredSnapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Restore to a new thread
	newThread := &Thread{
		Model:    "claude-3-opus", // Configuration must be re-applied
		ThreadId: "thread_123",
		Blocks:   []*ThreadBlock{},
	}
	newThread.Restore(&restoredSnapshot)

	// Verify blocks were restored
	if len(newThread.Blocks) != len(thread.Blocks) {
		t.Errorf("Block count mismatch: got %d, want %d", len(newThread.Blocks), len(thread.Blocks))
	}

	// Verify block content
	for i, b := range newThread.Blocks {
		if b.Type != thread.Blocks[i].Type {
			t.Errorf("Block %d type mismatch: got %q, want %q", i, b.Type, thread.Blocks[i].Type)
		}
		if b.Text != thread.Blocks[i].Text {
			t.Errorf("Block %d text mismatch: got %q, want %q", i, b.Text, thread.Blocks[i].Text)
		}
	}
}

func TestThread_XMLRoundTrip(t *testing.T) {
	// Note: XML serialization does not fully support Thread because
	// Go's xml package cannot marshal map[string]ToolDefinition.
	// This test verifies that ThreadBlock XML serialization works.
	block := &ThreadBlock{
		ID:       "block_1",
		Type:     InferenceBlockText,
		Text:     "Hello, World!",
		Complete: true,
	}

	// Serialize block
	data, err := xml.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal XML: %v", err)
	}

	// Deserialize
	var restored ThreadBlock
	if err := xml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal XML: %v", err)
	}

	// Verify
	if restored.ID != block.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, block.ID)
	}
	if restored.Type != block.Type {
		t.Errorf("Type mismatch: got %q, want %q", restored.Type, block.Type)
	}
	if restored.Text != block.Text {
		t.Errorf("Text mismatch: got %q, want %q", restored.Text, block.Text)
	}
	if restored.Complete != block.Complete {
		t.Errorf("Complete mismatch: got %v, want %v", restored.Complete, block.Complete)
	}
}

func TestSnapshot_ContinuedBlocks(t *testing.T) {
	thread := &Thread{
		CoalesceTextBlocks: true,
		Blocks:             []*ThreadBlock{},
	}

	// Add first text block
	thread.Text("text_1", "Hello")
	thread.Complete("text_1")

	// Add second text block with different ID (should coalesce)
	thread.Text("text_2", " World")

	// Verify Continued is set on first block
	if len(thread.Blocks) < 2 {
		t.Fatalf("Expected at least 2 blocks, got %d", len(thread.Blocks))
	}
	if !thread.Blocks[0].Continued {
		t.Error("First block should have Continued=true")
	}

	// Serialize snapshot and deserialize
	snapshot := thread.Snapshot()
	data, _ := json.Marshal(snapshot)
	var restored Snapshot
	json.Unmarshal(data, &restored)

	// Verify Continued is preserved
	if !restored.Blocks[0].Continued {
		t.Error("Continued flag not preserved after JSON round-trip")
	}
}

func TestSnapshot_ToolCallWithResult(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	// Add a tool call with result
	thread.ToolCall("call_1", "search", `{"q": "test"}`)
	thread.ToolResult(&ThreadToolCall{ID: "call_1"}, "search results here")

	// Verify initial state
	if thread.IncompleteToolCalls() != 0 {
		t.Errorf("Expected 0 incomplete tool calls, got %d", thread.IncompleteToolCalls())
	}

	// Serialize snapshot and restore
	snapshot := thread.Snapshot()
	data, _ := json.Marshal(snapshot)
	var restoredSnapshot Snapshot
	json.Unmarshal(data, &restoredSnapshot)

	restored := &Thread{Blocks: []*ThreadBlock{}}
	restored.Restore(&restoredSnapshot)

	// Verify tool result is preserved
	if len(restored.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(restored.Blocks))
	}
	if restored.Blocks[0].ToolResult == nil {
		t.Error("ToolResult not preserved after deserialization")
	}
	if restored.Blocks[0].ToolResult.Output != "search results here" {
		t.Errorf("ToolResult output mismatch: got %q", restored.Blocks[0].ToolResult.Output)
	}

	// Verify IncompleteToolCalls still works
	if restored.IncompleteToolCalls() != 0 {
		t.Errorf("Expected 0 incomplete tool calls after restore, got %d", restored.IncompleteToolCalls())
	}
}

func TestSnapshot_IncompleteToolCalls(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	// Add incomplete tool call
	thread.ToolCall("call_1", "search", `{"q": "test"}`)

	if thread.IncompleteToolCalls() != 1 {
		t.Errorf("Expected 1 incomplete tool call, got %d", thread.IncompleteToolCalls())
	}

	// Serialize snapshot and restore
	snapshot := thread.Snapshot()
	data, _ := json.Marshal(snapshot)
	var restoredSnapshot Snapshot
	json.Unmarshal(data, &restoredSnapshot)

	restored := &Thread{Blocks: []*ThreadBlock{}}
	restored.Restore(&restoredSnapshot)

	// Verify incomplete count is correct after restore
	if restored.IncompleteToolCalls() != 1 {
		t.Errorf("Expected 1 incomplete tool call after restore, got %d", restored.IncompleteToolCalls())
	}

	// Complete the tool call
	restored.ToolResult(&ThreadToolCall{ID: "call_1"}, "result")

	if restored.IncompleteToolCalls() != 0 {
		t.Errorf("Expected 0 incomplete tool calls after completion, got %d", restored.IncompleteToolCalls())
	}
}

func TestThread_SetError(t *testing.T) {
	thread := &Thread{}

	err := &AIError{
		Category: AIErrorCategoryAuthentication,
		Message:  "Invalid API key",
		Provider: "test",
	}
	thread.SetError(err)

	if thread.Success {
		t.Error("Expected Success to be false")
	}
	// AIError.Error() returns formatted string: "[provider] category: message"
	expectedError := "[test] authentication: Invalid API key"
	if thread.Error != expectedError {
		t.Errorf("Error mismatch: got %q, want %q", thread.Error, expectedError)
	}

	// Note: Thread.Error is execution state and is NOT serialized via Snapshot.
	// Callers should handle error state separately if persistence is needed.
}

func TestSnapshot_ImageSerialization(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	thread.InputImageBase64("aGVsbG8gd29ybGQ=", "image/png")

	// Serialize snapshot
	snapshot := thread.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Deserialize and restore
	var restoredSnapshot Snapshot
	if err := json.Unmarshal(data, &restoredSnapshot); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	restored := &Thread{Blocks: []*ThreadBlock{}}
	restored.Restore(&restoredSnapshot)

	// Verify
	if len(restored.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(restored.Blocks))
	}
	if restored.Blocks[0].Image == nil {
		t.Fatal("Image not preserved")
	}
	if restored.Blocks[0].Image.Base64 != "aGVsbG8gd29ybGQ=" {
		t.Errorf("Image base64 mismatch: got %q", restored.Blocks[0].Image.Base64)
	}
	if restored.Blocks[0].Image.MediaType != "image/png" {
		t.Errorf("Image media type mismatch: got %q", restored.Blocks[0].Image.MediaType)
	}
}

func TestSnapshot_WebSearchSerialization(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	thread.WebSearchQuery("search_1", "golang serialization")
	thread.WebSearchResult("search_1", ThreadWebSearchResult{
		Title: "Go JSON",
		URL:   "https://example.com",
	})

	// Serialize snapshot
	snapshot := thread.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Deserialize and restore
	var restoredSnapshot Snapshot
	if err := json.Unmarshal(data, &restoredSnapshot); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	restored := &Thread{Blocks: []*ThreadBlock{}}
	restored.Restore(&restoredSnapshot)

	// Verify
	if len(restored.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(restored.Blocks))
	}
	if restored.Blocks[0].WebSearch == nil {
		t.Fatal("WebSearch not preserved")
	}
	if restored.Blocks[0].WebSearch.Query != "golang serialization" {
		t.Errorf("WebSearch query mismatch: got %q", restored.Blocks[0].WebSearch.Query)
	}
}

func TestThread_SnapshotRestore(t *testing.T) {
	thread := &Thread{
		Model:    "claude-3-opus",
		ThreadId: "thread_123",
		Blocks:   []*ThreadBlock{},
	}
	thread.System("You are a helpful assistant.")
	thread.Input("Hello")
	thread.Text("text_1", "Hi there!")
	thread.Complete("text_1")

	// Create snapshot
	snapshot := thread.Snapshot()

	// Verify snapshot has blocks
	if len(snapshot.Blocks) != len(thread.Blocks) {
		t.Errorf("Snapshot block count mismatch: got %d, want %d", len(snapshot.Blocks), len(thread.Blocks))
	}

	// Create new thread and restore
	newThread := &Thread{
		Blocks: []*ThreadBlock{},
	}
	newThread.Restore(snapshot)

	// Verify blocks were restored
	if len(newThread.Blocks) != len(thread.Blocks) {
		t.Errorf("Restored block count mismatch: got %d, want %d", len(newThread.Blocks), len(thread.Blocks))
	}

	// Verify block content
	for i, b := range newThread.Blocks {
		if b.Type != thread.Blocks[i].Type {
			t.Errorf("Block %d type mismatch: got %q, want %q", i, b.Type, thread.Blocks[i].Type)
		}
		if b.Text != thread.Blocks[i].Text {
			t.Errorf("Block %d text mismatch: got %q, want %q", i, b.Text, thread.Blocks[i].Text)
		}
	}
}

func TestSnapshot_JSONRoundTrip(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}
	thread.System("System prompt")
	thread.Input("User input")
	thread.Text("text_1", "Response")
	thread.Complete("text_1")

	// Create snapshot and serialize
	snapshot := thread.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Deserialize
	var restored Snapshot
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Verify
	if len(restored.Blocks) != len(snapshot.Blocks) {
		t.Errorf("Block count mismatch: got %d, want %d", len(restored.Blocks), len(snapshot.Blocks))
	}
}

func TestSnapshot_ThinkingProviderID(t *testing.T) {
	thread := &Thread{
		CurrentProvider: "messages.anthropic",
		Blocks:          []*ThreadBlock{},
	}

	// Add thinking block
	thread.Thinking("thinking_1", "Let me think...")

	// Verify ProviderID is set
	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}
	if thread.Blocks[0].ProviderID != "messages.anthropic" {
		t.Errorf("ProviderID mismatch: got %q, want %q", thread.Blocks[0].ProviderID, "messages.anthropic")
	}

	// Serialize snapshot and verify ProviderID is preserved
	snapshot := thread.Snapshot()
	data, _ := json.Marshal(snapshot)
	var restoredSnapshot Snapshot
	json.Unmarshal(data, &restoredSnapshot)

	if restoredSnapshot.Blocks[0].ProviderID != "messages.anthropic" {
		t.Errorf("ProviderID not preserved: got %q", restoredSnapshot.Blocks[0].ProviderID)
	}
}

func TestThread_EncryptedThinkingProviderID(t *testing.T) {
	thread := &Thread{
		CurrentProvider: "messages.anthropic",
		Blocks:          []*ThreadBlock{},
	}

	// Add encrypted thinking block
	thread.EncryptedThinking("encrypted_data")

	// Verify ProviderID is set
	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}
	if thread.Blocks[0].ProviderID != "messages.anthropic" {
		t.Errorf("ProviderID mismatch: got %q, want %q", thread.Blocks[0].ProviderID, "messages.anthropic")
	}
}
