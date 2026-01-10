// Package handler provides HTTP handlers for the API.
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

// ConversationHandler handles conversation endpoints.
type ConversationHandler struct {
	service *service.ConversationService
	logger  *logger.Logger
}

// NewConversationHandler creates a new conversation handler.
func NewConversationHandler(svc *service.ConversationService, log *logger.Logger) *ConversationHandler {
	return &ConversationHandler{
		service: svc,
		logger:  log,
	}
}

// Create handles POST /api/v1/conversations
func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	var req model.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := middleware.ValidateTitle(req.Title); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	conv, err := h.service.Create(ctx, tenantID, userID, &req)
	if err != nil {
		h.logger.Error("failed to create conversation")
		writeError(w, http.StatusInternalServerError, "failed to create conversation")
		return
	}

	writeJSON(w, http.StatusCreated, conv)
}

// List handles GET /api/v1/conversations
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)

	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	resp, err := h.service.List(ctx, tenantID, limit, offset)
	if err != nil {
		h.logger.Error("failed to list conversations")
		writeError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /api/v1/conversations/:id
func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	conversationID := chi.URLParam(r, "id")

	if err := middleware.ValidateConversationID(conversationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	conv, err := h.service.Get(ctx, tenantID, conversationID)
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

// Update handles PUT /api/v1/conversations/:id
func (h *ConversationHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	conversationID := chi.URLParam(r, "id")

	if err := middleware.ValidateConversationID(conversationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req model.UpdateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != "" {
		if err := middleware.ValidateTitle(req.Title); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	conv, err := h.service.Update(ctx, tenantID, conversationID, &req)
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

// Delete handles DELETE /api/v1/conversations/:id
func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	conversationID := chi.URLParam(r, "id")

	if err := middleware.ValidateConversationID(conversationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.Delete(ctx, tenantID, conversationID); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
