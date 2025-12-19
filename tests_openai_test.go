package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var openaiTestReasoningEffort = "high"

func TestOpenAI(t *testing.T) {
	all := ""
	session := MakeRequest(aikit.OpenAIProvider(os.Getenv("OPENAI_KEY")), "gpt-5-nano", &openaiTestReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) { })
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}
