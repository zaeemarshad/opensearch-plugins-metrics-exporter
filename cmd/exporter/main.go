package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/collector/knn"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/collector/neural"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/exporter"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type ctxKey string

const cmdKey ctxKey = "cmd"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "opensearch-plugins-metrics-exporter",
		Short: "Prometheus exporter for OpenSearch plugin metrics",
		Long: `OpenSearch Plugins Metrics Exporter exposes metrics from OpenSearch plugins
in Prometheus format.

Currently supported plugins:
  - k-NN (vector search) - /_plugins/_knn/stats
  - Neural Search (semantic/hybrid search) - /_plugins/_neural/stats
    Note: Neural Search stats must be enabled via cluster setting:
    PUT /_cluster/settings {"persistent":{"plugins.neural_search.stats_enabled":true}}

Environment variables:
  OPENSEARCH_URL          OpenSearch URL (default: http://localhost:9200)
  OPENSEARCH_USERNAME     Basic auth username
  OPENSEARCH_PASSWORD     Basic auth password
  OPENSEARCH_TLS_INSECURE Skip TLS verification
  OPENSEARCH_TLS_CA_CERT  Path to CA certificate
  OPENSEARCH_TLS_CLIENT_CERT  Path to client certificate
  OPENSEARCH_TLS_CLIENT_KEY   Path to client key
  OPENSEARCH_TIMEOUT      Request timeout (default: 10s)
  OPENSEARCH_RETRY_COUNT  Number of retries (default: 3)
  OPENSEARCH_RETRY_DELAY  Delay between retries (default: 1s)
  EXPORTER_PORT           Port to expose metrics (default: 9206)
  METRICS_PATH            Metrics endpoint path (default: /metrics)
  ENABLE_KNN              Enable k-NN plugin metrics (default: true)
  ENABLE_NEURAL           Enable Neural Search plugin metrics (default: true)`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.WithValue(cmd.Context(), cmdKey, cmd)
			return run(ctx)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfgFile, "config", "", "config file path")
	flags.String("url", "http://localhost:9200", "OpenSearch URL")
	flags.String("username", "", "OpenSearch username")
	flags.String("password", "", "OpenSearch password")
	flags.Bool("tls-insecure", false, "Skip TLS certificate verification")
	flags.String("tls-ca-cert", "", "Path to CA certificate")
	flags.String("tls-client-cert", "", "Path to client certificate")
	flags.String("tls-client-key", "", "Path to client key")
	flags.Duration("timeout", 10*time.Second, "Request timeout")
	flags.Int("retry-count", 3, "Number of retries for failed requests")
	flags.Duration("retry-delay", 1*time.Second, "Delay between retries")
	flags.Int("port", 9206, "Port to expose metrics")
	flags.String("metrics-path", "/metrics", "Path to expose metrics")
	flags.String("log-level", "info", "Log level (debug, info, warn, error)")
	flags.String("log-format", "text", "Log format (text, json)")
	flags.Bool("enable-knn", true, "Enable k-NN plugin metrics collection")
	flags.Bool("enable-neural", true, "Enable Neural Search plugin metrics collection")

	_ = viper.BindPFlag("opensearch_url", flags.Lookup("url"))
	_ = viper.BindPFlag("opensearch_username", flags.Lookup("username"))
	_ = viper.BindPFlag("opensearch_password", flags.Lookup("password"))
	_ = viper.BindPFlag("tls_insecure", flags.Lookup("tls-insecure"))
	_ = viper.BindPFlag("tls_ca_cert", flags.Lookup("tls-ca-cert"))
	_ = viper.BindPFlag("tls_client_cert", flags.Lookup("tls-client-cert"))
	_ = viper.BindPFlag("tls_client_key", flags.Lookup("tls-client-key"))
	_ = viper.BindPFlag("opensearch_timeout", flags.Lookup("timeout"))
	_ = viper.BindPFlag("retry_count", flags.Lookup("retry-count"))
	_ = viper.BindPFlag("retry_delay", flags.Lookup("retry-delay"))
	_ = viper.BindPFlag("exporter_port", flags.Lookup("port"))
	_ = viper.BindPFlag("metrics_path", flags.Lookup("metrics-path"))
	_ = viper.BindPFlag("enable_knn", flags.Lookup("enable-knn"))
	_ = viper.BindPFlag("enable_neural", flags.Lookup("enable-neural"))

	return cmd
}

func run(ctx context.Context) error {
	logLevel := viper.GetString("log-level")
	logFormat := viper.GetString("log-format")
	logger := setupLogger(logLevel, logFormat)

	logger.Info("starting OpenSearch plugins metrics exporter",
		"version", version,
		"commit", commit,
		"built", date,
	)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// CLI flags take precedence over environment variables
	if cmd, ok := ctx.Value(cmdKey).(*cobra.Command); ok && cmd != nil {
		if cmd.Flags().Changed("url") {
			cfg.OpenSearchURL = viper.GetString("opensearch_url")
		}
		if cmd.Flags().Changed("username") {
			cfg.OpenSearchUsername = viper.GetString("opensearch_username")
		}
		if cmd.Flags().Changed("password") {
			cfg.OpenSearchPassword = viper.GetString("opensearch_password")
		}
		if cmd.Flags().Changed("tls-insecure") {
			cfg.TLSInsecure = viper.GetBool("tls_insecure")
		}
		if cmd.Flags().Changed("tls-ca-cert") {
			cfg.TLSCACert = viper.GetString("tls_ca_cert")
		}
		if cmd.Flags().Changed("tls-client-cert") {
			cfg.TLSClientCert = viper.GetString("tls_client_cert")
		}
		if cmd.Flags().Changed("tls-client-key") {
			cfg.TLSClientKey = viper.GetString("tls_client_key")
		}
		if cmd.Flags().Changed("timeout") {
			cfg.OpenSearchTimeout = viper.GetDuration("opensearch_timeout")
		}
		if cmd.Flags().Changed("retry-count") {
			cfg.RetryCount = viper.GetInt("retry_count")
		}
		if cmd.Flags().Changed("retry-delay") {
			cfg.RetryDelay = viper.GetDuration("retry_delay")
		}
		if cmd.Flags().Changed("port") {
			cfg.ExporterPort = viper.GetInt("exporter_port")
		}
		if cmd.Flags().Changed("metrics-path") {
			cfg.MetricsPath = viper.GetString("metrics_path")
		}
		if cmd.Flags().Changed("enable-knn") {
			cfg.EnableKNN = viper.GetBool("enable_knn")
		}
		if cmd.Flags().Changed("enable-neural") {
			cfg.EnableNeural = viper.GetBool("enable_neural")
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger.Info("configuration loaded",
		"opensearch_url", cfg.OpenSearchURL,
		"exporter_port", cfg.ExporterPort,
		"metrics_path", cfg.MetricsPath,
		"tls_insecure", cfg.TLSInsecure,
		"retry_count", cfg.RetryCount,
		"enable_knn", cfg.EnableKNN,
		"enable_neural", cfg.EnableNeural,
	)

	osClient, err := client.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create OpenSearch client: %w", err)
	}
	defer osClient.Close()

	var collectors []prometheus.Collector
	if cfg.EnableKNN {
		logger.Info("enabling k-NN plugin collector")
		collectors = append(collectors, knn.NewCollector(osClient, logger))
	}
	if cfg.EnableNeural {
		logger.Info("enabling Neural Search plugin collector")
		collectors = append(collectors, neural.NewCollector(osClient, logger))
	}

	if len(collectors) == 0 {
		return fmt.Errorf("no plugin collectors enabled; enable at least one plugin")
	}

	exporterCfg := exporter.Config{
		Port:        cfg.ExporterPort,
		MetricsPath: cfg.MetricsPath,
	}

	server, err := exporter.New(exporterCfg, collectors, logger)
	if err != nil {
		return fmt.Errorf("failed to create exporter server: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		logger.Info("received shutdown signal", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
}

func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
