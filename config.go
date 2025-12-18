package aikit

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type ProviderConfig struct {
	// BaseURL is a base URL (e.g. "https://api.openai.com/v1") that will be
	// combined with a provider's default endpoint path when Endpoint is empty.
	BaseURL string
	// Endpoint is a full URL to the provider endpoint (e.g. ".../v1/responses").
	// When set, it takes precedence over BaseURL.
	Endpoint string

	APIKey string

	HTTPClient *http.Client
}

func (c ProviderConfig) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &InferenceClient
}

func (c ProviderConfig) resolveEndpoint(defaultPath string) (string, error) {
	raw := strings.TrimSpace(c.Endpoint)
	if raw == "" {
		raw = strings.TrimSpace(c.BaseURL)
		if raw == "" {
			return "", errors.New("missing BaseURL/Endpoint")
		}
		if defaultPath == "" {
			return "", errors.New("missing default path")
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		joined, err := url.JoinPath(parsed.String(), defaultPath)
		if err != nil {
			return "", err
		}
		return joined, nil
	}

	_, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return raw, nil
}
