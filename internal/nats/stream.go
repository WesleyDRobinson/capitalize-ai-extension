package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/capitalize-ai/conversational-platform/internal/model"
)

const (
	// StreamName is the name of the conversations stream.
	StreamName = "CONVERSATIONS"

	// SubjectPrefix is the prefix for all conversation subjects.
	SubjectPrefix = "conv"
)

// StreamManager handles JetStream stream operations.
type StreamManager struct {
	client *Client
}

// NewStreamManager creates a new stream manager.
func NewStreamManager(client *Client) *StreamManager {
	return &StreamManager{client: client}
}

// EnsureStream ensures the conversations stream exists with proper configuration.
func (m *StreamManager) EnsureStream(ctx context.Context) error {
	js := m.client.JetStream()

	// Check if stream exists
	_, err := js.Stream(ctx, StreamName)
	if err == nil {
		return nil // Stream already exists
	}

	// Create stream
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        StreamName,
		Subjects:    []string{fmt.Sprintf("%s.>", SubjectPrefix)},
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      365 * 24 * time.Hour, // 1 year
		MaxBytes:    100 * 1024 * 1024 * 1024, // 100GB
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		Compression: jetstream.S2Compression,
		DenyDelete:  true,
		DenyPurge:   true,
		Description: "All conversation messages and events",
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	return nil
}

// MessageSubject returns the subject for a message.
func MessageSubject(tenantID, conversationID string, role model.Role) string {
	return fmt.Sprintf("%s.%s.%s.msg.%s", SubjectPrefix, tenantID, conversationID, role)
}

// EventSubject returns the subject for an event.
func EventSubject(tenantID, conversationID string, eventType model.EventType) string {
	return fmt.Sprintf("%s.%s.%s.event.%s", SubjectPrefix, tenantID, conversationID, eventType)
}

// ConversationFilter returns the filter subject for all messages in a conversation.
func ConversationFilter(tenantID, conversationID string) string {
	return fmt.Sprintf("%s.%s.%s.>", SubjectPrefix, tenantID, conversationID)
}

// PublishMessage publishes a message to JetStream.
func (m *StreamManager) PublishMessage(ctx context.Context, msg *model.Message) (uint64, error) {
	subject := MessageSubject(msg.TenantID, msg.ConversationID, msg.Role)

	data, err := json.Marshal(msg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal message: %w", err)
	}

	ack, err := m.client.JetStream().Publish(ctx, subject, data)
	if err != nil {
		return 0, fmt.Errorf("failed to publish message: %w", err)
	}

	return ack.Sequence, nil
}

// PublishEvent publishes an event to JetStream.
func (m *StreamManager) PublishEvent(ctx context.Context, event *model.ConversationEvent) (uint64, error) {
	subject := EventSubject(event.TenantID, event.ConversationID, event.Type)

	data, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal event: %w", err)
	}

	ack, err := m.client.JetStream().Publish(ctx, subject, data)
	if err != nil {
		return 0, fmt.Errorf("failed to publish event: %w", err)
	}

	return ack.Sequence, nil
}

// GetMessages retrieves messages from a conversation starting after a sequence.
func (m *StreamManager) GetMessages(ctx context.Context, tenantID, conversationID string, afterSequence uint64, limit int) ([]model.Message, uint64, bool, error) {
	js := m.client.JetStream()

	// Create ephemeral consumer
	filterSubject := fmt.Sprintf("%s.%s.%s.msg.>", SubjectPrefix, tenantID, conversationID)

	consumerConfig := jetstream.ConsumerConfig{
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckNonePolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	if afterSequence > 0 {
		consumerConfig.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		consumerConfig.OptStartSeq = afterSequence + 1
	}

	consumer, err := js.CreateConsumer(ctx, StreamName, consumerConfig)
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to create consumer: %w", err)
	}

	var messages []model.Message
	var lastSequence uint64

	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch, err := consumer.Fetch(limit, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to fetch messages: %w", err)
	}

	for msg := range batch.Messages() {
		select {
		case <-fetchCtx.Done():
			break
		default:
		}

		var message model.Message
		if err := json.Unmarshal(msg.Data(), &message); err != nil {
			continue
		}

		meta, err := msg.Metadata()
		if err == nil {
			message.Sequence = meta.Sequence.Stream
			lastSequence = meta.Sequence.Stream
		}

		messages = append(messages, message)
	}

	if batch.Error() != nil && batch.Error() != context.DeadlineExceeded {
		return nil, 0, false, fmt.Errorf("batch error: %w", batch.Error())
	}

	hasMore := len(messages) == limit

	return messages, lastSequence, hasMore, nil
}
