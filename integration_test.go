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
// MODULAR TEST ARCHITECTURE
// =============================================================================

type integrationTestConfig struct {
	Provider  aikit.ProviderConfig
	Model     string
	Reasoning *aikit.ReasoningConfig
	TestName  string
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
			Parameters: &aikit.JsonSchema{
				Type: "object",
				Properties: &map[string]*aikit.JsonSchema{
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
// SHARED VALIDATION RUNNER - STRUCTURED OUTPUT
// =============================================================================

func runStructuredOutputValidation(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	var lastHash string

	session := cfg.Provider.Session()
	session.Thread.Model = cfg.Model
	session.Thread.System("Return only JSON that matches the provided schema.")
	session.Thread.Input("Return the number 2+2 as a string value.")
	session.Thread.StructuredOutputSchema = structuredOutputSchema()
	if cfg.Reasoning != nil {
		session.Thread.Reasoning = *cfg.Reasoning
	}
	session.Thread.CoalesceTextBlocks = true

	result := session.Stream(func(result *aikit.Thread) {
		bytes, _ := json.Marshal(result.Blocks)
		hash := sha256.Sum256(bytes)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash == lastHash && lastHash != "" {
			t.Errorf("Streaming callback received duplicate data")
		}
		lastHash = currentHash
	})
	all := snapshotResult(*result)

	writeTestRun(cfg.TestName+"_structured", all)

	validateBasicResults(t, result)
	validateBlockIntegrity(t, result)
	validateBlockIDUniqueness(t, result)
	validateStructuredOutputFormat(t, result)
	validateReasoningBlocks(t, cfg.Reasoning, result)
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

func validateStructuredOutputFormat(t *testing.T, result *aikit.Thread) {
	t.Helper()

	var output strings.Builder
	for _, b := range result.Blocks {
		if b.Type == aikit.InferenceBlockText {
			output.WriteString(b.Text)
		}
	}

	trimmed := strings.TrimSpace(output.String())
	if trimmed == "" {
		t.Fatalf("Structured output is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		t.Fatalf("Expected JSON output, got %q", trimmed)
	}
	value, ok := payload["answer"]
	if !ok {
		t.Fatalf("Structured output missing 'answer' field")
	}
	answer, ok := value.(string)
	if !ok {
		t.Fatalf("Structured output 'answer' should be string, got %T", value)
	}
	if strings.TrimSpace(answer) == "" {
		t.Fatalf("Structured output 'answer' is empty")
	}
}

func structuredOutputSchema() *aikit.JsonSchema {
	return &aikit.JsonSchema{
		Type: "object",
		Properties: &map[string]*aikit.JsonSchema{
			"answer": {Type: "string"},
		},
		Required: []string{"answer"},
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
// PROVIDER MATRIX
// =============================================================================

type providerTestCase struct {
	Name             string
	Provider         func() aikit.ProviderConfig
	Model            string
	Reasoning        *aikit.ReasoningConfig
	ToolCall         bool
	WebSearch        bool
	ImageInput       bool
	StructuredOutput bool
}

var integrationProviders = []providerTestCase{
	{
		Name:             "Anthropic",
		Provider:         func() aikit.ProviderConfig { return aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")) },
		Model:            "claude-haiku-4-5-20251001",
		Reasoning:        &aikit.ReasoningConfig{Budget: 1024},
		ToolCall:         true,
		WebSearch:        true,
		ImageInput:       true,
		StructuredOutput: true,
	},
	{
		Name:             "OpenAI",
		Provider:         func() aikit.ProviderConfig { return aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")) },
		Model:            "gpt-5-nano",
		Reasoning:        &aikit.ReasoningConfig{Effort: "low"},
		ToolCall:         true,
		WebSearch:        true,
		ImageInput:       true,
		StructuredOutput: true,
	},
	{
		Name:             "Google",
		Provider:         func() aikit.ProviderConfig { return aikit.GoogleProvider(os.Getenv("GOOGLE_KEY")) },
		Model:            "gemini-3-flash-preview",
		Reasoning:        &aikit.ReasoningConfig{Effort: "low"},
		ToolCall:         true,
		StructuredOutput: true,
	},
	{
		Name:             "Groq",
		Provider:         func() aikit.ProviderConfig { return aikit.GroqProvider(os.Getenv("GROQ_KEY")) },
		Model:            "openai/gpt-oss-20b",
		ToolCall:         true,
		StructuredOutput: true,
	},
	{
		Name:             "Fireworks",
		Provider:         func() aikit.ProviderConfig { return aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")) },
		Model:            "accounts/fireworks/models/gpt-oss-20b",
		Reasoning:        &aikit.ReasoningConfig{Effort: "low"},
		ToolCall:         true,
		StructuredOutput: true,
	},
	{
		Name:             "XAI",
		Provider:         func() aikit.ProviderConfig { return aikit.XAIProvider(os.Getenv("XAI_KEY")) },
		Model:            "grok-4-1-fast-reasoning-latest",
		ToolCall:         true,
		ImageInput:       true,
		StructuredOutput: true,
	},
	{
		Name:             "OpenRouter",
		Provider:         func() aikit.ProviderConfig { return aikit.OpenRouterProvider(os.Getenv("OPENROUTER_KEY")) },
		Model:            "openrouter/elephant-alpha",
		Reasoning:        &aikit.ReasoningConfig{Effort: "low"},
		ToolCall:         true,
		WebSearch:        true,
		ImageInput:       true,
		StructuredOutput: true,
	},
}

// =============================================================================
// INTEGRATION TEST RUNNER
// =============================================================================

func TestIntegration(t *testing.T) {
	for _, tc := range integrationProviders {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			cfg := integrationTestConfig{
				Provider:  tc.Provider(),
				Model:     tc.Model,
				Reasoning: tc.Reasoning,
				TestName:  strings.ToLower(tc.Name),
			}
			if tc.ToolCall {
				t.Run("ToolCall", func(t *testing.T) {
					runToolCallValidation(t, cfg)
				})
			}
			if tc.WebSearch {
				t.Run("WebSearch", func(t *testing.T) {
					runWebSearchValidation(t, cfg)
				})
			}
			if tc.ImageInput {
				t.Run("ImageInput", func(t *testing.T) {
					runImageInputValidation(t, cfg)
				})
			}
			if tc.StructuredOutput {
				t.Run("StructuredOutput", func(t *testing.T) {
					runStructuredOutputValidation(t, cfg)
				})
			}
		})
	}
}
