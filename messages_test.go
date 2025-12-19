package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var anthropicReasoningEffort = "1024"

func TestAnthropicTool(t *testing.T) {
	all := ""

	session := MakeRequest(t, aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")), "claude-haiku-4-5-20251001", &anthropicReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}

func TestAnthropicResearch(t *testing.T) {
	all := ""

	session := MakeSearchRequest(t, aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")), "claude-haiku-4-5-20251001", &anthropicReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}
