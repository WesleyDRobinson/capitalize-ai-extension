// Package service provides business logic for the conversation platform.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/capitalize-ai/conversational-platform/internal/model"
	natsclient "github.com/capitalize-ai/conversational-platform/internal/nats"
	"github.com/capitalize-ai/conversational-platform/pkg/logger"
)

// ConversationService handles conversation operations.
type ConversationService struct {
	streamManager *natsclient.StreamManager
	logger        *logger.Logger

	// In-memory storage for conversations (would be replaced with a database in production)
	conversations map[string]*model.Conversation
	mu            sync.RWMutex
}

// NewConversationService creates a new conversation service.
func NewConversationService(streamManager *natsclient.StreamManager, log *logger.Logger) *ConversationService {
	return &ConversationService{
		streamManager: streamManager,
		logger:        log,
		conversations: make(map[string]*model.Conversation),
	}
}

// Create creates a new conversation.
func (s *ConversationService) Create(ctx context.Context, tenantID, userID string, req *model.CreateConversationRequest) (*model.Conversation, error) {
	now := time.Now()

	conv := &model.Conversation{
		ID:        uuid.Must(uuid.NewV7()).String(),
		TenantID:  tenantID,
		UserID:    userID,
		Title:     req.Title,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  req.Metadata,
	}

	s.mu.Lock()
	s.conversations[conv.ID] = conv
	s.mu.Unlock()

	s.logger.Info("conversation created",
		logger.Global().With().Logger.Sugar().Infow("", "conversation_id", conv.ID, "tenant_id", tenantID).Desugar().Check(0, "").Entry,
	)

	return conv, nil
}

// Get retrieves a conversation by ID.
func (s *ConversationService) Get(ctx context.Context, tenantID, conversationID string) (*model.Conversation, error) {
	s.mu.RLock()
	conv, exists := s.conversations[conversationID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("conversation not found")
	}

	if conv.TenantID != tenantID {
		return nil, fmt.Errorf("conversation not found")
	}

	if conv.Deleted {
		return nil, fmt.Errorf("conversation not found")
	}

	return conv, nil
}

// List retrieves conversations for a tenant.
func (s *ConversationService) List(ctx context.Context, tenantID string, limit, offset int) (*model.ListConversationsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var convs []model.Conversation
	for _, conv := range s.conversations {
		if conv.TenantID == tenantID && !conv.Deleted {
			convs = append(convs, *conv)
		}
	}

	// Simple pagination
	total := len(convs)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return &model.ListConversationsResponse{
		Conversations: convs[start:end],
		Total:         total,
		HasMore:       end < total,
	}, nil
}

// Update updates a conversation.
func (s *ConversationService) Update(ctx context.Context, tenantID, conversationID string, req *model.UpdateConversationRequest) (*model.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[conversationID]
	if !exists {
		return nil, fmt.Errorf("conversation not found")
	}

	if conv.TenantID != tenantID {
		return nil, fmt.Errorf("conversation not found")
	}

	if req.Title != "" {
		conv.Title = req.Title
	}
	if req.Metadata != nil {
		conv.Metadata = req.Metadata
	}
	conv.UpdatedAt = time.Now()

	return conv, nil
}

// Delete soft deletes a conversation.
func (s *ConversationService) Delete(ctx context.Context, tenantID, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[conversationID]
	if !exists {
		return fmt.Errorf("conversation not found")
	}

	if conv.TenantID != tenantID {
		return fmt.Errorf("conversation not found")
	}

	conv.Deleted = true
	conv.UpdatedAt = time.Now()

	return nil
}

// UpdateLastMessage updates the last message for a conversation.
func (s *ConversationService) UpdateLastMessage(ctx context.Context, tenantID, conversationID string, msg *model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[conversationID]
	if !exists {
		return fmt.Errorf("conversation not found")
	}

	if conv.TenantID != tenantID {
		return fmt.Errorf("conversation not found")
	}

	conv.LastMessage = msg
	conv.MessageCount++
	conv.UpdatedAt = time.Now()

	return nil
}
