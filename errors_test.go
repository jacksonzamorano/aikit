package aikit

import (
	"testing"
)

func TestUnit_Error_ConstructorCategories(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, string) *AIError
		expected AIErrorCategory
	}{
		{"DecodingError", DecodingError, AIErrorCategoryDecodingError},
		{"AuthenticationError", AuthenticationError, AIErrorCategoryAuthentication},
		{"RateLimitError", RateLimitError, AIErrorCategoryRateLimit},
		{"UnknownError", UnknownError, AIErrorCategoryUnknown},
		{"ConfigurationError", ConfigurationError, AIErrorCategoryConfiguration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("provider", "message")
			if err.Category != tt.expected {
				t.Errorf("got %v, want %v", err.Category, tt.expected)
			}
			if err.Provider != "provider" {
				t.Errorf("got provider %q, want 'provider'", err.Provider)
			}
		})
	}
}
