package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var googleTestProvider = aikit.GoogleAPI{
	Config: aikit.ProviderConfig{
		BaseURL: "https://generativelanguage.googleapis.com",
		APIKey:  os.Getenv("GOOGLE_KEY"),
	},
}

var googleTestReasoningEffort = "1024"

func TestGoogle(t *testing.T) {
	all := ""
	session, request := MakeRequest(googleTestProvider, "gemini-2.5-flash-lite", &googleTestReasoningEffort)
	result := session.Stream(request, func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
