package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
)

func BenchmarkClientGet(b *testing.B) {
	responseBody := `{"cluster_name":"test","nodes":{}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 0

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client, err := New(cfg, logger)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.Get(ctx, "/test")
		if err != nil {
			b.Fatalf("Get() error: %v", err)
		}
	}
}

func BenchmarkClientGetWithRetry(b *testing.B) {
	callCount := 0
	responseBody := `{"cluster_name":"test","nodes":{}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Fail first request, succeed on retry
		if callCount%2 == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 1
	cfg.RetryDelay = 0 // No delay for benchmark

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client, err := New(cfg, logger)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.Get(ctx, "/test")
		if err != nil {
			b.Fatalf("Get() error: %v", err)
		}
	}
}
