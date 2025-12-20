package aikit

import (
	"net/url"
	"strings"
)

type ProviderConfig struct {
	Name string
	// BaseURL is a base URL (e.g. "https://api.openai.com/v1") that will be
	// combined with a provider's default endpoint path when Endpoint is empty.
	BaseURL string
	// Endpoint is a full URL to the provider endpoint (e.g. ".../v1/responses").
	// When set, it takes precedence over BaseURL.
	Endpoint string

	APIKey string

	WebSearchToolName string
	WebFetchToolName  string

	BetaFeatures         []string
	APIVersion           string
	UseThinkingSummaries bool

	MakeSessionFunction func(*ProviderConfig) *Session
}

func (c *ProviderConfig) Session() *Session {
	return c.MakeSessionFunction(c)
}

func (c ProviderConfig) resolveEndpoint(defaultPath string) string {
	raw := strings.TrimSpace(c.Endpoint)
	if raw == "" {
		raw = strings.TrimSpace(c.BaseURL)
		parsed, err := url.Parse(raw)
		if err != nil {
			panic(err)
		}
		joined, err := url.JoinPath(parsed.String(), defaultPath)
		if err != nil {
			panic(err)
		}
		return joined
	}

	_, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return raw
}
