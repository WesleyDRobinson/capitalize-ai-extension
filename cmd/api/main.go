// Package main is the entry point for the API server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/capitalize-ai/conversational-platform/internal/config"
	"github.com/capitalize-ai/conversational-platform/internal/handler"
	"github.com/capitalize-ai/conversational-platform/internal/llm"
	"github.com/capitalize-ai/conversational-platform/internal/middleware"
	natsclient "github.com/capitalize-ai/conversational-platform/internal/nats"
	"github.com/capitalize-ai/conversational-platform/internal/service"
	"github.com/capitalize-ai/conversational-platform/pkg/logger"
	"github.com/capitalize-ai/conversational-platform/pkg/tracing"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()
	logger.SetGlobal(log)

	log.Info("starting API server")

	// Initialize tracing if enabled
	ctx := context.Background()
	if cfg.TracingEnabled {
		tp, err := tracing.InitTracer(ctx, "conversational-platform", cfg.TracingEndpoint)
		if err != nil {
			log.Warn("failed to initialize tracing")
		} else {
			defer tracing.Shutdown(ctx, tp)
		}
	}

	// Connect to NATS
	natsClient, err := natsclient.Connect(ctx, natsclient.Config{
		URL:      cfg.NATSURL,
		CAFile:   cfg.NATSCAFile,
		CertFile: cfg.NATSCertFile,
		KeyFile:  cfg.NATSKeyFile,
		Token:    cfg.NATSToken,
	}, log)
	if err != nil {
		log.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// Ensure JetStream stream exists
	streamManager := natsclient.NewStreamManager(natsClient)
	if err := streamManager.EnsureStream(ctx); err != nil {
		log.Error("failed to ensure stream", "error", err)
		os.Exit(1)
	}

	// Initialize LLM client
	var llmClient llm.Client
	if cfg.AnthropicAPIKey != "" {
		llmClient, err = llm.NewAnthropicClient(cfg.AnthropicAPIKey)
		if err != nil {
			log.Warn("failed to create Anthropic client, LLM features disabled")
		}
	} else if cfg.OpenAIAPIKey != "" {
		llmClient, err = llm.NewOpenAIClient(cfg.OpenAIAPIKey)
		if err != nil {
			log.Warn("failed to create OpenAI client, LLM features disabled")
		}
	}

	// Initialize services
	conversationSvc := service.NewConversationService(streamManager, log)
	messageSvc := service.NewMessageService(streamManager, conversationSvc, llmClient, log)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler(natsClient)
	conversationHandler := handler.NewConversationHandler(conversationSvc, log)
	messageHandler := handler.NewMessageHandler(messageSvc, conversationSvc, log)
	streamHandler := handler.NewStreamHandler(messageSvc, conversationSvc, log)

	// Create router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(log))
	r.Use(middleware.SecurityHeaders)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "X-Stream-URL", "X-Correlation-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health endpoints (no auth required)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes with authentication
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))
		r.Use(middleware.RateLimit(cfg.RateLimitRequests, cfg.RateLimitWindow))

		// Conversations
		r.Route("/conversations", func(r chi.Router) {
			r.Post("/", conversationHandler.Create)
			r.Get("/", conversationHandler.List)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", conversationHandler.Get)
				r.Put("/", conversationHandler.Update)
				r.Delete("/", conversationHandler.Delete)

				// Messages
				r.Get("/messages", messageHandler.List)
				r.Post("/messages", messageHandler.Send)

				// Streaming
				r.Get("/stream", streamHandler.Stream)
				r.Post("/stream", streamHandler.StreamWithMessage)
			})
		})
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("server listening", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "error", err)
	}

	log.Info("server stopped")
}
