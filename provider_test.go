package aikit_test

import (
	"os"
	"testing"

	"github.com/jacksonzamorano/aikit"
)

var testDebugEnabled = false

func TestGROQ(t *testing.T) {
	all := ""
	session := MakeRequest(aikit.GroqProvider(os.Getenv("GROQ_KEY")), "openai/gpt-oss-20b", nil)
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)
	VerifyResults(t, all, *result)
}

var openaiTestReasoningEffort = "high"

func TestOpenAI(t *testing.T) {
	session := MakeRequest(aikit.OpenAIProvider(os.Getenv("OPENAI_KEY")), "gpt-5-nano", &openaiTestReasoningEffort)

	all := ""
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, all, *result)
}

var googleTestReasoningEffort = "1024"

func TestGoogle(t *testing.T) {
	provider := aikit.GoogleProvider(os.Getenv("GOOGLE_KEY"))

	all := ""
	session := MakeRequest(provider, "gemini-3-flash-preview", &googleTestReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, all, *result)
}

var fireworksReasoningEffort = "low"

func TestFireworks(t *testing.T) {
	all := ""

	session := MakeRequest(aikit.FireworksProvider(os.Getenv("FIREWORKS_KEY")), "accounts/fireworks/models/gpt-oss-20b", &fireworksReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, all, *result)
}

var anthropicReasoningEffort = "1024"

func TestAnthropicStream(t *testing.T) {
	all := ""

	session := MakeRequest(aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY")), "claude-haiku-4-5-20251001", &anthropicReasoningEffort)
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, all, *result)
}

func TestXAI(t *testing.T) {
	all := ""

	session := MakeRequest(aikit.XAIProvider(os.Getenv("XAI_KEY")), "grok-4-1-fast-reasoning", nil)
	result := session.Stream(func(result *aikit.ProviderState) {
		all += SnapshotResult(*result)
	})
	all += SnapshotResult(*result)

	VerifyResults(t, all, *result)
}
