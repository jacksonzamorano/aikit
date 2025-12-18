package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var fireworksReasoningEffort = "low"

func TestFireworks(t *testing.T) {
	all := ""
	session := MakeRequest(aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")), "accounts/fireworks/models/gpt-oss-20b", &fireworksReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) {
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
