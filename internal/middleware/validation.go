package middleware

import (
	"errors"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ValidateMessageContent validates message content.
func ValidateMessageContent(content string) error {
	if len(content) == 0 {
		return errors.New("content cannot be empty")
	}
	if len(content) > 100000 { // ~100KB limit
		return errors.New("content exceeds maximum length")
	}
	if !utf8.ValidString(content) {
		return errors.New("content must be valid UTF-8")
	}
	return nil
}

// ValidateConversationID validates a conversation ID.
func ValidateConversationID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return errors.New("invalid conversation ID format")
	}
	return nil
}

// ValidateMessageID validates a message ID.
func ValidateMessageID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return errors.New("invalid message ID format")
	}
	return nil
}

// ValidateTenantID validates a tenant ID.
func ValidateTenantID(id string) error {
	if len(id) == 0 {
		return errors.New("tenant ID cannot be empty")
	}
	if len(id) > 64 {
		return errors.New("tenant ID exceeds maximum length")
	}
	return nil
}

// ValidateTitle validates a conversation title.
func ValidateTitle(title string) error {
	if len(title) > 256 {
		return errors.New("title exceeds maximum length")
	}
	if !utf8.ValidString(title) {
		return errors.New("title must be valid UTF-8")
	}
	return nil
}
