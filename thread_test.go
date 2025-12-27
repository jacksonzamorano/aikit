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

var testDebugEnabled = false

type toolResponse struct {
	Time  string `json:"time"`
	Error string `json:"error,omitempty"`
}

func MakeRequest(t *testing.T, provider aikit.ProviderConfig, modelname string, reasoning *string) *aikit.Session {
	t.Helper()

	session := provider.Session()

	session.Thread.Model = modelname
	session.Thread.System("You are a helpful assistant. You will always request the current time using the get_time tool and use it in your response.")
	session.Thread.Input("What date is exactly 365 days from today, and what day of the week will it be?")
	session.Thread.Tools = map[string]aikit.ToolDefinition{
		"get_time": {
			Description: "Get the current time in ISO 8601 format.",
			Parameters: &aikit.ToolJsonSchema{
				Type:       "object",
				Properties: &map[string]*aikit.ToolJsonSchema{},
			},
		},
	}
	if reasoning != nil {
		session.Thread.ReasoningEffort = *reasoning
	}
	session.Thread.CoalesceTextBlocks = true
	session.Thread.HandleToolFunction = func(name string, args string) string {
		switch name {
		case "get_time":
			return time.Now().Format(time.RFC3339)
		default:
			return fmt.Sprintf("Error: Unknown tool: %s", name)
		}
	}

	return session
}

func MakeSearchRequest(t *testing.T, provider aikit.ProviderConfig, modelname string, reasoning *string) *aikit.Session {
	t.Helper()
	sess := provider.Session()
	sess.Thread.Model = modelname
	sess.Thread.MaxWebSearches = 1
	sess.Thread.CoalesceTextBlocks = true
	sess.Thread.System("You are a helpful assistant. Always check for the most up-to-date information.")
	sess.Thread.Input("What's new in the newest version of React? Keep your answer concise.")
	if reasoning != nil {
		sess.Thread.ReasoningEffort = *reasoning
	}

	return sess
}

func SnapshotResult(results aikit.Thread) string {
	bytes, err := json.MarshalIndent(results.Blocks, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bytes) + ","
}

func VerifyResults(t *testing.T, name string, results string, result aikit.Thread) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	testPath := path.Join(cwd, "tests")
	os.MkdirAll(testPath, 0755)
	results_cleaned := fmt.Sprintf("[%s]", strings.TrimRight(results, ","))
	testRunPath := path.Join(testPath, fmt.Sprintf("run_%s_%d.json", name, time.Now().UnixNano()))
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
	for _, b := range result.Blocks {
		if !b.Complete {
			t.Errorf("Block %s of type %s not marked complete.", b.ID, b.Type)
		}
		if b.ID == "" && b.Type != aikit.InferenceBlockInput && b.Type != aikit.InferenceBlockSystem {
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
