package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

func TestGROQ(t *testing.T) {
	all := ""

	session := MakeRequest(t, aikit.GroqProvider(os.Getenv("GROQ_KEY")), "openai/gpt-oss-20b", nil)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}


var fireworksReasoningEffort = "low"

func TestFireworks(t *testing.T) {
	all := ""

	session := MakeRequest(t, aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")), "accounts/fireworks/models/gpt-oss-20b", &fireworksReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}


func TestXAI(t *testing.T) {
	all := ""

	session := MakeRequest(t, aikit.XAIProvider(os.Getenv("XAI_KEY")), "grok-4-1-fast-reasoning-latest", nil)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}
