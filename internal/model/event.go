package model

import (
	"time"
)

// EventType represents the type of conversation event.
type EventType string

const (
	EventTypeError     EventType = "error"
	EventTypeCancel    EventType = "cancel"
	EventTypeRateLimit EventType = "rate_limit"
	EventTypeTimeout   EventType = "timeout"
)

// ConversationEvent represents an event in a conversation.
type ConversationEvent struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	TenantID       string         `json:"tenant_id"`
	Type           EventType      `json:"type"`
	Reason         string         `json:"reason"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	Sequence       uint64         `json:"sequence,omitempty"`
}
