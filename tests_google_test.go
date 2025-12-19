package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var googleTestReasoningEffort = "1024"

func TestGoogle(t *testing.T) {
	provider := aikit.GoogleProvider(os.Getenv("GOOGLE_KEY"))

	session := MakeRequest(provider, "gemini-3-flash-preview", &googleTestReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) { })
	all := SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
