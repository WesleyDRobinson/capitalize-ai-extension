package handler

import (
	"net/http"

	natsclient "github.com/capitalize-ai/conversational-platform/internal/nats"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	natsClient *natsclient.Client
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(natsClient *natsclient.Client) *HealthHandler {
	return &HealthHandler{
		natsClient: natsClient,
	}
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Ready handles GET /ready
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Check NATS connection
	if h.natsClient == nil || !h.natsClient.IsConnected() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "NATS not connected",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
