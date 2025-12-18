package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var openaiTestReasoningEffort = "minimal"
var openaiTestProvider = aikit.ResponsesAPI{
	Config: aikit.ProviderConfig{
		BaseURL: "https://api.openai.com",
		APIKey:  os.Getenv("OPENAI_KEY"),
	},
}

func TestOpenAi(t *testing.T) {
	all := ""
	session, request := MakeRequest(openaiTestProvider, "gpt-5-nano", &openaiTestReasoningEffort)
	result := session.Stream(request, func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
