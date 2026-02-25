// Package exporter provides the HTTP server that exposes Prometheus metrics.
// It registers collectors and serves metrics on a configurable endpoint,
// along with health and readiness probes for Kubernetes compatibility.
package exporter

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	httpServer *http.Server
	registry   *prometheus.Registry
	logger     *slog.Logger
}

type Config struct {
	Port        int
	MetricsPath string
}

func New(cfg Config, collectors []prometheus.Collector, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	registry := prometheus.NewRegistry()

	for _, c := range collectors {
		if err := registry.Register(c); err != nil {
			return nil, fmt.Errorf("failed to register collector: %w", err)
		}
	}

	mux := http.NewServeMux()

	metricsPath := cfg.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>OpenSearch Plugins Metrics Exporter</title></head>
<body>
<h1>OpenSearch Plugins Metrics Exporter</h1>
<p><a href="%s">Metrics</a></p>
<p><a href="/health">Health</a></p>
<p><a href="/ready">Ready</a></p>
</body>
</html>`, html.EscapeString(metricsPath))
	})

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		registry:   registry,
		logger:     logger,
	}, nil
}

func (s *Server) Start() error {
	s.logger.Info("starting exporter server", "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down exporter server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}
