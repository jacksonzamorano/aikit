package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)


var googleTestReasoningEffort = "1024"

func TestGoogle(t *testing.T) {
	provider := aikit.GoogleProvider(os.Getenv("GOOGLE_KEY"))

	all := ""
	session := MakeRequest(t, provider, "gemini-3-flash-preview", &googleTestReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}
