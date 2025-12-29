package aikit

func GoogleProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                "google",
		BaseURL:             "https://generativelanguage.googleapis.com",
		APIKey:              key,
		MakeSessionFunction: CreateAIStudioSession,
	}
}

func OpenAIProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                "openai",
		BaseURL:             "https://api.openai.com",
		APIKey:              key,
		WebSearchToolName:   "web_search",
		MakeSessionFunction: CreateResponsesSession,
	}
}

func OpenAIVerifiedProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                 "openai",
		BaseURL:              "https://api.openai.com",
		APIKey:               key,
		WebSearchToolName:    "web_search",
		UseThinkingSummaries: true,
		MakeSessionFunction:  CreateResponsesSession,
	}
}

func FireworksProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                "fireworks",
		BaseURL:             "https://api.fireworks.ai/inference",
		APIKey:              key,
		MakeSessionFunction: CreateCompletionsSession,
	}
}
func GroqProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                "groq",
		BaseURL:             "https://api.groq.com/openai",
		APIKey:              key,
		MakeSessionFunction: CreateCompletionsSession,
	}
}
func AnthropicProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:              "anthropic",
		BaseURL:           "https://api.anthropic.com",
		APIKey:            key,
		WebSearchToolName: "web_search_20250305",
		WebFetchToolName:  "web_fetch_20250910",
		BetaFeatures: []string{
			"interleaved-thinking-2025-05-14",
		},
		MaxTokens:           64_000,
		MakeSessionFunction: CreateMessagesSession,
	}
}
func XAIProvider(key string) ProviderConfig {
	return ProviderConfig{
		Name:                "xai",
		BaseURL:             "https://api.x.ai",
		APIKey:              key,
		MakeSessionFunction: CreateCompletionsSession,
	}
}
