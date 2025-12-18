package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

func TestGROQ(t *testing.T) {
	all := ""
	session := MakeRequest(aikit.GroqProvider(os.Getenv("GROQ_KEY")), "openai/gpt-oss-20b", nil)
	result := session.Stream(func(result *aikit.ProviderState) {
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
