package aikit_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jacksonzamorano/aikit"
)

// =============================================================================
// TEST CONFIGURATION
// =============================================================================

var testDebugEnabled = false

// =============================================================================
// MODULAR TEST ARCHITECTURE
// =============================================================================

type integrationTestConfig struct {
	Provider  aikit.ProviderConfig
	Model     string
	Reasoning *aikit.ReasoningConfig
	TestName  string
}

type snapshotTestConfig struct {
	Provider1  aikit.ProviderConfig
	Model1     string
	Reasoning1 *aikit.ReasoningConfig
	Provider2  aikit.ProviderConfig
	Model2     string
	Reasoning2 *aikit.ReasoningConfig
	TestName   string
}

// Memory tools for snapshot tests - allows data dependency between phases
var memoryTools = map[string]aikit.ToolDefinition{
	"memory_store": {
		Description: "Store a value in memory with a key",
		Parameters: &aikit.ToolJsonSchema{
			Type: "object",
			Properties: &map[string]*aikit.ToolJsonSchema{
				"key":   {Type: "string", Description: "The key to store the value under"},
				"value": {Type: "string", Description: "The value to store"},
			},
			Required: []string{"key", "value"},
		},
	},
	"memory_get": {
		Description: "Retrieve a value from memory by key",
		Parameters: &aikit.ToolJsonSchema{
			Type: "object",
			Properties: &map[string]*aikit.ToolJsonSchema{
				"key": {Type: "string", Description: "The key to retrieve"},
			},
			Required: []string{"key"},
		},
	},
}

// =============================================================================
// SHARED VALIDATION RUNNER - TOOL CALLS
// =============================================================================

func runToolCallValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	var lastHash string
	toolFunctionCalled := 0

	session := cfg.Provider.Session()
	session.Thread.Model = cfg.Model
	session.Thread.System("You are a helpful assistant. You will always request the current time using the get_time tool with the timezone parameter set to 'UTC', and use the result in your response.")
	session.Thread.Input("What date is exactly 365 days from today, and what day of the week will it be?")
	session.Thread.Tools = map[string]aikit.ToolDefinition{
		"get_time": {
			Description: "Get the current time in ISO 8601 format for a specific timezone.",
			Parameters: &aikit.ToolJsonSchema{
				Type: "object",
				Properties: &map[string]*aikit.ToolJsonSchema{
					"timezone": {
						Type:        "string",
						Description: "The timezone to get the time for (e.g., 'UTC', 'America/New_York', 'Europe/London').",
					},
				},
				Required: []string{"timezone"},
			},
		},
	}
	if cfg.Reasoning != nil {
		session.Thread.Reasoning = *cfg.Reasoning
	}
	session.Thread.CoalesceTextBlocks = true
	session.Thread.HandleToolFunction = func(name string, args string) string {
		toolFunctionCalled++
		switch name {
		case "get_time":
			var parsedArgs struct {
				Timezone string `json:"timezone"`
			}
			if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
				return fmt.Sprintf("Error: Invalid arguments: %s", err.Error())
			}
			if parsedArgs.Timezone == "" {
				return "Error: timezone parameter is required"
			}
			loc, err := time.LoadLocation(parsedArgs.Timezone)
			if err != nil {
				loc = time.UTC
			}
			return time.Now().In(loc).Format(time.RFC3339)
		default:
			return fmt.Sprintf("Error: Unknown tool: %s", name)
		}
	}
	session.Debug = testDebugEnabled

	result := session.Stream(func(result *aikit.Thread) {
		// Streaming hash uniqueness check
		bytes, _ := json.Marshal(result.Blocks)
		hash := sha256.Sum256(bytes)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash == lastHash && lastHash != "" {
			t.Errorf("Streaming callback received duplicate data")
		}
		lastHash = currentHash
	})
	all := snapshotResult(*result)

	// Write test run data
	writeTestRun(cfg.TestName+"_tool", all)

	// Run all validations
	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateToolCallPairing(t, result)
	validateBlockIDUniqueness(t, result)
	validateToolArguments(t, result)
	validateToolFunctionExecution(t, toolFunctionCalled, result)
	validateReasoningBlocks(t, cfg.Reasoning, result)
}

// =============================================================================
// SHARED VALIDATION RUNNER - WEB SEARCH
// =============================================================================

func runWebSearchValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	var lastHash string

	session := cfg.Provider.Session()
	session.Thread.Model = cfg.Model
	session.Thread.MaxWebSearches = 1
	session.Thread.CoalesceTextBlocks = true
	session.Thread.System("You are a helpful assistant. Always check for the most up-to-date information.")
	session.Thread.Input("What's new in the newest version of React? Keep your answer concise.")
	if cfg.Reasoning != nil {
		session.Thread.Reasoning = *cfg.Reasoning
	}
	session.Debug = testDebugEnabled

	result := session.Stream(func(result *aikit.Thread) {
		// Streaming hash uniqueness check
		bytes, _ := json.Marshal(result.Blocks)
		hash := sha256.Sum256(bytes)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash == lastHash && lastHash != "" {
			t.Errorf("Streaming callback received duplicate data")
		}
		lastHash = currentHash
	})
	all := snapshotResult(*result)

	// Write test run data
	writeTestRun(cfg.TestName+"_websearch", all)

	// Run all validations
	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateBlockIDUniqueness(t, result)
	validateWebSearchResults(t, result)
	validateReasoningBlocks(t, cfg.Reasoning, result)
}

// =============================================================================
// SHARED VALIDATION RUNNER - IMAGE INPUT
// =============================================================================

func runImageInputValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	// Read test image
	imageData, err := os.ReadFile("test_image.jpg")
	if err != nil {
		t.Fatalf("Failed to read test image: %v", err)
	}

	all := ""
	var lastHash string

	session := cfg.Provider.Session()
	session.Thread.Model = cfg.Model
	session.Thread.CoalesceTextBlocks = true
	session.Thread.System("You are a helpful assistant that identifies images.")
	session.Thread.InputImage(imageData, "image/jpeg")
	session.Thread.Input("What famous video is this frame from?")
	if cfg.Reasoning != nil {
		session.Thread.Reasoning = *cfg.Reasoning
	}
	session.Debug = testDebugEnabled

	result := session.Stream(func(result *aikit.Thread) {
		all += snapshotResult(*result)

		// Streaming hash uniqueness check
		bytes, _ := json.Marshal(result.Blocks)
		hash := sha256.Sum256(bytes)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash == lastHash && lastHash != "" {
			t.Errorf("Streaming callback received duplicate data")
		}
		lastHash = currentHash
	})
	all += snapshotResult(*result)

	// Write test run data
	writeTestRun(cfg.TestName+"_image", all)

	// Run all validations
	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateBlockIDUniqueness(t, result)
	validateImageInputResponse(t, result)
	validateReasoningBlocks(t, cfg.Reasoning, result)
}

// =============================================================================
// SHARED VALIDATION RUNNER - SNAPSHOT RESTORE
// =============================================================================

func runSnapshotRestoreValidation(t *testing.T, cfg snapshotTestConfig) {
	t.Helper()

	memoryStore := make(map[string]string)
	secretValue := fmt.Sprintf("secret_%d", time.Now().UnixNano())

	// Memory tool handler - shared across both sessions
	handleMemoryTool := func(name string, args string) string {
		var parsed struct {
			Key   string `json:"key"`
			Value string `json:"value,omitempty"`
		}
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return fmt.Sprintf("Error: Invalid arguments: %s", err.Error())
		}

		switch name {
		case "memory_store":
			memoryStore[parsed.Key] = parsed.Value
			return fmt.Sprintf("Stored '%s' = '%s'", parsed.Key, parsed.Value)
		case "memory_get":
			if val, ok := memoryStore[parsed.Key]; ok {
				return val
			}
			return "Key not found"
		default:
			return fmt.Sprintf("Error: Unknown tool: %s", name)
		}
	}

	all := ""

	// ==========================================================================
	// Phase 1: Initial conversation with Provider 1
	// ==========================================================================
	session1 := cfg.Provider1.Session()
	session1.Thread.Model = cfg.Model1
	if cfg.Reasoning1 != nil {
		session1.Thread.Reasoning = *cfg.Reasoning1
	}
	session1.Thread.Tools = memoryTools
	session1.Thread.HandleToolFunction = handleMemoryTool
	session1.Thread.CoalesceTextBlocks = true
	session1.Thread.System("You are a helpful assistant with memory capabilities. When asked to store something, use the memory_store tool. When asked to retrieve something, use the memory_get tool.")
	session1.Thread.Input(fmt.Sprintf("Please store the value '%s' with key 'secret' using the memory_store tool, then confirm what you stored.", secretValue))
	session1.Debug = testDebugEnabled

	result1 := session1.Stream(func(result *aikit.Thread) {})
	all += snapshotResult(*result1)

	// Validate Phase 1
	if !result1.Success {
		t.Fatalf("Phase 1 failed: %s", result1.Error)
	}
	validateBasicResults(t, result1)
	validateToolCallPairing(t, result1)

	// Verify the tool was actually called with the secret value
	if storedVal, ok := memoryStore["secret"]; !ok || storedVal != secretValue {
		t.Errorf("Phase 1: Expected secret value '%s' to be stored, got '%s'", secretValue, storedVal)
	}

	// ==========================================================================
	// Phase 2: Serialize and restore snapshot
	// ==========================================================================
	snapshot := session1.Thread.Snapshot()

	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Deserialize to new snapshot
	var restoredSnapshot aikit.Snapshot
	if err := json.Unmarshal(data, &restoredSnapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Validate snapshot integrity
	validateSnapshotIntegrity(t, snapshot, &restoredSnapshot)

	// ==========================================================================
	// Phase 3: Continue conversation with Provider 2
	// ==========================================================================
	session2 := cfg.Provider2.Session()
	session2.Thread.Model = cfg.Model2
	if cfg.Reasoning2 != nil {
		session2.Thread.Reasoning = *cfg.Reasoning2
	}
	session2.Thread.Restore(&restoredSnapshot)
	session2.Thread.Tools = memoryTools
	session2.Thread.HandleToolFunction = handleMemoryTool
	session2.Thread.CoalesceTextBlocks = true
	session2.Thread.Input("What value did you store earlier? Use the memory_get tool with key 'secret' to retrieve it and tell me the value.")
	session2.Debug = testDebugEnabled

	result2 := session2.Stream(func(result *aikit.Thread) {})
	all += snapshotResult(*result2)

	// Write test run data
	writeTestRun(cfg.TestName+"_snapshot", all)

	// Validate Phase 2
	if !result2.Success {
		t.Fatalf("Phase 2 failed: %s", result2.Error)
	}
	validateBasicResults(t, result2)
	validateCrossProviderContext(t, result2, secretValue)
}

// =============================================================================
// VALIDATION FUNCTIONS
// =============================================================================

func validateBasicResults(t *testing.T, result *aikit.Thread) {
	t.Helper()
	if !result.Success {
		t.Error(result.Error)
	}
	if result.Result.OutputTokens == 0 {
		t.Fatalf("Received no output tokens.")
	}
	if result.Result.InputTokens == 0 {
		t.Fatalf("Received no input tokens.")
	}
}

func validateBlockIntegrity(t *testing.T, result *aikit.Thread) {
	t.Helper()
	for _, b := range result.Blocks {
		if !b.Complete {
			t.Errorf("Block %s of type %s not marked complete.", b.ID, b.Type)
		}
		if b.ID == "" && b.Type != aikit.InferenceBlockInput && b.Type != aikit.InferenceBlockSystem && b.Type != aikit.InferenceBlockInputImage {
			t.Errorf("Block of type %s has no ID.", b.Type)
		}
		// Continued blocks are valid - no special validation needed
	}
}

func validateToolCallPairing(t *testing.T, result *aikit.Thread) {
	t.Helper()
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockToolCall && b.ToolCall != nil {
			if b.ToolResult == nil {
				t.Errorf("Tool call %s (%s) has no ToolResult", b.ID, b.ToolCall.Name)
			}
		}
	}
}

func validateBlockIDUniqueness(t *testing.T, result *aikit.Thread) {
	t.Helper()
	seenIds := make(map[string]bool)
	for _, b := range result.Blocks {
		if b.ID != "" {
			if seenIds[b.ID] {
				t.Errorf("Duplicate block ID: %s", b.ID)
			}
			seenIds[b.ID] = true
		}
	}
}

func validateToolArguments(t *testing.T, result *aikit.Thread) {
	t.Helper()
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockToolCall && b.ToolCall != nil {
			if b.ToolCall.Name == "get_time" {
				var args struct {
					Timezone string `json:"timezone"`
				}
				if err := json.Unmarshal([]byte(b.ToolCall.Arguments), &args); err != nil {
					t.Errorf("Tool call %s has invalid JSON arguments: %s (raw: %q)", b.ID, err.Error(), b.ToolCall.Arguments)
				} else if args.Timezone == "" {
					t.Errorf("Tool call %s missing required 'timezone' parameter (raw: %q)", b.ID, b.ToolCall.Arguments)
				}
			}
		}
	}
}

func validateToolFunctionExecution(t *testing.T, callCount int, result *aikit.Thread) {
	t.Helper()
	toolCallBlocks := 0
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockToolCall && b.ToolCall != nil {
			toolCallBlocks++
		}
	}
	if toolCallBlocks > 0 && callCount == 0 {
		t.Errorf("Tool calls exist (%d) but HandleToolFunction was never called", toolCallBlocks)
	}
}

func validateWebSearchResults(t *testing.T, result *aikit.Thread) {
	t.Helper()
	webSearchCount := 0
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockWebSearch {
			webSearchCount++
			if b.WebSearch == nil {
				t.Errorf("WebSearch block %s has nil WebSearch", b.ID)
				continue
			}
			// Either query or results should be populated
			if b.WebSearch.Query == "" && len(b.WebSearch.Results) == 0 {
				t.Errorf("WebSearch block %s has neither query nor results", b.ID)
			}
			for i, res := range b.WebSearch.Results {
				if res.Title == "" {
					t.Errorf("WebSearch block %s result %d missing Title", b.ID, i)
				}
				if res.URL == "" {
					t.Errorf("WebSearch block %s result %d missing URL", b.ID, i)
				}
			}
		}
	}
	if webSearchCount == 0 {
		t.Logf("Note: No web search blocks found (provider may handle differently)")
	}
}

func validateReasoningBlocks(t *testing.T, reasoning *aikit.ReasoningConfig, result *aikit.Thread) {
	t.Helper()
	if reasoning == nil || (reasoning.Effort == "" && reasoning.Budget == 0) {
		return
	}
	hasThinking := false
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockThinking || b.Type == aikit.InferenceBlockEncryptedThinking {
			hasThinking = true
			break
		}
	}
	if !hasThinking {
		t.Logf("Note: Reasoning configured but no thinking blocks found")
	}
}

func validateImageInputResponse(t *testing.T, result *aikit.Thread) {
	t.Helper()
	hasTextResponse := false
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockText && len(b.Text) > 0 {
			hasTextResponse = true
			break
		}
	}
	if !hasTextResponse {
		t.Error("No text response found for image input")
	}
}

func validateSnapshotIntegrity(t *testing.T, original *aikit.Snapshot, restored *aikit.Snapshot) {
	t.Helper()
	if len(original.Blocks) != len(restored.Blocks) {
		t.Errorf("Snapshot block count mismatch: original=%d, restored=%d", len(original.Blocks), len(restored.Blocks))
		return
	}
	for i, origBlock := range original.Blocks {
		restoredBlock := restored.Blocks[i]
		if origBlock.Type != restoredBlock.Type {
			t.Errorf("Block %d type mismatch: original=%s, restored=%s", i, origBlock.Type, restoredBlock.Type)
		}
		if origBlock.Text != restoredBlock.Text {
			t.Errorf("Block %d text mismatch", i)
		}
		if origBlock.ID != restoredBlock.ID {
			t.Errorf("Block %d ID mismatch: original=%s, restored=%s", i, origBlock.ID, restoredBlock.ID)
		}
	}
}

func validateCrossProviderContext(t *testing.T, result *aikit.Thread, expectedValue string) {
	t.Helper()

	// Check if there's a memory_get tool call
	hasMemoryGet := false
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockToolCall && b.ToolCall != nil {
			if b.ToolCall.Name == "memory_get" {
				hasMemoryGet = true
				// Verify the tool result contains the expected value
				if b.ToolResult != nil && !strings.Contains(b.ToolResult.Output, expectedValue) {
					t.Errorf("memory_get tool result doesn't contain expected value '%s', got '%s'", expectedValue, b.ToolResult.Output)
				}
			}
		}
	}

	if !hasMemoryGet {
		t.Log("Note: No memory_get tool call found - model may have used context from conversation history instead")
	}

	// Check if the response text mentions the secret value
	hasSecretInResponse := false
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockText && strings.Contains(b.Text, expectedValue) {
			hasSecretInResponse = true
			break
		}
	}
	if !hasSecretInResponse {
		t.Errorf("Response doesn't contain the expected secret value '%s'", expectedValue)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func snapshotResult(results aikit.Thread) string {
	bytes, err := json.MarshalIndent(results.Blocks, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bytes) + ","
}

func writeTestRun(name string, results string) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	testPath := path.Join(cwd, "tests")
	os.MkdirAll(testPath, 0755)
	resultsCleaned := fmt.Sprintf("[%s]", strings.TrimRight(results, ","))
	testRunPath := path.Join(testPath, fmt.Sprintf("run_%s_%d.json", name, time.Now().UnixNano()))
	os.WriteFile(testRunPath, []byte(resultsCleaned), 0644)
}

// =============================================================================
// ANTHROPIC (MESSAGES API) INTEGRATION TESTS
// =============================================================================

var anthropicReasoning = aikit.ReasoningConfig{Budget: 1024}

func TestIntegration_Anthropic_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		Reasoning: &anthropicReasoning,
		TestName:        "anthropic",
	})
}

func TestIntegration_Anthropic_WebSearch(t *testing.T) {
	runWebSearchValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		Reasoning: &anthropicReasoning,
		TestName:        "anthropic",
	})
}

func TestIntegration_Anthropic_ImageInput(t *testing.T) {
	runImageInputValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		Reasoning: &anthropicReasoning,
		TestName:        "anthropic",
	})
}

// =============================================================================
// OPENAI (RESPONSES API) INTEGRATION TESTS
// =============================================================================

var openaiReasoning = aikit.ReasoningConfig{Effort: "low"}

func TestIntegration_OpenAI_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		Reasoning: &openaiReasoning,
		TestName:        "openai",
	})
}

func TestIntegration_OpenAI_WebSearch(t *testing.T) {
	runWebSearchValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		Reasoning: &openaiReasoning,
		TestName:        "openai",
	})
}

func TestIntegration_OpenAI_ImageInput(t *testing.T) {
	runImageInputValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		Reasoning: &openaiReasoning,
		TestName:        "openai",
	})
}

// =============================================================================
// GOOGLE (AI STUDIO API) INTEGRATION TESTS
// =============================================================================

var googleReasoning = aikit.ReasoningConfig{Budget: 1024}

func TestIntegration_Google_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.GoogleProvider(os.Getenv("GOOGLE_KEY")),
		Model:           "gemini-3-flash-preview",
		Reasoning: &googleReasoning,
		TestName:        "google",
	})
}

// =============================================================================
// GROQ (COMPLETIONS API) INTEGRATION TESTS
// =============================================================================

func TestIntegration_Groq_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider: aikit.GroqProvider(os.Getenv("GROQ_KEY")),
		Model:    "openai/gpt-oss-20b",
		TestName: "groq",
	})
}

// =============================================================================
// FIREWORKS (COMPLETIONS API) INTEGRATION TESTS
// =============================================================================

var fireworksReasoning = aikit.ReasoningConfig{Effort: "low"}

func TestIntegration_Fireworks_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")),
		Model:           "accounts/fireworks/models/gpt-oss-20b",
		Reasoning: &fireworksReasoning,
		TestName:        "fireworks",
	})
}

// =============================================================================
// XAI (COMPLETIONS API) INTEGRATION TESTS
// =============================================================================

func TestIntegration_XAI_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider: aikit.XAIProvider(os.Getenv("XAI_KEY")),
		Model:    "grok-4-1-fast-reasoning-latest",
		TestName: "xai",
	})
}

func TestIntegration_XAI_ImageInput(t *testing.T) {
	runImageInputValidation(t, integrationTestConfig{
		Provider: aikit.XAIProvider(os.Getenv("XAI_KEY")),
		Model:    "grok-4-1-fast-reasoning-latest",
		TestName: "xai",
	})
}

// =============================================================================
// SNAPSHOT/RESTORE INTEGRATION TESTS
// =============================================================================

func TestIntegration_Snapshot_SameProvider_Anthropic(t *testing.T) {
	runSnapshotRestoreValidation(t, snapshotTestConfig{
		Provider1:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model1:           "claude-haiku-4-5-20251001",
		Reasoning1: &anthropicReasoning,
		Provider2:  aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model2:     "claude-haiku-4-5-20251001",
		Reasoning2: &anthropicReasoning,
		TestName:         "snapshot_anthropic_to_anthropic",
	})
}

func TestIntegration_Snapshot_CrossProvider_Anthropic_OpenAI(t *testing.T) {
	// Cross-provider snapshot/restore with tool calls is not supported because
	// tool call IDs are provider-specific (Anthropic uses toolu_..., OpenAI uses call_...).
	// This test validates context preservation without tool call history.
	runSnapshotRestoreValidation(t, snapshotTestConfig{
		Provider1:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model1:           "claude-haiku-4-5-20251001",
		Reasoning1: &anthropicReasoning,
		Provider2:  aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model2:     "gpt-5-nano",
		Reasoning2: &openaiReasoning,
		TestName:         "snapshot_anthropic_to_openai",
	})
}
