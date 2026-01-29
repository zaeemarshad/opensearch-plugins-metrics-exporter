package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if client.baseURL != cfg.OpenSearchURL {
		t.Errorf("expected baseURL %s, got %s", cfg.OpenSearchURL, client.baseURL)
	}
	if client.retryCount != cfg.RetryCount {
		t.Errorf("expected retryCount %d, got %d", cfg.RetryCount, client.retryCount)
	}
	if client.retryDelay != cfg.RetryDelay {
		t.Errorf("expected retryDelay %v, got %v", cfg.RetryDelay, client.retryDelay)
	}
}

func TestGet_Success(t *testing.T) {
	expectedBody := `{"cluster_name": "test-cluster"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test/path" {
			t.Errorf("expected path /test/path, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected method GET, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept header application/json, got %s", r.Header.Get("Accept"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	body, err := client.Get(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if string(body) != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, string(body))
	}
}

func TestGet_BasicAuth(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.OpenSearchUsername = "testuser"
	cfg.OpenSearchPassword = "testpass"

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if receivedAuth == "" {
		t.Error("expected Authorization header to be set")
	}
	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Errorf("expected Basic auth, got %s", receivedAuth)
	}
}

func TestGet_ErrorStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 0 // No retries for this test

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), "/")
	if err == nil {
		t.Error("expected error for 500 status code")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got %v", err)
	}
}

func TestGet_Retry(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "service unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 3
	cfg.RetryDelay = 10 * time.Millisecond // Fast retries for testing

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	body, err := client.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
	if string(body) != `{"status": "ok"}` {
		t.Errorf("unexpected body: %s", string(body))
	}
}

func TestGet_RetryExhausted(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "always fails"}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 2
	cfg.RetryDelay = 10 * time.Millisecond

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), "/")
	if err == nil {
		t.Error("expected error after retries exhausted")
	}

	// Initial attempt + 2 retries = 3 total attempts
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("expected error to mention attempts, got %v", err)
	}
}

func TestGet_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.OpenSearchTimeout = 10 * time.Second

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.Get(ctx, "/")
	if err == nil {
		t.Error("expected error from context cancellation")
	}
}

func TestGet_ContextCancellationDuringRetry(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.OpenSearchURL = server.URL
	cfg.RetryCount = 10
	cfg.RetryDelay = 1 * time.Second // Slow retries

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err = client.Get(ctx, "/")
	if err == nil {
		t.Error("expected error from context cancellation")
	}

	// Should have made at least 1 attempt but not all 10
	if attemptCount == 0 {
		t.Error("expected at least 1 attempt")
	}
	if attemptCount > 3 {
		t.Errorf("expected context to cancel before many retries, got %d attempts", attemptCount)
	}
}

func TestClose(t *testing.T) {
	cfg := config.DefaultConfig()

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Should not panic
	client.Close()
	client.Close() // Multiple closes should be safe
}
