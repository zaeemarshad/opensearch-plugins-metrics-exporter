package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.OpenSearchURL != "http://localhost:9200" {
		t.Errorf("expected OpenSearchURL http://localhost:9200, got %s", cfg.OpenSearchURL)
	}
	if cfg.OpenSearchTimeout != 10*time.Second {
		t.Errorf("expected OpenSearchTimeout 10s, got %v", cfg.OpenSearchTimeout)
	}
	if cfg.TLSInsecure != false {
		t.Error("expected TLSInsecure false")
	}
	if cfg.RetryCount != 3 {
		t.Errorf("expected RetryCount 3, got %d", cfg.RetryCount)
	}
	if cfg.RetryDelay != 1*time.Second {
		t.Errorf("expected RetryDelay 1s, got %v", cfg.RetryDelay)
	}
	if cfg.ExporterPort != 9206 {
		t.Errorf("expected ExporterPort 9206, got %d", cfg.ExporterPort)
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("expected MetricsPath /metrics, got %s", cfg.MetricsPath)
	}
	if !cfg.EnableKNN {
		t.Error("expected EnableKNN true")
	}
	if !cfg.EnableNeural {
		t.Error("expected EnableNeural true")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore environment
	envVars := []string{
		"OPENSEARCH_URL",
		"OPENSEARCH_USERNAME",
		"OPENSEARCH_PASSWORD",
		"OPENSEARCH_TIMEOUT",
		"OPENSEARCH_TLS_INSECURE",
		"OPENSEARCH_TLS_CA_CERT",
		"OPENSEARCH_TLS_CLIENT_CERT",
		"OPENSEARCH_TLS_CLIENT_KEY",
		"OPENSEARCH_RETRY_COUNT",
		"OPENSEARCH_RETRY_DELAY",
		"EXPORTER_PORT",
		"METRICS_PATH",
		"ENABLE_KNN",
		"ENABLE_NEURAL",
	}

	savedEnv := make(map[string]string)
	for _, key := range envVars {
		savedEnv[key] = os.Getenv(key)
	}
	defer func() {
		for key, val := range savedEnv {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}()

	// Clear all env vars first
	for _, key := range envVars {
		os.Unsetenv(key)
	}

	// Set test values
	os.Setenv("OPENSEARCH_URL", "https://test:9200")
	os.Setenv("OPENSEARCH_USERNAME", "testuser")
	os.Setenv("OPENSEARCH_PASSWORD", "testpass")
	os.Setenv("OPENSEARCH_TIMEOUT", "30s")
	os.Setenv("OPENSEARCH_TLS_INSECURE", "true")
	os.Setenv("OPENSEARCH_TLS_CA_CERT", "/path/to/ca.crt")
	os.Setenv("OPENSEARCH_TLS_CLIENT_CERT", "/path/to/client.crt")
	os.Setenv("OPENSEARCH_TLS_CLIENT_KEY", "/path/to/client.key")
	os.Setenv("OPENSEARCH_RETRY_COUNT", "5")
	os.Setenv("OPENSEARCH_RETRY_DELAY", "2s")
	os.Setenv("EXPORTER_PORT", "9999")
	os.Setenv("METRICS_PATH", "/custom-metrics")
	os.Setenv("ENABLE_KNN", "false")
	os.Setenv("ENABLE_NEURAL", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.OpenSearchURL != "https://test:9200" {
		t.Errorf("expected OpenSearchURL https://test:9200, got %s", cfg.OpenSearchURL)
	}
	if cfg.OpenSearchUsername != "testuser" {
		t.Errorf("expected OpenSearchUsername testuser, got %s", cfg.OpenSearchUsername)
	}
	if cfg.OpenSearchPassword != "testpass" {
		t.Errorf("expected OpenSearchPassword testpass, got %s", cfg.OpenSearchPassword)
	}
	if cfg.OpenSearchTimeout != 30*time.Second {
		t.Errorf("expected OpenSearchTimeout 30s, got %v", cfg.OpenSearchTimeout)
	}
	if !cfg.TLSInsecure {
		t.Error("expected TLSInsecure true")
	}
	if cfg.TLSCACert != "/path/to/ca.crt" {
		t.Errorf("expected TLSCACert /path/to/ca.crt, got %s", cfg.TLSCACert)
	}
	if cfg.TLSClientCert != "/path/to/client.crt" {
		t.Errorf("expected TLSClientCert /path/to/client.crt, got %s", cfg.TLSClientCert)
	}
	if cfg.TLSClientKey != "/path/to/client.key" {
		t.Errorf("expected TLSClientKey /path/to/client.key, got %s", cfg.TLSClientKey)
	}
	if cfg.RetryCount != 5 {
		t.Errorf("expected RetryCount 5, got %d", cfg.RetryCount)
	}
	if cfg.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay 2s, got %v", cfg.RetryDelay)
	}
	if cfg.ExporterPort != 9999 {
		t.Errorf("expected ExporterPort 9999, got %d", cfg.ExporterPort)
	}
	if cfg.MetricsPath != "/custom-metrics" {
		t.Errorf("expected MetricsPath /custom-metrics, got %s", cfg.MetricsPath)
	}
	if cfg.EnableKNN {
		t.Error("expected EnableKNN false")
	}
	if cfg.EnableNeural {
		t.Error("expected EnableNeural false")
	}
}

func TestLoadTLSInsecureVariants(t *testing.T) {
	tests := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false}, // defaults to false when not set
	}

	savedVal := os.Getenv("OPENSEARCH_TLS_INSECURE")
	defer func() {
		if savedVal == "" {
			os.Unsetenv("OPENSEARCH_TLS_INSECURE")
		} else {
			os.Setenv("OPENSEARCH_TLS_INSECURE", savedVal)
		}
	}()

	for _, tt := range tests {
		t.Run("value="+tt.envValue, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("OPENSEARCH_TLS_INSECURE")
			} else {
				os.Setenv("OPENSEARCH_TLS_INSECURE", tt.envValue)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg.TLSInsecure != tt.expected {
				t.Errorf("expected TLSInsecure %v for env value %q, got %v", tt.expected, tt.envValue, cfg.TLSInsecure)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "empty URL",
			modify:  func(c *Config) { c.OpenSearchURL = "" },
			wantErr: true,
			errMsg:  "OpenSearch URL is required",
		},
		{
			name:    "port too low",
			modify:  func(c *Config) { c.ExporterPort = 0 },
			wantErr: true,
			errMsg:  "exporter port must be between 1 and 65535",
		},
		{
			name:    "port too high",
			modify:  func(c *Config) { c.ExporterPort = 70000 },
			wantErr: true,
			errMsg:  "exporter port must be between 1 and 65535",
		},
		{
			name:    "negative retry count",
			modify:  func(c *Config) { c.RetryCount = -1 },
			wantErr: true,
			errMsg:  "retry count must be non-negative",
		},
		{
			name:    "negative retry delay",
			modify:  func(c *Config) { c.RetryDelay = -1 * time.Second },
			wantErr: true,
			errMsg:  "retry delay must be non-negative",
		},
		{
			name:    "client cert without key",
			modify:  func(c *Config) { c.TLSClientCert = "/path/to/cert"; c.TLSClientKey = "" },
			wantErr: true,
			errMsg:  "both TLS client certificate and key must be provided together",
		},
		{
			name:    "client key without cert",
			modify:  func(c *Config) { c.TLSClientCert = ""; c.TLSClientKey = "/path/to/key" },
			wantErr: true,
			errMsg:  "both TLS client certificate and key must be provided together",
		},
		{
			name:    "valid client cert and key",
			modify:  func(c *Config) { c.TLSClientCert = "/path/to/cert"; c.TLSClientKey = "/path/to/key" },
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadInvalidEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		errMust string
	}{
		{
			name:    "invalid OPENSEARCH_TIMEOUT",
			envKey:  "OPENSEARCH_TIMEOUT",
			envVal:  "notaduration",
			errMust: "invalid OPENSEARCH_TIMEOUT",
		},
		{
			name:    "invalid OPENSEARCH_RETRY_COUNT",
			envKey:  "OPENSEARCH_RETRY_COUNT",
			envVal:  "notanint",
			errMust: "invalid OPENSEARCH_RETRY_COUNT",
		},
		{
			name:    "invalid OPENSEARCH_RETRY_DELAY",
			envKey:  "OPENSEARCH_RETRY_DELAY",
			envVal:  "notaduration",
			errMust: "invalid OPENSEARCH_RETRY_DELAY",
		},
		{
			name:    "invalid EXPORTER_PORT",
			envKey:  "EXPORTER_PORT",
			envVal:  "notanint",
			errMust: "invalid EXPORTER_PORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Getenv(tt.envKey)
			defer func() {
				if saved == "" {
					os.Unsetenv(tt.envKey)
				} else {
					os.Setenv(tt.envKey, saved)
				}
			}()

			os.Setenv(tt.envKey, tt.envVal)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for invalid %s=%q, got nil", tt.envKey, tt.envVal)
			}
			if !strings.Contains(err.Error(), tt.errMust) {
				t.Errorf("expected error to contain %q, got %q", tt.errMust, err.Error())
			}
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	t.Run("insecure skip verify", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TLSInsecure = true

		tlsCfg, err := cfg.BuildTLSConfig()
		if err != nil {
			t.Fatalf("BuildTLSConfig() error: %v", err)
		}

		if !tlsCfg.InsecureSkipVerify {
			t.Error("expected InsecureSkipVerify true")
		}
		if tlsCfg.MinVersion != 0x0303 { // TLS 1.2
			t.Errorf("expected MinVersion TLS 1.2 (0x0303), got 0x%04x", tlsCfg.MinVersion)
		}
	})

	t.Run("nonexistent CA cert", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TLSCACert = "/nonexistent/ca.crt"

		_, err := cfg.BuildTLSConfig()
		if err == nil {
			t.Error("expected error for nonexistent CA cert")
		}
	})

	t.Run("nonexistent client cert", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TLSClientCert = "/nonexistent/client.crt"
		cfg.TLSClientKey = "/nonexistent/client.key"

		_, err := cfg.BuildTLSConfig()
		if err == nil {
			t.Error("expected error for nonexistent client cert")
		}
	})
}
