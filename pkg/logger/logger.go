// Package logger provides structured logging utilities.
package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger.
type Logger struct {
	*zap.Logger
}

// New creates a new structured logger.
func New(level string) (*Logger, error) {
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(parseLevel(level)),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

// NewDevelopment creates a development logger with pretty output.
func NewDevelopment() (*Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

// With creates a child logger with additional fields.
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...)}
}

// WithContext creates a child logger with request context fields.
func (l *Logger) WithContext(correlationID, tenantID, userID string) *Logger {
	return l.With(
		zap.String("correlation_id", correlationID),
		zap.String("tenant_id", tenantID),
		zap.String("user_id", userID),
	)
}

func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Global logger instance for convenience.
var global *Logger

func init() {
	if os.Getenv("ENV") == "development" {
		global, _ = NewDevelopment()
	} else {
		global, _ = New("info")
	}
}

// Global returns the global logger instance.
func Global() *Logger {
	return global
}

// SetGlobal sets the global logger instance.
func SetGlobal(l *Logger) {
	global = l
}
