package knn

import (
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/testutil"
)

func TestOpenSearchVersions(t *testing.T) {
	testutil.AssertVersionFixtures(t, func(c client.HTTPClient, l *slog.Logger) prometheus.Collector { return NewCollector(c, l) }, "opensearch_knn", []string{
		"opensearch_knn_remote_build_client_merge_abort_exceptions_total",
		"opensearch_knn_remote_build_client_terminal_exceptions_total",
	})
}
