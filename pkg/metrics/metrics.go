// Package metrics provides Prometheus metrics instrumentation.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestDuration tracks HTTP request duration.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	// RequestsTotal tracks total HTTP requests.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// LLMStreamDuration tracks LLM streaming response duration.
	LLMStreamDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_stream_duration_seconds",
			Help:    "LLM streaming response duration",
			Buckets: []float64{1, 2, 5, 10, 20, 30, 45, 60, 90, 120},
		},
		[]string{"model", "status"},
	)

	// LLMTokensTotal tracks total LLM tokens processed.
	LLMTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_tokens_total",
			Help: "Total LLM tokens processed",
		},
		[]string{"model", "direction"},
	)

	// SSEConnectionsActive tracks active SSE connections.
	SSEConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sse_connections_active",
			Help: "Number of active SSE connections",
		},
	)

	// NATSStreamMessages tracks messages in NATS stream.
	NATSStreamMessages = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nats_stream_messages",
			Help: "Number of messages in NATS stream",
		},
		[]string{"stream"},
	)

	// NATSStreamBytes tracks bytes in NATS stream.
	NATSStreamBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nats_stream_bytes",
			Help: "Bytes in NATS stream",
		},
		[]string{"stream"},
	)

	// NATSConsumerPending tracks pending messages for consumers.
	NATSConsumerPending = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nats_consumer_pending",
			Help: "Pending messages for NATS consumer",
		},
		[]string{"stream", "consumer"},
	)

	// ConversationsTotal tracks total conversations created.
	ConversationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conversations_total",
			Help: "Total conversations created",
		},
		[]string{"tenant_id"},
	)

	// MessagesTotal tracks total messages sent.
	MessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_total",
			Help: "Total messages sent",
		},
		[]string{"tenant_id", "role"},
	)
)

// RecordRequest records metrics for an HTTP request.
func RecordRequest(method, path, status string, duration float64) {
	RequestDuration.WithLabelValues(method, path, status).Observe(duration)
	RequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordLLMStream records metrics for an LLM streaming response.
func RecordLLMStream(model, status string, duration float64, tokensIn, tokensOut int) {
	LLMStreamDuration.WithLabelValues(model, status).Observe(duration)
	LLMTokensTotal.WithLabelValues(model, "in").Add(float64(tokensIn))
	LLMTokensTotal.WithLabelValues(model, "out").Add(float64(tokensOut))
}

// IncrementSSEConnections increments the active SSE connection count.
func IncrementSSEConnections() {
	SSEConnectionsActive.Inc()
}

// DecrementSSEConnections decrements the active SSE connection count.
func DecrementSSEConnections() {
	SSEConnectionsActive.Dec()
}
