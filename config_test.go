package aikit

import (
	"testing"
)

func TestUnit_Config_ResolveEndpointWithExplicitEndpoint(t *testing.T) {
	config := ProviderConfig{
		BaseURL:  "https://api.example.com/v1",
		Endpoint: "https://custom.endpoint.com/v2/messages",
	}

	result := config.resolveEndpoint("/v1/messages")

	if result != "https://custom.endpoint.com/v2/messages" {
		t.Errorf("Expected explicit endpoint, got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointWithBaseURL(t *testing.T) {
	config := ProviderConfig{BaseURL: "https://api.example.com/v1"}
	result := config.resolveEndpoint("/v1/messages")

	if result != "https://api.example.com/v1/v1/messages" {
		t.Errorf("Expected %q, got %q", "https://api.example.com/v1/v1/messages", result)
	}
}

func TestUnit_Config_ResolveEndpointTrimsWhitespace(t *testing.T) {
	config := ProviderConfig{Endpoint: "  https://api.example.com/messages  "}
	result := config.resolveEndpoint("/default")

	if result != "https://api.example.com/messages" {
		t.Errorf("Expected trimmed endpoint, got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointTrailingSlash(t *testing.T) {
	config := ProviderConfig{BaseURL: "https://api.example.com/v1/"}
	result := config.resolveEndpoint("/messages")

	if result != "https://api.example.com/v1/messages" {
		t.Errorf("Expected %q, got %q", "https://api.example.com/v1/messages", result)
	}
}

func TestUnit_Config_ResolveEndpointEmptyBaseURL(t *testing.T) {
	config := ProviderConfig{BaseURL: "", Endpoint: ""}
	result := config.resolveEndpoint("/messages")

	if result != "messages" {
		t.Errorf("Expected 'messages', got %q", result)
	}
}

func TestUnit_Config_ResolveEndpointMalformedPanics(t *testing.T) {
	config := ProviderConfig{Endpoint: "://invalid-url"}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for malformed Endpoint URL")
		}
	}()

	config.resolveEndpoint("/messages")
}

func TestUnit_Config_SessionCallsFactory(t *testing.T) {
	called := false
	config := &ProviderConfig{
		Name: "test",
		MakeSessionFunction: func(c *ProviderConfig) *Session {
			called = true
			if c.Name != "test" {
				t.Errorf("Expected config name 'test', got %q", c.Name)
			}
			return &Session{Thread: NewProviderState()}
		},
	}

	session := config.Session()

	if !called {
		t.Error("MakeSessionFunction was not called")
	}
	if session == nil {
		t.Error("Session should not be nil")
	}
}
