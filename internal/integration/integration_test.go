//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/collector/knn"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/collector/neural"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/config"
)

func getOpenSearchURL() string {
	url := os.Getenv("OPENSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	return url
}

func waitForOpenSearch(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < 30; i++ {
		resp, err := client.Get(url + "/_cluster/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("OpenSearch at %s did not become ready", url)
}

func createTestKNNIndex(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}

	// Delete if exists
	req, _ := http.NewRequest(http.MethodDelete, url+"/integration-test-knn", nil)
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Create index
	indexBody := `{
		"settings": {
			"index": {
				"knn": true,
				"number_of_shards": 1,
				"number_of_replicas": 0
			}
		},
		"mappings": {
			"properties": {
				"embedding": {
					"type": "knn_vector",
					"dimension": 3,
					"method": {
						"name": "hnsw",
						"space_type": "l2",
						"engine": "lucene"
					}
				}
			}
		}
	}`

	req, _ = http.NewRequest(http.MethodPut, url+"/integration-test-knn", strings.NewReader(indexBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		t.Fatalf("failed to create index, status: %d", resp.StatusCode)
	}

	// Index a document
	docBody := `{"embedding": [1.0, 2.0, 3.0]}`
	req, _ = http.NewRequest(http.MethodPost, url+"/integration-test-knn/_doc/1", strings.NewReader(docBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to index document: %v", err)
	}
	resp.Body.Close()

	// Refresh
	req, _ = http.NewRequest(http.MethodPost, url+"/integration-test-knn/_refresh", nil)
	resp, _ = client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func cleanupTestIndex(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodDelete, url+"/integration-test-knn", nil)
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func enableNeuralStats(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	body := `{"persistent": {"plugins.neural_search.stats_enabled": true}}`
	req, _ := http.NewRequest(http.MethodPut, url+"/_cluster/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("warning: failed to enable neural stats: %v", err)
		return
	}
	resp.Body.Close()
}

func runKNNQuery(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	queryBody := `{
		"query": {
			"knn": {
				"embedding": {
					"vector": [1.0, 2.0, 3.0],
					"k": 1
				}
			}
		}
	}`

	req, _ := http.NewRequest(http.MethodPost, url+"/integration-test-knn/_search", strings.NewReader(queryBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("warning: k-NN query failed: %v", err)
		return
	}
	resp.Body.Close()
}

func TestIntegration_KNNCollector(t *testing.T) {
	url := getOpenSearchURL()
	waitForOpenSearch(t, url)

	// Setup
	createTestKNNIndex(t, url)
	defer cleanupTestIndex(t, url)

	// Run a k-NN query to generate stats
	runKNNQuery(t, url)

	// Create client and collector
	cfg := &config.Config{
		OpenSearchURL:     url,
		OpenSearchTimeout: 30 * time.Second,
		RetryCount:        3,
		RetryDelay:        1 * time.Second,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := knn.NewCollector(osClient, nil)

	// Register and gather
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Check for expected metrics
	metricsMap := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricsMap[mf.GetName()] = true
	}

	requiredMetrics := []string{
		"opensearch_knn_up",
		"opensearch_knn_scrape_duration_seconds",
	}

	for _, name := range requiredMetrics {
		if !metricsMap[name] {
			t.Errorf("required metric %s not found", name)
		}
	}

	// Verify up metric is 1
	for _, mf := range metricFamilies {
		if mf.GetName() == "opensearch_knn_up" {
			for _, m := range mf.GetMetric() {
				if m.GetGauge().GetValue() != 1 {
					t.Errorf("expected opensearch_knn_up = 1, got %f", m.GetGauge().GetValue())
				}
			}
		}
	}

	t.Logf("k-NN collector gathered %d metric families", len(metricFamilies))
}

func TestIntegration_NeuralCollector(t *testing.T) {
	url := getOpenSearchURL()
	waitForOpenSearch(t, url)

	// Enable neural stats
	enableNeuralStats(t, url)

	// Create client and collector
	cfg := &config.Config{
		OpenSearchURL:     url,
		OpenSearchTimeout: 30 * time.Second,
		RetryCount:        3,
		RetryDelay:        1 * time.Second,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	collector := neural.NewCollector(osClient, nil)

	// Register and gather
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Check for expected metrics
	metricsMap := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricsMap[mf.GetName()] = true
	}

	requiredMetrics := []string{
		"opensearch_neural_up",
		"opensearch_neural_scrape_duration_seconds",
	}

	for _, name := range requiredMetrics {
		if !metricsMap[name] {
			t.Errorf("required metric %s not found", name)
		}
	}

	t.Logf("Neural collector gathered %d metric families", len(metricFamilies))
}

func TestIntegration_ClientConnectivity(t *testing.T) {
	url := getOpenSearchURL()
	waitForOpenSearch(t, url)

	cfg := &config.Config{
		OpenSearchURL:     url,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        1,
		RetryDelay:        500 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	// Test basic connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body, err := osClient.Get(ctx, "/")
	if err != nil {
		t.Fatalf("failed to connect to OpenSearch: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := response["cluster_name"]; !ok {
		t.Error("expected cluster_name in response")
	}

	version := response["version"].(map[string]interface{})
	t.Logf("Connected to OpenSearch %s", version["number"])
}

func TestIntegration_KNNStats(t *testing.T) {
	url := getOpenSearchURL()
	waitForOpenSearch(t, url)

	cfg := &config.Config{
		OpenSearchURL:     url,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        1,
		RetryDelay:        500 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body, err := osClient.Get(ctx, "/_plugins/_knn/stats")
	if err != nil {
		t.Fatalf("failed to fetch k-NN stats: %v", err)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("failed to parse k-NN stats: %v", err)
	}

	if _, ok := stats["cluster_name"]; !ok {
		t.Error("expected cluster_name in k-NN stats")
	}

	t.Logf("k-NN stats retrieved successfully")
}

func TestIntegration_NeuralStats(t *testing.T) {
	url := getOpenSearchURL()
	waitForOpenSearch(t, url)

	// Enable neural stats first
	enableNeuralStats(t, url)

	cfg := &config.Config{
		OpenSearchURL:     url,
		OpenSearchTimeout: 10 * time.Second,
		RetryCount:        1,
		RetryDelay:        500 * time.Millisecond,
	}

	osClient, err := client.New(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer osClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body, err := osClient.Get(ctx, "/_plugins/_neural/stats")
	if err != nil {
		t.Fatalf("failed to fetch neural stats: %v", err)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("failed to parse neural stats: %v", err)
	}

	if _, ok := stats["cluster_name"]; !ok {
		t.Error("expected cluster_name in neural stats")
	}

	t.Logf("Neural stats retrieved successfully")
}
