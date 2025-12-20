package aikit

func GoogleProvider(key string) Gateway {
	return &AIStudioAPI{
		Config: ProviderConfig{
			Name:    "google",
			BaseURL: "https://generativelanguage.googleapis.com",
			APIKey:  key,
		},
	}
}

func OpenAIProvider(key string) Gateway {
	return &ResponsesAPI{
		Config: ProviderConfig{
			Name:              "openai",
			BaseURL:           "https://api.openai.com",
			APIKey:            key,
			WebSearchToolName: "web_search",
		},
		GenerateSummary: false,
	}
}

func OpenAIVerifiedProvider(key string) Gateway {
	return &ResponsesAPI{
		Config: ProviderConfig{
			Name:              "openai",
			BaseURL:           "https://api.openai.com",
			APIKey:            key,
			WebSearchToolName: "web_search",
		},
		GenerateSummary: true,
	}
}

func FireworksProvider(key string) Gateway {
	return &CompletionsAPI{
		Config: ProviderConfig{
			Name:    "fireworks",
			BaseURL: "https://api.fireworks.ai/inference",
			APIKey:  key,
		},
	}
}
func GroqProvider(key string) Gateway {
	return &CompletionsAPI{
		Config: ProviderConfig{
			Name:    "groq",
			BaseURL: "https://api.groq.com/openai",
			APIKey:  key,
		},
	}
}
func AnthropicProvider(key string) Gateway {
	return &MessagesAPI{
		Config: ProviderConfig{
			Name:              "anthropic",
			BaseURL:           "https://api.anthropic.com",
			APIKey:            key,
			WebSearchToolName: "web_search_20250305",
			WebFetchToolName:  "web_fetch_20250910",
		},
		BetaFeatures: []string{
			"interleaved-thinking-2025-05-14",
		},
	}
}
func XAIProvider(key string) Gateway {
	return &CompletionsAPI{
		Config: ProviderConfig{
			Name:    "xai",
			BaseURL: "https://api.x.ai",
			APIKey:  key,
		},
	}
}
