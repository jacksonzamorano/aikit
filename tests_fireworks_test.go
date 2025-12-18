package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var fireworksProvider = aikit.CompletionsAPI{
	Config: aikit.ProviderConfig{
		BaseURL: "https://api.fireworks.ai/inference",
		APIKey:  os.Getenv("FIREWORKS_KEY"),
	},
}

var fireworksReasoningEffort = "low"

func TestFireworks(t *testing.T) {
	all := ""
	session, request := MakeRequest(fireworksProvider, "accounts/fireworks/models/gpt-oss-20b", &fireworksReasoningEffort)
	result := session.Stream(request, func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
