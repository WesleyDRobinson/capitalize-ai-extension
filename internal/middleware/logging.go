package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/capitalize-ai/conversational-platform/pkg/logger"
	"github.com/capitalize-ai/conversational-platform/pkg/metrics"
)

const (
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey ContextKey = "correlation_id"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Logging creates request logging middleware.
func Logging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate or extract correlation ID
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
			}

			// Create response writer wrapper
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Add correlation ID to response headers
			wrapped.Header().Set("X-Correlation-ID", correlationID)

			// Add correlation ID to context
			ctx := r.Context()
			ctx = contextWithCorrelationID(ctx, correlationID)
			r = r.WithContext(ctx)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)
			durationSec := duration.Seconds()

			// Extract tenant and user from context
			tenantID := GetTenantID(r.Context())
			userID := GetUserID(r.Context())

			// Log request
			log.Info("request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Int64("bytes", wrapped.written),
				zap.Duration("duration", duration),
				zap.String("correlation_id", correlationID),
				zap.String("tenant_id", tenantID),
				zap.String("user_id", userID),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)

			// Record metrics
			statusStr := http.StatusText(wrapped.statusCode)
			metrics.RecordRequest(r.Method, r.URL.Path, statusStr, durationSec)
		})
	}
}

func contextWithCorrelationID(ctx interface{ Value(any) any }, correlationID string) interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(any) any
} {
	return &correlationContext{ctx, correlationID}
}

type correlationContext struct {
	parent interface{ Value(any) any }
	id     string
}

func (c *correlationContext) Deadline() (time.Time, bool) {
	if d, ok := c.parent.(interface{ Deadline() (time.Time, bool) }); ok {
		return d.Deadline()
	}
	return time.Time{}, false
}

func (c *correlationContext) Done() <-chan struct{} {
	if d, ok := c.parent.(interface{ Done() <-chan struct{} }); ok {
		return d.Done()
	}
	return nil
}

func (c *correlationContext) Err() error {
	if d, ok := c.parent.(interface{ Err() error }); ok {
		return d.Err()
	}
	return nil
}

func (c *correlationContext) Value(key any) any {
	if key == CorrelationIDKey {
		return c.id
	}
	return c.parent.Value(key)
}

// GetCorrelationID gets correlation ID from context.
func GetCorrelationID(ctx interface{ Value(any) any }) string {
	if v := ctx.Value(CorrelationIDKey); v != nil {
		return v.(string)
	}
	return ""
}
