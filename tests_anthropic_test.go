package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var anthropicTestProvider = aikit.MessagesAPI{
	Config: aikit.ProviderConfig{
		BaseURL: "https://api.anthropic.com",
		APIKey:  os.Getenv("ANTHROPIC_KEY"),
	},
}

var anthropicReasoningEffort = "1024"

func TestAnthropicStream(t *testing.T) {
	all := ""
	session, request := MakeRequest(anthropicTestProvider, "claude-haiku-4-5-20251001", &anthropicReasoningEffort)
	result := session.Stream(request, func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
