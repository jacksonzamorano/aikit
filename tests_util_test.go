package aikit_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jacksonzamorano/aikit"
)

type toolResponse struct {
	Time string `json:"time"`
}

func MakeRequest(provider aikit.InferenceProvider, modelname string, reasoning *string) (aikit.Session, aikit.InferenceRequest) {
	sys_prompt := "You are a helpful assistant. You will always request the current time using the get_time tool and embed it in your response."
	prompt := "What is the current time?"
	tools := map[string]aikit.ToolDefinition{
		"get_time": {
			Description: "Get the current time in ISO 8601 format.",
			Parameters: &aikit.ToolJsonSchema{
				Type:       "object",
				Properties: map[string]*aikit.ToolJsonSchema{},
			},
		},
	}
	session := aikit.Session{
		Provider: provider,
	}
	request := aikit.InferenceRequest{
		Model:           modelname,
		ReasoningEffort: reasoning,
		SystemPrompt:    sys_prompt,
		Prompt:          prompt,
		Tools:           tools,
		ToolHandler: func(name string, args json.RawMessage) any {
			return toolResponse{
				Time: time.Now().String(),
			}
		},
	}
	return session, request
}

func SnapshotResult(results aikit.ProviderState) string {
	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bytes) + ","
}

func VerifyResults(t *testing.T, results string, result aikit.ProviderState) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	testPath := path.Join(cwd, "tests")
	testRunPath := path.Join(testPath, fmt.Sprintf("test_run_%d.json", time.Now().UnixNano()))
	os.MkdirAll(testPath, 0755)
	results_cleaned := fmt.Sprintf("[%s]", strings.TrimRight(results, ","))
	os.WriteFile(testRunPath, []byte(results_cleaned), 0644)
	t.Helper()
	if !result.Success {
		t.Error(result.Error)
	}
	if result.Result.OutputTokens == 0 {
		t.Fatalf("Recieved no output tokens.")
	}
	if result.Result.InputTokens == 0 {
		t.Fatalf("Recieved no input tokens.")
	}
}
