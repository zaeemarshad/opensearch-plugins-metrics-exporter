package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
)

// Client is a concrete implementation of HTTPClient for OpenSearch communication.
// It handles authentication, TLS configuration, and automatic retries with exponential backoff.
type Client struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
	retryCount int
	retryDelay time.Duration
	logger     *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 5,
	}

	tlsConfig, err := cfg.BuildTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}
	transport.TLSClientConfig = tlsConfig

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.OpenSearchTimeout,
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    cfg.OpenSearchURL,
		username:   cfg.OpenSearchUsername,
		password:   cfg.OpenSearchPassword,
		retryCount: cfg.RetryCount,
		retryDelay: cfg.RetryDelay,
		logger:     logger,
	}, nil
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying request",
				"attempt", attempt,
				"max_attempts", c.retryCount,
				"path", path,
			)

			// Exponential backoff: 1x, 2x, 4x, 8x... the base delay
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		body, err := c.doRequest(ctx, url)
		if err == nil {
			return body, nil
		}

		lastErr = err
		c.logger.Warn("request failed",
			"attempt", attempt+1,
			"path", path,
			"error", err,
		)
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryCount+1, lastErr)
}

func (c *Client) doRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
