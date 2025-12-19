package aikit

import (
	"fmt"
	"strings"
)

type AIErrorCategory string

const (
	AIErrorCategoryRateLimit       AIErrorCategory = "rate_limit"
	AIErrorCategoryAuthentication  AIErrorCategory = "authentication"
	AIErrorCategoryStreamingError  AIErrorCategory = "streaming"
	AIErrorCategoryDecodingError   AIErrorCategory = "decoding"
	AIErrorCategoryToolResultError AIErrorCategory = "tool_result_encode"
	AIErrorCategoryHTTPStatus      AIErrorCategory = "http_status"
	AIErrorCategoryUnknown         AIErrorCategory = "unknown"
)

type AIError struct {
	Category AIErrorCategory
	Provider string
	Message  string
}

func (e *AIError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Provider, e.Category, e.Message)
}

func cleanupMessage(message string) string {
	return strings.ReplaceAll(message, "\n", " ")
}

func DecodingError(provider, message string) *AIError {
	return &AIError{
		Category: AIErrorCategoryDecodingError,
		Provider: provider,
		Message:  cleanupMessage(message),
	}
}

func AuthenticationError(provider, message string) *AIError {
	return &AIError{
		Category: AIErrorCategoryAuthentication,
		Provider: provider,
		Message:  cleanupMessage(message),
	}
}

func RateLimitError(provider, message string) *AIError {
	return &AIError{
		Category: AIErrorCategoryRateLimit,
		Provider: provider,
		Message:  cleanupMessage(message),
	}
}

func UnknownError(provider, message string) *AIError {
	return &AIError{
		Category: AIErrorCategoryUnknown,
		Provider: provider,
		Message:  cleanupMessage(message),
	}
}
