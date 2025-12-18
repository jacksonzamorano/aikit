package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var groqProvider = aikit.CompletionsAPI{
	Config: aikit.ProviderConfig{
		BaseURL: "https://api.groq.com/openai",
		APIKey:  os.Getenv("GROQ_KEY"),
	},
}

func TestGROQ(t *testing.T) {
	all := ""
	session, request := MakeRequest(groqProvider, "openai/gpt-oss-120b", nil)
	result := session.Stream(request, func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
