package llm

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicClient is the Anthropic LLM client.
type AnthropicClient struct {
	client *anthropic.Client
	apiKey string
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(apiKey string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, errors.New("Anthropic API key is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &AnthropicClient{
		client: client,
		apiKey: apiKey,
	}, nil
}

// Name returns the provider name.
func (c *AnthropicClient) Name() string {
	return "anthropic"
}

// Models returns available models.
func (c *AnthropicClient) Models() []string {
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// Complete sends a completion request.
func (c *AnthropicClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Convert messages to Anthropic format
	messages := make([]anthropic.MessageParam, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = anthropic.MessageParam{
			Role: anthropic.F(anthropic.MessageParamRole(msg.Role)),
			Content: anthropic.F([]anthropic.ContentBlockParamUnion{
				anthropic.TextBlockParam{
					Type: anthropic.F(anthropic.TextBlockParamTypeText),
					Text: anthropic.F(msg.Content),
				},
			}),
		}
	}

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(model),
		MaxTokens: anthropic.F(int64(maxTokens)),
		Messages:  anthropic.F(messages),
	})
	if err != nil {
		return nil, err
	}

	// Extract content
	var content string
	for _, block := range resp.Content {
		if block.Type == anthropic.ContentBlockTypeText {
			content += block.Text
		}
	}

	return &CompletionResponse{
		Content:    content,
		Model:      resp.Model,
		TokensIn:   int(resp.Usage.InputTokens),
		TokensOut:  int(resp.Usage.OutputTokens),
		StopReason: string(resp.StopReason),
		LatencyMs:  time.Since(start).Milliseconds(),
	}, nil
}

// CompleteStream sends a streaming completion request.
func (c *AnthropicClient) CompleteStream(ctx context.Context, req *CompletionRequest, callback StreamCallback) (*CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Convert messages to Anthropic format
	messages := make([]anthropic.MessageParam, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = anthropic.MessageParam{
			Role: anthropic.F(anthropic.MessageParamRole(msg.Role)),
			Content: anthropic.F([]anthropic.ContentBlockParamUnion{
				anthropic.TextBlockParam{
					Type: anthropic.F(anthropic.TextBlockParamTypeText),
					Text: anthropic.F(msg.Content),
				},
			}),
		}
	}

	stream := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(model),
		MaxTokens: anthropic.F(int64(maxTokens)),
		Messages:  anthropic.F(messages),
	})

	var content string
	var tokensIn, tokensOut int
	var stopReason string
	index := 0

	message := stream.Current()

	for stream.Next() {
		event := stream.Current()

		switch event.Type {
		case anthropic.MessageStreamEventTypeContentBlockDelta:
			if event.Delta.Type == "text_delta" {
				token := event.Delta.Text
				content += token
				if err := callback(token, index); err != nil {
					return nil, err
				}
				index++
			}
		case anthropic.MessageStreamEventTypeMessageDelta:
			stopReason = string(event.Delta.StopReason)
			tokensOut = int(event.Usage.OutputTokens)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	tokensIn = int(message.Usage.InputTokens)

	return &CompletionResponse{
		Content:    content,
		Model:      model,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		StopReason: stopReason,
		LatencyMs:  time.Since(start).Milliseconds(),
	}, nil
}
