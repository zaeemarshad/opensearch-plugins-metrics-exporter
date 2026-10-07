package neural

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/testutil"
)

func getMetricValue(m prometheus.Metric) float64 {
	pb := &dto.Metric{}
	if err := m.Write(pb); err != nil {
		return 0
	}
	if pb.Gauge != nil {
		return pb.Gauge.GetValue()
	}
	if pb.Counter != nil {
		return pb.Counter.GetValue()
	}
	return 0
}

func loadTestData(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/stats_response.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}
	return data
}

var testLocalNodeResponse = []byte(`{"nodes":{"` + testutil.LocalNodeID + `":{"name":"test-node"}}}`)

// statsHandler serves the local node lookup and the neural stats for that node only.
func statsHandler(t *testing.T, stats []byte) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/_nodes/_local":
			if r.URL.Query().Get("filter_path") != "nodes.*.name" {
				t.Errorf("unexpected filter_path: %s", r.URL.RawQuery)
			}
			w.Write(testLocalNodeResponse)
		case "/_plugins/_neural/" + testutil.LocalNodeID + "/stats":
			w.Write(stats)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestStatsResponseParsing(t *testing.T) {
	data := loadTestData(t)

	var stats StatsResponse
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats response: %v", err)
	}

	// Verify cluster info
	if stats.ClusterName != "test-cluster" {
		t.Errorf("expected cluster_name test-cluster, got %s", stats.ClusterName)
	}
	if stats.Info.ClusterVersion != "3.4.0" {
		t.Errorf("expected cluster_version 3.4.0, got %s", stats.Info.ClusterVersion)
	}

	// Verify nodes info
	if stats.Nodes.Total != 1 {
		t.Errorf("expected nodes.total 1, got %d", stats.Nodes.Total)
	}
	if stats.Nodes.Successful != 1 {
		t.Errorf("expected nodes.successful 1, got %d", stats.Nodes.Successful)
	}
	if stats.Nodes.Failed != 0 {
		t.Errorf("expected nodes.failed 0, got %d", stats.Nodes.Failed)
	}

	// Verify info processor counts
	if stats.Info.Processors.Search.Hybrid.NormalizationProcessors != 10 {
		t.Errorf("expected normalization_processors 10, got %d", stats.Info.Processors.Search.Hybrid.NormalizationProcessors)
	}
	if stats.Info.Processors.Ingest.TextEmbeddingProcessorsInPipelines != 10 {
		t.Errorf("expected text_embedding_processors_in_pipelines 10, got %d", stats.Info.Processors.Ingest.TextEmbeddingProcessorsInPipelines)
	}

	// Verify node stats
	if len(stats.NodeStats) != 1 {
		t.Fatalf("expected 1 node, got %d", len(stats.NodeStats))
	}

	node, ok := stats.NodeStats["test-node-1"]
	if !ok {
		t.Fatal("expected node test-node-1 not found")
	}

	// Verify query stats
	if node.Query.Hybrid.HybridQueryRequests != 500 {
		t.Errorf("expected hybrid_query_requests 500, got %d", node.Query.Hybrid.HybridQueryRequests)
	}
	if node.Query.Neural.NeuralQueryRequests != 1000 {
		t.Errorf("expected neural_query_requests 1000, got %d", node.Query.Neural.NeuralQueryRequests)
	}
	if node.Query.NeuralSparse.NeuralSparseQueryRequests != 200 {
		t.Errorf("expected neural_sparse_query_requests 200, got %d", node.Query.NeuralSparse.NeuralSparseQueryRequests)
	}

	// Verify processor executions
	if node.Processors.Search.Hybrid.NormalizationProcessorExecutions != 400 {
		t.Errorf("expected normalization_processor_executions 400, got %d", node.Processors.Search.Hybrid.NormalizationProcessorExecutions)
	}
	if node.Processors.Ingest.TextEmbeddingExecutions != 5000 {
		t.Errorf("expected text_embedding_executions 5000, got %d", node.Processors.Ingest.TextEmbeddingExecutions)
	}

	// Verify memory stats
	if node.Memory.Sparse.SparseMemoryUsage != 1024.5 {
		t.Errorf("expected sparse_memory_usage 1024.5, got %f", node.Memory.Sparse.SparseMemoryUsage)
	}
	if node.Memory.Sparse.SparseMemoryUsagePercentage != 5.5 {
		t.Errorf("expected sparse_memory_usage_percentage 5.5, got %f", node.Memory.Sparse.SparseMemoryUsagePercentage)
	}

	// Verify semantic highlighting
	if node.SemanticHighlighting.SemanticHighlightingRequestCount != 50 {
		t.Errorf("expected semantic_highlighting_request_count 50, got %d", node.SemanticHighlighting.SemanticHighlightingRequestCount)
	}
}

func TestCollectorCollect(t *testing.T) {
	testData := loadTestData(t)

	server := httptest.NewServer(statsHandler(t, testData))
	defer server.Close()

	cfg := &config.Config{
		OpenSearchURL:     server.URL,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        0,
		RetryDelay:        100 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := NewCollector(osClient, nil)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Build a map of metric names for easier testing
	metricsMap := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricsMap[mf.GetName()] = true
	}

	// Verify expected metrics exist
	expectedMetrics := []string{
		"opensearch_neural_up",
		"opensearch_neural_scrape_duration_seconds",
		"opensearch_neural_nodes_total",
		"opensearch_neural_nodes_successful",
		"opensearch_neural_nodes_failed",
		"opensearch_neural_info_normalization_processors",
		"opensearch_neural_info_text_embedding_processors_in_pipelines",
		"opensearch_neural_hybrid_query_requests_total",
		"opensearch_neural_neural_query_requests_total",
		"opensearch_neural_neural_sparse_query_requests_total",
		"opensearch_neural_normalization_processor_executions_total",
		"opensearch_neural_text_embedding_executions_total",
		"opensearch_neural_sparse_memory_usage_bytes",
		"opensearch_neural_semantic_highlighting_requests_total",
	}

	for _, metricName := range expectedMetrics {
		if !metricsMap[metricName] {
			t.Errorf("expected metric %s not found", metricName)
		}
	}
}

func TestCollectorUpMetricOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		OpenSearchURL:     server.URL,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        0,
		RetryDelay:        100 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := NewCollector(osClient, nil)

	// Collect metrics
	ch := make(chan prometheus.Metric, 100)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	// Find the up metric
	var upMetric prometheus.Metric
	for metric := range ch {
		desc := metric.Desc()
		if strings.Contains(desc.String(), "opensearch_neural_up") {
			upMetric = metric
			break
		}
	}

	if upMetric == nil {
		t.Fatal("up metric not found")
	}

	// Verify up metric is 0 (failure)
	value := getMetricValue(upMetric)
	if value != 0 {
		t.Errorf("expected up metric to be 0 on error, got %f", value)
	}
}

func TestCollectorDescribe(t *testing.T) {
	cfg := config.DefaultConfig()

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := NewCollector(osClient, nil)

	ch := make(chan *prometheus.Desc, 200)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	descCount := 0
	for range ch {
		descCount++
	}

	// Verify we have a reasonable number of metric descriptors
	if descCount < 50 {
		t.Errorf("expected at least 50 metric descriptors, got %d", descCount)
	}
}

func TestMemoryConversion(t *testing.T) {
	// Verify KB to bytes conversion
	testData := loadTestData(t)

	server := httptest.NewServer(statsHandler(t, testData))
	defer server.Close()

	cfg := &config.Config{
		OpenSearchURL:     server.URL,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        0,
		RetryDelay:        100 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := NewCollector(osClient, nil)

	ch := make(chan prometheus.Metric, 200)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	// Find memory metric and verify conversion
	for metric := range ch {
		desc := metric.Desc()
		if strings.Contains(desc.String(), "opensearch_neural_sparse_memory_usage_bytes") {
			value := getMetricValue(metric)
			// 1024.5 KB * 1024 = 1049088 bytes
			expectedBytes := 1024.5 * 1024
			if value != expectedBytes {
				t.Errorf("expected sparse_memory_usage_bytes %f, got %f", expectedBytes, value)
			}
			return
		}
	}
}

func TestCollectorWithEmptyResponse(t *testing.T) {
	emptyResponse := `{
		"_nodes": {"total": 0, "successful": 0, "failed": 0},
		"cluster_name": "empty-cluster",
		"info": {
			"cluster_version": "3.4.0",
			"processors": {
				"search": {"hybrid": {}, "rerank_ml_processors": 0, "rerank_by_field_processors": 0, "neural_sparse_two_phase_processors": 0, "neural_query_enricher_processors": 0},
				"ingest": {}
			}
		},
		"all_nodes": {"query": {"hybrid": {}, "neural": {}, "neural_sparse": {}}, "semantic_highlighting": {}, "processors": {"search": {"hybrid": {}}, "ingest": {}}, "memory": {"sparse": {}}},
		"nodes": {}
	}`

	server := httptest.NewServer(statsHandler(t, []byte(emptyResponse)))
	defer server.Close()

	cfg := &config.Config{
		OpenSearchURL:     server.URL,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        0,
		RetryDelay:        100 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := NewCollector(osClient, nil)

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	// Should not panic with empty nodes
	_, err = registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics with empty response: %v", err)
	}
}

func TestStatsAreScopedToLocalNode(t *testing.T) {
	mock := &testutil.MockClient{Response: loadTestData(t)}
	collector := NewCollector(mock, nil)

	ch := make(chan prometheus.Metric, 200)
	collector.Collect(ch)
	close(ch)

	want := []string{client.LocalNodePath, "/_plugins/_neural/" + testutil.LocalNodeID + "/stats"}
	if strings.Join(mock.Paths, " ") != strings.Join(want, " ") {
		t.Errorf("expected requests %v, got %v", want, mock.Paths)
	}
}

func TestUpIsZeroWhenLocalNodeUnresolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_nodes/_local" {
			t.Errorf("stats must not be requested without a local node ID, got %s", r.URL.Path)
		}
		w.Write([]byte(`{"nodes":{"../../_cluster/settings":{}}}`))
	}))
	defer server.Close()

	osClient, err := client.New(&config.Config{OpenSearchURL: server.URL, OpenSearchTimeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	ch := make(chan prometheus.Metric, 10)
	NewCollector(osClient, nil).Collect(ch)
	close(ch)
	for m := range ch {
		if strings.Contains(m.Desc().String(), "opensearch_neural_up") && getMetricValue(m) != 0 {
			t.Errorf("expected up 0, got %v", getMetricValue(m))
		}
	}
}

// Each agentic, seismic and MMR field has a distinct value, so a field wired to the wrong metric fails.
func TestAgenticSeismicMMRMetrics(t *testing.T) {
	body := []byte(`{
		"cluster_name": "c",
		"info": {"processors": {"search": {"agentic": {"agentic_context_processors": 1, "agentic_query_translator_processors": 2}}}},
		"nodes": {"n1": {
			"query": {"agentic": {"agentic_query_requests": 3}},
			"processors": {
				"search": {"mmr_neural_query_transformer_executions": 4, "agentic": {"agentic_query_translator_executions": 5, "agentic_context_executions": 6}},
				"ingest": {"sparse_encoding_seismic_executions": 7}
			}
		}}
	}`)

	ch := make(chan prometheus.Metric, 200)
	NewCollector(&testutil.MockClient{Response: body}, nil).Collect(ch)
	close(ch)

	got := make(map[string]float64)
	for m := range ch {
		desc := m.Desc().String()
		start := strings.Index(desc, `fqName: "`) + len(`fqName: "`)
		got[desc[start:start+strings.Index(desc[start:], `"`)]] = getMetricValue(m)
	}

	want := map[string]float64{
		"opensearch_neural_info_agentic_context_processors":               1,
		"opensearch_neural_info_agentic_query_translator_processors":      2,
		"opensearch_neural_agentic_query_requests_total":                  3,
		"opensearch_neural_mmr_neural_query_transformer_executions_total": 4,
		"opensearch_neural_agentic_query_translator_executions_total":     5,
		"opensearch_neural_agentic_context_executions_total":              6,
		"opensearch_neural_sparse_encoding_seismic_executions_total":      7,
	}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s: expected %v, got %v", name, v, got[name])
		}
	}
}
