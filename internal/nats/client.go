// Package nats provides NATS JetStream client management.
package nats

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/capitalize-ai/conversational-platform/pkg/logger"
)

// Config holds NATS connection configuration.
type Config struct {
	URL      string
	CAFile   string
	CertFile string
	KeyFile  string
	Token    string
}

// Client wraps NATS connection and JetStream context.
type Client struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	logger *logger.Logger
}

// Connect establishes a connection to NATS server.
func Connect(ctx context.Context, cfg Config, log *logger.Logger) (*Client, error) {
	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(8 * 1024 * 1024),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Warn("NATS disconnected", logger.Global().With().Logger.Sugar().Desugar().Check(0, "").Entry)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("NATS reconnected")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Error("NATS error")
		}),
	}

	// Add TLS configuration if certificates are provided
	if cfg.CAFile != "" && cfg.CertFile != "" && cfg.KeyFile != "" {
		tlsConfig, err := createTLSConfig(cfg.CAFile, cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	// Add token authentication if provided
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Client{
		conn:   nc,
		js:     js,
		logger: log,
	}, nil
}

// JetStream returns the JetStream context.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Conn returns the underlying NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// Close closes the NATS connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// IsConnected returns true if connected to NATS.
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

func createTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert: %w", err)
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
