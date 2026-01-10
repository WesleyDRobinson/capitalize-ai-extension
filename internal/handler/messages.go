package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/capitalize-ai/conversational-platform/internal/middleware"
	"github.com/capitalize-ai/conversational-platform/internal/model"
	"github.com/capitalize-ai/conversational-platform/internal/service"
	"github.com/capitalize-ai/conversational-platform/pkg/logger"
)

// MessageHandler handles message endpoints.
type MessageHandler struct {
	messageService      *service.MessageService
	conversationService *service.ConversationService
	logger              *logger.Logger
}

// NewMessageHandler creates a new message handler.
func NewMessageHandler(
	msgSvc *service.MessageService,
	convSvc *service.ConversationService,
	log *logger.Logger,
) *MessageHandler {
	return &MessageHandler{
		messageService:      msgSvc,
		conversationService: convSvc,
		logger:              log,
	}
}

// List handles GET /api/v1/conversations/:id/messages
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
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

	// Parse query params
	afterSequence := uint64(0)
	limit := 50

	if seq := r.URL.Query().Get("after_sequence"); seq != "" {
		if parsed, err := strconv.ParseUint(seq, 10, 64); err == nil {
			afterSequence = parsed
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	resp, err := h.messageService.GetMessages(ctx, tenantID, conversationID, afterSequence, limit)
	if err != nil {
		h.logger.Error("failed to get messages")
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Send handles POST /api/v1/conversations/:id/messages
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
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

	if req.Stream {
		// For streaming, return 202 Accepted with stream URL
		w.Header().Set("X-Stream-URL", "/api/v1/conversations/"+conversationID+"/stream")
		w.WriteHeader(http.StatusAccepted)

		// Store the pending request for the stream handler
		// In production, this would use a proper queue or state store
		return
	}

	// Non-streaming response
	userMsg, seq, err := h.messageService.Send(ctx, tenantID, conversationID, &req)
	if err != nil {
		h.logger.Error("failed to send message")
		writeError(w, http.StatusInternalServerError, "failed to send message")
		return
	}

	writeJSON(w, http.StatusCreated, &model.SendMessageResponse{
		Message:  userMsg,
		Sequence: seq,
	})
}
