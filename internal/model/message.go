package model

import (
	"time"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a conversation message.
type Message struct {
	// Identity
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	TenantID       string `json:"tenant_id"`

	// Content
	Role    Role   `json:"role"`
	Content string `json:"content"`

	// LLM Metadata (nullable for non-assistant messages)
	Model      *string `json:"model,omitempty"`
	TokensIn   *int    `json:"tokens_in,omitempty"`
	TokensOut  *int    `json:"tokens_out,omitempty"`
	LatencyMs  *int64  `json:"latency_ms,omitempty"`
	StopReason *string `json:"stop_reason,omitempty"`

	// Timestamps
	CreatedAt     time.Time  `json:"created_at"`
	StreamStarted *time.Time `json:"stream_started,omitempty"`
	StreamEnded   *time.Time `json:"stream_ended,omitempty"`

	// JetStream Metadata (populated on read)
	Sequence uint64 `json:"sequence,omitempty"`
}

// SendMessageRequest is the request to send a new message.
type SendMessageRequest struct {
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
	Stream  bool   `json:"stream"`
}

// SendMessageResponse is the response after sending a message.
type SendMessageResponse struct {
	Message  *Message `json:"message,omitempty"`
	Sequence uint64   `json:"sequence,omitempty"`
}

// ListMessagesResponse is the response for listing messages.
type ListMessagesResponse struct {
	Messages     []Message `json:"messages"`
	HasMore      bool      `json:"has_more"`
	LastSequence uint64    `json:"last_sequence"`
	StreamActive bool      `json:"stream_active"`
}

// TokenEvent represents a streaming token event.
type TokenEvent struct {
	Token string `json:"token"`
	Index int    `json:"index"`
}

// MessageCompleteEvent represents a message completion event.
type MessageCompleteEvent struct {
	Message  Message `json:"message"`
	Sequence uint64  `json:"sequence"`
}

// ErrorEvent represents an error event.
type ErrorEvent struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

// HeartbeatEvent represents a heartbeat event.
type HeartbeatEvent struct {
	Timestamp time.Time `json:"timestamp"`
}
