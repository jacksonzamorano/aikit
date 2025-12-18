package aikit

func GoogleProvider(key string) InferenceProvider {
	return &GoogleAPI{
		Config: ProviderConfig{
			BaseURL: "https://generativelanguage.googleapis.com",
			APIKey:  key,
		},
	}
}

func OpenAIProvider(key string) InferenceProvider {
	return &ResponsesAPI{
		Config: ProviderConfig{
			BaseURL: "https://api.openai.com",
			APIKey:  key,
		},
		GenerateSummary: false,
	}
}

func FireworksProvider(key string) InferenceProvider {
	return &CompletionsAPI{
		Config: ProviderConfig{
			BaseURL: "https://api.fireworks.ai/inference",
			APIKey:  key,
		},
	}
}
func GroqProvider(key string) InferenceProvider {
	return &CompletionsAPI{
		Config: ProviderConfig{
			BaseURL: "https://api.groq.com/openai",
			APIKey:  key,
		},
	}
}
func AnthropicProvider(key string) InferenceProvider {
	return &MessagesAPI{
		Config: ProviderConfig{
			BaseURL: "https://api.anthropic.com",
			APIKey:  key,
		},
	}
}
func XAIProvider(key string) InferenceProvider {
	return &CompletionsAPI{
		Config: ProviderConfig{
			BaseURL: "https://api.x.ai/v1",
			APIKey:  key,
		},
	}
}
