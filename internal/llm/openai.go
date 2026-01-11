package llm

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIClient is the OpenAI LLM client.
type OpenAIClient struct {
	client *openai.Client
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)

	return &OpenAIClient{
		client: client,
	}, nil
}

// Name returns the provider name.
func (c *OpenAIClient) Name() string {
	return "openai"
}

// Models returns available models.
func (c *OpenAIClient) Models() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
	}
}

// Complete sends a completion request.
func (c *OpenAIClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Convert messages to OpenAI format
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: float32(req.Temperature),
	})
	if err != nil {
		return nil, err
	}

	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	stopReason := ""
	if len(resp.Choices) > 0 {
		stopReason = string(resp.Choices[0].FinishReason)
	}

	return &CompletionResponse{
		Content:    content,
		Model:      resp.Model,
		TokensIn:   resp.Usage.PromptTokens,
		TokensOut:  resp.Usage.CompletionTokens,
		StopReason: stopReason,
		LatencyMs:  time.Since(start).Milliseconds(),
	}, nil
}

// CompleteStream sends a streaming completion request.
func (c *OpenAIClient) CompleteStream(ctx context.Context, req *CompletionRequest, callback StreamCallback) (*CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Convert messages to OpenAI format
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: float32(req.Temperature),
		Stream:      true,
	})
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var content string
	var stopReason string
	index := 0

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				content += delta
				if err := callback(delta, index); err != nil {
					return nil, err
				}
				index++
			}

			if response.Choices[0].FinishReason != "" {
				stopReason = string(response.Choices[0].FinishReason)
			}
		}
	}

	// Note: OpenAI streaming doesn't provide token counts
	// We estimate based on content length
	tokensIn := len(content) / 4  // Rough estimate
	tokensOut := len(content) / 4 // Rough estimate

	return &CompletionResponse{
		Content:    content,
		Model:      model,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		StopReason: stopReason,
		LatencyMs:  time.Since(start).Milliseconds(),
	}, nil
}
