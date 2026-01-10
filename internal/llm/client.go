// Package llm provides LLM client interfaces and implementations.
package llm

import (
	"context"
)

// StreamCallback is called for each token during streaming.
type StreamCallback func(token string, index int) error

// CompletionRequest represents a completion request.
type CompletionRequest struct {
	Model       string
	Messages    []ChatMessage
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// ChatMessage represents a chat message for LLM.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse represents a completion response.
type CompletionResponse struct {
	Content    string
	Model      string
	TokensIn   int
	TokensOut  int
	StopReason string
	LatencyMs  int64
}

// Client is the interface for LLM providers.
type Client interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// CompleteStream sends a streaming completion request.
	CompleteStream(ctx context.Context, req *CompletionRequest, callback StreamCallback) (*CompletionResponse, error)

	// Name returns the provider name.
	Name() string

	// Models returns available models.
	Models() []string
}

// Provider is the type of LLM provider.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// NewClient creates a new LLM client based on provider.
func NewClient(provider Provider, apiKey string) (Client, error) {
	switch provider {
	case ProviderAnthropic:
		return NewAnthropicClient(apiKey)
	case ProviderOpenAI:
		return NewOpenAIClient(apiKey)
	default:
		return NewAnthropicClient(apiKey)
	}
}
