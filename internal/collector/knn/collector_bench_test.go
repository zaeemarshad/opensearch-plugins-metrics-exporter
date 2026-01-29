package knn

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// mockClient implements client.HTTPClient for benchmarking.
type mockClient struct {
	response []byte
}

func (m *mockClient) Get(_ context.Context, _ string) ([]byte, error) {
	return m.response, nil
}

func (m *mockClient) Close() {}

func BenchmarkCollectorCollect(b *testing.B) {
	testData, err := os.ReadFile("testdata/stats_response.json")
	if err != nil {
		b.Fatalf("failed to read test data: %v", err)
	}

	mock := &mockClient{response: testData}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := NewCollector(mock, logger)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ch := make(chan prometheus.Metric, 500)
		go func() {
			collector.Collect(ch)
			close(ch)
		}()
		for range ch {
			// Drain the channel
		}
	}
}

func BenchmarkCollectorDescribe(b *testing.B) {
	mock := &mockClient{response: []byte(`{}`)}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := NewCollector(mock, logger)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ch := make(chan *prometheus.Desc, 200)
		go func() {
			collector.Describe(ch)
			close(ch)
		}()
		for range ch {
			// Drain the channel
		}
	}
}

func BenchmarkStatsResponseParsing(b *testing.B) {
	testData, err := os.ReadFile("testdata/stats_response.json")
	if err != nil {
		b.Fatalf("failed to read test data: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var stats StatsResponse
		if err := json.Unmarshal(testData, &stats); err != nil {
			b.Fatalf("failed to parse stats: %v", err)
		}
	}
}
