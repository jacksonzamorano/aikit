package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var anthropicReasoningEffort = "1024"

func TestAnthropicStream(t *testing.T) {
	all := ""
	session := MakeRequest(aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")), "claude-haiku-4-5-20251001", &anthropicReasoningEffort)
	session.Debug = false
	result := session.Stream(func(result *aikit.ProviderState) {
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
