package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var openaiTestReasoningEffort = "low"

func TestResponsesOpenAITool(t *testing.T) {
	all := ""

	session := MakeRequest(t, aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")), "gpt-5-nano", &openaiTestReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}

func TestResponsesOpenAIWeb(t *testing.T) {
	all := ""

	session := MakeSearchRequest(t, aikit.OpenAIVerifiedProvider(os.Getenv("OPENAI_KEY")), "gpt-5-nano", &openaiTestReasoningEffort)
	session.Debug = testDebugEnabled
	result := session.Stream(func(result *aikit.Thread) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, session.Provider.Name(), all, *result)
}
