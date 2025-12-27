package aikit

import (
	"encoding/json"
	"testing"
)

// TestToolDefinitionSerialization verifies that tool definitions are properly
// serialized when using map[string]any for providers like Messages, Completions, and AIStudio.
func TestToolDefinitionSerialization(t *testing.T) {
	// Create a tool definition with parameters
	toolDef := ToolDefinition{
		Description: "Search for information on a topic",
		Parameters: &ToolJsonSchema{
			Type: "object",
			Properties: &map[string]*ToolJsonSchema{
				"query": {
					Type:        "string",
					Description: "The search query",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results",
				},
			},
			Required: []string{"query"},
		},
	}

	// Test serialization when assigned to map[string]any (like Messages/Completions do)
	toolSpec := map[string]any{}
	toolSpec["description"] = toolDef.Description
	toolSpec["parameters"] = toolDef.Parameters
	toolSpec["name"] = "search"

	// Marshal and unmarshal to verify serialization
	data, err := json.Marshal(toolSpec)
	if err != nil {
		t.Fatalf("Failed to marshal tool spec: %v", err)
	}

	// Parse the JSON to verify structure
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal tool spec: %v", err)
	}

	// Verify parameters are present and properly structured
	params, ok := parsed["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("Parameters not properly serialized: got %T", parsed["parameters"])
	}

	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got %v", params["type"])
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Properties not properly serialized: got %T", params["properties"])
	}

	if len(props) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(props))
	}

	queryProp, ok := props["query"].(map[string]any)
	if !ok {
		t.Fatalf("Query property not properly serialized: got %T", props["query"])
	}

	if queryProp["type"] != "string" {
		t.Errorf("Expected query type 'string', got %v", queryProp["type"])
	}

	if queryProp["description"] != "The search query" {
		t.Errorf("Expected query description 'The search query', got %v", queryProp["description"])
	}
}

// TestMessagesToolArgumentAccumulation tests that the Messages API properly accumulates
// tool call arguments from streaming input_json_delta events.
func TestMessagesToolArgumentAccumulation(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	// Simulate content_block_start with tool_use
	// This creates the initial tool call
	thread.ToolCall("call_123", "search", "")

	// Simulate multiple input_json_delta chunks
	// The arguments are streamed in parts
	chunks := []string{
		`{"`,
		`query":`,
		` "hello`,
		` world`,
		`"}`,
	}

	for _, chunk := range chunks {
		thread.ToolCall("call_123", "", chunk)
	}

	// Verify the tool call block exists and has accumulated arguments
	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	block := thread.Blocks[0]
	if block.Type != InferenceBlockToolCall {
		t.Fatalf("Expected block type %s, got %s", InferenceBlockToolCall, block.Type)
	}

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

// TestCompletionsToolArgumentAccumulation tests that the Completions API properly accumulates
// tool call arguments from streaming function call events.
func TestCompletionsToolArgumentAccumulation(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	// First chunk: tool call with name and empty arguments
	thread.ToolCall("call_456", "get_weather", "")

	// Subsequent chunks: arguments streamed in parts
	chunks := []string{
		`{"location": "`,
		`New York`,
		`", "unit": "`,
		`celsius`,
		`"}`,
	}

	for _, chunk := range chunks {
		thread.ToolCall("call_456", "", chunk)
	}

	// Verify the tool call block exists and has accumulated arguments
	if len(thread.Blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(thread.Blocks))
	}

	block := thread.Blocks[0]
	if block.Type != InferenceBlockToolCall {
		t.Fatalf("Expected block type %s, got %s", InferenceBlockToolCall, block.Type)
	}

	if block.ToolCall == nil {
		t.Fatal("ToolCall is nil")
	}

	expectedArgs := `{"location": "New York", "unit": "celsius"}`
	if block.ToolCall.Arguments != expectedArgs {
		t.Errorf("Expected arguments %q, got %q", expectedArgs, block.ToolCall.Arguments)
	}

	if block.ToolCall.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %q", block.ToolCall.Name)
	}
}

// TestMultipleToolCallsAccumulation tests that multiple concurrent tool calls
// each accumulate their arguments independently.
func TestMultipleToolCallsAccumulation(t *testing.T) {
	thread := &Thread{
		Blocks: []*ThreadBlock{},
	}

	// Start two tool calls
	thread.ToolCall("call_1", "tool_a", "")
	thread.ToolCall("call_2", "tool_b", "")

	// Interleaved argument chunks for both tools
	thread.ToolCall("call_1", "", `{"a": `)
	thread.ToolCall("call_2", "", `{"b": `)
	thread.ToolCall("call_1", "", `1}`)
	thread.ToolCall("call_2", "", `2}`)

	// Verify both tool calls
	if len(thread.Blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(thread.Blocks))
	}

	// Find blocks by ID
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

// TestMessagesOnChunkToolCall tests that MessagesAPIRequest.OnChunk properly handles
// tool call streaming events and accumulates arguments.
func TestMessagesOnChunkToolCall(t *testing.T) {
	provider := AnthropicProvider("test-key")
	session := provider.Session()
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

	// Simulate content_block_start with tool_use
	startChunk := []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_abc","name":"search","input":{}}}`)
	result := req.OnChunk(startChunk, session.Thread)
	if result.Error != nil {
		t.Fatalf("OnChunk failed on content_block_start: %v", result.Error)
	}

	// Simulate content_block_delta with input_json_delta
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

	// Find the tool call block
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

	expectedArgs := `{"query": "test query"}`
	if toolBlock.ToolCall.Arguments != expectedArgs {
		t.Errorf("Expected arguments %q, got %q", expectedArgs, toolBlock.ToolCall.Arguments)
	}
}

// TestCompletionsOnChunkToolCall tests that CompletionsAPIRequest.OnChunk properly handles
// tool call streaming events and accumulates arguments.
func TestCompletionsOnChunkToolCall(t *testing.T) {
	provider := GroqProvider("test-key")
	session := provider.Session()
	session.Thread.Model = "llama-3"
	session.Thread.Tools = map[string]ToolDefinition{
		"get_weather": {
			Description: "Get weather information",
			Parameters: &ToolJsonSchema{
				Type: "object",
				Properties: &map[string]*ToolJsonSchema{
					"location": {Type: "string"},
				},
			},
		},
	}

	req := session.Provider.(*CompletionsAPIRequest)
	req.InitSession(session.Thread)

	// Simulate streaming chunks with tool calls
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

	// Find the tool call block
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

	expectedArgs := `{"location": "Paris"}`
	if toolBlock.ToolCall.Arguments != expectedArgs {
		t.Errorf("Expected arguments %q, got %q", expectedArgs, toolBlock.ToolCall.Arguments)
	}
}
