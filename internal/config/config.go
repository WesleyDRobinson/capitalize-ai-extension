// Package config provides environment configuration for the API server.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	// Server settings
	ServerPort         string
	ServerReadTimeout  time.Duration
	ServerWriteTimeout time.Duration

	// NATS settings
	NATSURL      string
	NATSCAFile   string
	NATSCertFile string
	NATSKeyFile  string
	NATSToken    string

	// JWT settings
	JWTSecret     string
	JWTExpiration time.Duration

	// LLM settings
	AnthropicAPIKey string
	OpenAIAPIKey    string
	DefaultLLM      string

	// Rate limiting
	RateLimitRequests int
	RateLimitWindow   time.Duration

	// Logging
	LogLevel string

	// Tracing
	TracingEndpoint string
	TracingEnabled  bool
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		// Server
		ServerPort:         getEnv("PORT", "8080"),
		ServerReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 30*time.Second),
		ServerWriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 120*time.Second),

		// NATS
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		NATSCAFile:   getEnv("NATS_CA_FILE", ""),
		NATSCertFile: getEnv("NATS_CERT_FILE", ""),
		NATSKeyFile:  getEnv("NATS_KEY_FILE", ""),
		NATSToken:    getEnv("NATS_TOKEN", ""),

		// JWT
		JWTSecret:     getEnv("JWT_SECRET", "development-secret-change-in-production"),
		JWTExpiration: getDurationEnv("JWT_EXPIRATION", 15*time.Minute),

		// LLM
		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
		OpenAIAPIKey:    getEnv("OPENAI_API_KEY", ""),
		DefaultLLM:      getEnv("DEFAULT_LLM", "anthropic"),

		// Rate limiting
		RateLimitRequests: getIntEnv("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow:   getDurationEnv("RATE_LIMIT_WINDOW", time.Minute),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Tracing
		TracingEndpoint: getEnv("TRACING_ENDPOINT", "localhost:4318"),
		TracingEnabled:  getBoolEnv("TRACING_ENABLED", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
