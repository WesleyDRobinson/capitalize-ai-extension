package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/capitalize-ai/conversational-platform/internal/llm"
	"github.com/capitalize-ai/conversational-platform/internal/model"
	natsclient "github.com/capitalize-ai/conversational-platform/internal/nats"
	"github.com/capitalize-ai/conversational-platform/pkg/logger"
	"github.com/capitalize-ai/conversational-platform/pkg/metrics"
)

// MessageService handles message operations.
type MessageService struct {
	streamManager       *natsclient.StreamManager
	conversationService *ConversationService
	llmClient           llm.Client
	logger              *logger.Logger
}

// NewMessageService creates a new message service.
func NewMessageService(
	streamManager *natsclient.StreamManager,
	conversationService *ConversationService,
	llmClient llm.Client,
	log *logger.Logger,
) *MessageService {
	return &MessageService{
		streamManager:       streamManager,
		conversationService: conversationService,
		llmClient:           llmClient,
		logger:              log,
	}
}

// TokenCallback is called for each token during streaming.
type TokenCallback func(token string, index int) error

// Send sends a user message and generates an AI response.
func (s *MessageService) Send(ctx context.Context, tenantID, conversationID string, req *model.SendMessageRequest) (*model.Message, uint64, error) {
	now := time.Now()

	// Create user message
	userMsg := &model.Message{
		ID:             uuid.Must(uuid.NewV7()).String(),
		ConversationID: conversationID,
		TenantID:       tenantID,
		Role:           model.RoleUser,
		Content:        req.Content,
		CreatedAt:      now,
	}

	// Publish user message
	seq, err := s.streamManager.PublishMessage(ctx, userMsg)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to publish user message: %w", err)
	}
	userMsg.Sequence = seq

	// Update conversation
	s.conversationService.UpdateLastMessage(ctx, tenantID, conversationID, userMsg)

	// Track metrics
	metrics.MessagesTotal.WithLabelValues(tenantID, string(model.RoleUser)).Inc()

	return userMsg, seq, nil
}

// SendWithStream sends a user message and streams the AI response.
func (s *MessageService) SendWithStream(
	ctx context.Context,
	tenantID, conversationID string,
	req *model.SendMessageRequest,
	onToken TokenCallback,
) (*model.Message, *model.Message, error) {
	// Send user message
	userMsg, _, err := s.Send(ctx, tenantID, conversationID, req)
	if err != nil {
		return nil, nil, err
	}

	// Get conversation history for context
	messages, _, _, err := s.streamManager.GetMessages(ctx, tenantID, conversationID, 0, 50)
	if err != nil {
		return userMsg, nil, fmt.Errorf("failed to get message history: %w", err)
	}

	// Convert to LLM format
	chatMessages := make([]llm.ChatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = llm.ChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Stream from LLM
	streamStart := time.Now()
	modelName := req.Model
	if modelName == "" {
		modelName = "claude-3-5-sonnet-20241022"
	}

	resp, err := s.llmClient.CompleteStream(ctx, &llm.CompletionRequest{
		Model:     modelName,
		Messages:  chatMessages,
		MaxTokens: 4096,
		Stream:    true,
	}, func(token string, index int) error {
		return onToken(token, index)
	})
	if err != nil {
		// Publish error event
		s.streamManager.PublishEvent(ctx, &model.ConversationEvent{
			ID:             uuid.Must(uuid.NewV7()).String(),
			ConversationID: conversationID,
			TenantID:       tenantID,
			Type:           model.EventTypeError,
			Reason:         err.Error(),
			CreatedAt:      time.Now(),
		})
		return userMsg, nil, fmt.Errorf("LLM stream failed: %w", err)
	}

	streamEnd := time.Now()

	// Create assistant message
	assistantMsg := &model.Message{
		ID:             uuid.Must(uuid.NewV7()).String(),
		ConversationID: conversationID,
		TenantID:       tenantID,
		Role:           model.RoleAssistant,
		Content:        resp.Content,
		Model:          &resp.Model,
		TokensIn:       &resp.TokensIn,
		TokensOut:      &resp.TokensOut,
		LatencyMs:      &resp.LatencyMs,
		StopReason:     &resp.StopReason,
		CreatedAt:      time.Now(),
		StreamStarted:  &streamStart,
		StreamEnded:    &streamEnd,
	}

	// Publish assistant message
	seq, err := s.streamManager.PublishMessage(ctx, assistantMsg)
	if err != nil {
		return userMsg, nil, fmt.Errorf("failed to publish assistant message: %w", err)
	}
	assistantMsg.Sequence = seq

	// Update conversation
	s.conversationService.UpdateLastMessage(ctx, tenantID, conversationID, assistantMsg)

	// Track metrics
	metrics.MessagesTotal.WithLabelValues(tenantID, string(model.RoleAssistant)).Inc()
	metrics.RecordLLMStream(resp.Model, "success", resp.LatencyMs/1000.0, resp.TokensIn, resp.TokensOut)

	return userMsg, assistantMsg, nil
}

// GetMessages retrieves messages for a conversation.
func (s *MessageService) GetMessages(ctx context.Context, tenantID, conversationID string, afterSequence uint64, limit int) (*model.ListMessagesResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	messages, lastSeq, hasMore, err := s.streamManager.GetMessages(ctx, tenantID, conversationID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return &model.ListMessagesResponse{
		Messages:     messages,
		HasMore:      hasMore,
		LastSequence: lastSeq,
		StreamActive: false,
	}, nil
}
