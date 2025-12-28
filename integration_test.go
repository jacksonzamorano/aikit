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
	Provider        aikit.ProviderConfig
	Model           string
	ReasoningEffort *string
	TestName        string
}

// =============================================================================
// SHARED VALIDATION RUNNER - TOOL CALLS
// =============================================================================

func runToolCallValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	all := ""
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
	if cfg.ReasoningEffort != nil {
		session.Thread.ReasoningEffort = *cfg.ReasoningEffort
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
	writeTestRun(cfg.TestName+"_tool", all)

	// Run all validations
	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateToolCallPairing(t, result)
	validateBlockIDUniqueness(t, result)
	validateToolArguments(t, result)
	validateToolFunctionExecution(t, toolFunctionCalled, result)
	validateReasoningBlocks(t, cfg.ReasoningEffort, result)
}

// =============================================================================
// SHARED VALIDATION RUNNER - WEB SEARCH
// =============================================================================

func runWebSearchValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	all := ""
	var lastHash string

	session := cfg.Provider.Session()
	session.Thread.Model = cfg.Model
	session.Thread.MaxWebSearches = 1
	session.Thread.CoalesceTextBlocks = true
	session.Thread.System("You are a helpful assistant. Always check for the most up-to-date information.")
	session.Thread.Input("What's new in the newest version of React? Keep your answer concise.")
	if cfg.ReasoningEffort != nil {
		session.Thread.ReasoningEffort = *cfg.ReasoningEffort
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
	writeTestRun(cfg.TestName+"_websearch", all)

	// Run all validations
	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateBlockIDUniqueness(t, result)
	validateWebSearchResults(t, result)
	validateReasoningBlocks(t, cfg.ReasoningEffort, result)
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
	if cfg.ReasoningEffort != nil {
		session.Thread.ReasoningEffort = *cfg.ReasoningEffort
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
	validateReasoningBlocks(t, cfg.ReasoningEffort, result)
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
		if b.AliasFor != nil && b.AliasId == "" {
			t.Errorf("Block %s is an alias but has no AliasId.", b.ID)
		}
		if b.AliasId != "" && b.AliasFor == nil {
			t.Errorf("Block %s has an AliasId but is not an alias.", b.ID)
		}
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

func validateReasoningBlocks(t *testing.T, reasoningEffort *string, result *aikit.Thread) {
	t.Helper()
	if reasoningEffort == nil || *reasoningEffort == "" {
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
		t.Logf("Note: Reasoning effort set to %q but no thinking blocks found", *reasoningEffort)
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

var anthropicReasoningEffort = "1024"

func TestIntegration_Anthropic_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		ReasoningEffort: &anthropicReasoningEffort,
		TestName:        "anthropic",
	})
}

func TestIntegration_Anthropic_WebSearch(t *testing.T) {
	runWebSearchValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		ReasoningEffort: &anthropicReasoningEffort,
		TestName:        "anthropic",
	})
}

func TestIntegration_Anthropic_ImageInput(t *testing.T) {
	runImageInputValidation(t, integrationTestConfig{
		Provider:        aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")),
		Model:           "claude-haiku-4-5-20251001",
		ReasoningEffort: &anthropicReasoningEffort,
		TestName:        "anthropic",
	})
}

// =============================================================================
// OPENAI (RESPONSES API) INTEGRATION TESTS
// =============================================================================

var openaiReasoningEffort = "low"

func TestIntegration_OpenAI_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		ReasoningEffort: &openaiReasoningEffort,
		TestName:        "openai",
	})
}

func TestIntegration_OpenAI_WebSearch(t *testing.T) {
	runWebSearchValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		ReasoningEffort: &openaiReasoningEffort,
		TestName:        "openai",
	})
}

func TestIntegration_OpenAI_ImageInput(t *testing.T) {
	runImageInputValidation(t, integrationTestConfig{
		Provider:        aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")),
		Model:           "gpt-5-nano",
		ReasoningEffort: &openaiReasoningEffort,
		TestName:        "openai",
	})
}

// =============================================================================
// GOOGLE (AI STUDIO API) INTEGRATION TESTS
// =============================================================================

var googleReasoningEffort = "1024"

func TestIntegration_Google_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.GoogleProvider(os.Getenv("GOOGLE_KEY")),
		Model:           "gemini-3-flash-preview",
		ReasoningEffort: &googleReasoningEffort,
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

var fireworksReasoningEffort = "low"

func TestIntegration_Fireworks_ToolCall(t *testing.T) {
	runToolCallValidation(t, integrationTestConfig{
		Provider:        aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")),
		Model:           "accounts/fireworks/models/gpt-oss-20b",
		ReasoningEffort: &fireworksReasoningEffort,
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
