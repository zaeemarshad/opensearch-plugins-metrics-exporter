// Package config provides configuration management for the OpenSearch plugins metrics exporter.
// It supports loading configuration from environment variables and CLI flags,
// with sensible defaults for local development.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	OpenSearchURL      string
	OpenSearchUsername string
	OpenSearchPassword string
	OpenSearchTimeout  time.Duration

	TLSInsecure   bool
	TLSCACert     string
	TLSClientCert string
	TLSClientKey  string

	RetryCount int
	RetryDelay time.Duration

	ExporterPort int
	MetricsPath  string

	// Plugin toggles
	EnableKNN    bool
	EnableNeural bool
}

func DefaultConfig() *Config {
	return &Config{
		OpenSearchURL:     "http://localhost:9200",
		OpenSearchTimeout: 10 * time.Second,
		TLSInsecure:       false,
		RetryCount:        3,
		RetryDelay:        1 * time.Second,
		ExporterPort:      9206,
		MetricsPath:       "/metrics",
		EnableKNN:         true,
		EnableNeural:      true,
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	if url := os.Getenv("OPENSEARCH_URL"); url != "" {
		cfg.OpenSearchURL = url
	}
	if username := os.Getenv("OPENSEARCH_USERNAME"); username != "" {
		cfg.OpenSearchUsername = username
	}
	if password := os.Getenv("OPENSEARCH_PASSWORD"); password != "" {
		cfg.OpenSearchPassword = password
	}
	if timeout := os.Getenv("OPENSEARCH_TIMEOUT"); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid OPENSEARCH_TIMEOUT %q: %w", timeout, err)
		}
		cfg.OpenSearchTimeout = d
	}
	if insecure := os.Getenv("OPENSEARCH_TLS_INSECURE"); insecure != "" {
		cfg.TLSInsecure = insecure == "true" || insecure == "1" || insecure == "yes"
	}
	if caCert := os.Getenv("OPENSEARCH_TLS_CA_CERT"); caCert != "" {
		cfg.TLSCACert = caCert
	}
	if clientCert := os.Getenv("OPENSEARCH_TLS_CLIENT_CERT"); clientCert != "" {
		cfg.TLSClientCert = clientCert
	}
	if clientKey := os.Getenv("OPENSEARCH_TLS_CLIENT_KEY"); clientKey != "" {
		cfg.TLSClientKey = clientKey
	}
	if retryCount := os.Getenv("OPENSEARCH_RETRY_COUNT"); retryCount != "" {
		n, err := strconv.Atoi(retryCount)
		if err != nil {
			return nil, fmt.Errorf("invalid OPENSEARCH_RETRY_COUNT %q: %w", retryCount, err)
		}
		cfg.RetryCount = n
	}
	if retryDelay := os.Getenv("OPENSEARCH_RETRY_DELAY"); retryDelay != "" {
		d, err := time.ParseDuration(retryDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid OPENSEARCH_RETRY_DELAY %q: %w", retryDelay, err)
		}
		cfg.RetryDelay = d
	}
	if port := os.Getenv("EXPORTER_PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid EXPORTER_PORT %q: %w", port, err)
		}
		cfg.ExporterPort = n
	}
	if metricsPath := os.Getenv("METRICS_PATH"); metricsPath != "" {
		cfg.MetricsPath = metricsPath
	}

	// Plugin toggles - only override if explicitly set
	if enableKNN := os.Getenv("ENABLE_KNN"); enableKNN != "" {
		cfg.EnableKNN = enableKNN == "true" || enableKNN == "1" || enableKNN == "yes"
	}
	if enableNeural := os.Getenv("ENABLE_NEURAL"); enableNeural != "" {
		cfg.EnableNeural = enableNeural == "true" || enableNeural == "1" || enableNeural == "yes"
	}

	return cfg, nil
}

func (c *Config) BuildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.TLSInsecure, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}

	if c.TLSCACert != "" {
		caCert, err := os.ReadFile(c.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if c.TLSClientCert != "" && c.TLSClientKey != "" {
		cert, err := tls.LoadX509KeyPair(c.TLSClientCert, c.TLSClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func (c *Config) Validate() error {
	if c.OpenSearchURL == "" {
		return fmt.Errorf("OpenSearch URL is required")
	}

	if c.ExporterPort <= 0 || c.ExporterPort > 65535 {
		return fmt.Errorf("exporter port must be between 1 and 65535")
	}

	if c.RetryCount < 0 {
		return fmt.Errorf("retry count must be non-negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay must be non-negative")
	}

	if (c.TLSClientCert != "" && c.TLSClientKey == "") || (c.TLSClientCert == "" && c.TLSClientKey != "") {
		return fmt.Errorf("both TLS client certificate and key must be provided together")
	}

	return nil
}
