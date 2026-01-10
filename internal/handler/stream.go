package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/capitalize-ai/conversational-platform/internal/middleware"
	"github.com/capitalize-ai/conversational-platform/internal/model"
	"github.com/capitalize-ai/conversational-platform/internal/service"
	"github.com/capitalize-ai/conversational-platform/pkg/logger"
	"github.com/capitalize-ai/conversational-platform/pkg/metrics"
)

// StreamHandler handles SSE streaming endpoints.
type StreamHandler struct {
	messageService      *service.MessageService
	conversationService *service.ConversationService
	logger              *logger.Logger
}

// NewStreamHandler creates a new stream handler.
func NewStreamHandler(
	msgSvc *service.MessageService,
	convSvc *service.ConversationService,
	log *logger.Logger,
) *StreamHandler {
	return &StreamHandler{
		messageService:      msgSvc,
		conversationService: convSvc,
		logger:              log,
	}
}

// Stream handles GET /api/v1/conversations/:id/stream
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	conversationID := chi.URLParam(r, "id")

	if err := middleware.ValidateConversationID(conversationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify conversation exists and belongs to tenant
	if _, err := h.conversationService.Get(ctx, tenantID, conversationID); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Track active connection
	metrics.IncrementSSEConnections()
	defer metrics.DecrementSSEConnections()

	// Create a channel for client disconnection
	done := ctx.Done()

	// Start heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Send initial connection event
	sendSSEEvent(w, flusher, "connected", map[string]string{
		"conversation_id": conversationID,
	})

	for {
		select {
		case <-done:
			// Client disconnected
			h.logger.Info("SSE client disconnected")
			return

		case <-heartbeat.C:
			// Send heartbeat
			sendSSEEvent(w, flusher, "heartbeat", &model.HeartbeatEvent{
				Timestamp: time.Now(),
			})
		}
	}
}

// StreamWithMessage handles POST /api/v1/conversations/:id/stream
// This endpoint accepts a message and streams the response
func (h *StreamHandler) StreamWithMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	conversationID := chi.URLParam(r, "id")

	if err := middleware.ValidateConversationID(conversationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify conversation exists and belongs to tenant
	if _, err := h.conversationService.Get(ctx, tenantID, conversationID); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	var req model.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := middleware.ValidateMessageContent(req.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Track active connection
	metrics.IncrementSSEConnections()
	defer metrics.DecrementSSEConnections()

	// Send user message and stream response
	userMsg, assistantMsg, err := h.messageService.SendWithStream(
		ctx,
		tenantID,
		conversationID,
		&req,
		func(token string, index int) error {
			// Check if client disconnected
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Send token event
			return sendSSEEvent(w, flusher, "token", &model.TokenEvent{
				Token: token,
				Index: index,
			})
		},
	)

	if err != nil {
		// Send error event
		sendSSEEvent(w, flusher, "error", &model.ErrorEvent{
			Code:    "stream_error",
			Message: err.Error(),
		})
		return
	}

	// Send user message confirmation
	sendSSEEvent(w, flusher, "user_message", userMsg)

	// Send message complete event
	if assistantMsg != nil {
		sendSSEEvent(w, flusher, "message_complete", &model.MessageCompleteEvent{
			Message:  *assistantMsg,
			Sequence: assistantMsg.Sequence,
		})
	}

	// Send done event
	sendSSEEvent(w, flusher, "done", map[string]bool{"success": true})
}

func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()

	return nil
}
