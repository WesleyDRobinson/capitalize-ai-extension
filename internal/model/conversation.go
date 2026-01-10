// Package model defines data structures for the conversation platform.
package model

import (
	"time"
)

// Conversation represents a conversation thread.
type Conversation struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	UserID       string            `json:"user_id"`
	Title        string            `json:"title"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	MessageCount int               `json:"message_count,omitempty"`
	LastMessage  *Message          `json:"last_message,omitempty"`
	Deleted      bool              `json:"deleted,omitempty"`
}

// CreateConversationRequest is the request to create a new conversation.
type CreateConversationRequest struct {
	Title    string            `json:"title"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UpdateConversationRequest is the request to update a conversation.
type UpdateConversationRequest struct {
	Title    string            `json:"title,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ListConversationsResponse is the response for listing conversations.
type ListConversationsResponse struct {
	Conversations []Conversation `json:"conversations"`
	Total         int            `json:"total"`
	HasMore       bool           `json:"has_more"`
	NextCursor    string         `json:"next_cursor,omitempty"`
}
