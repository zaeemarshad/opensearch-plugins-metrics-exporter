package neural

import (
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/testutil"
)

func TestOpenSearchVersions(t *testing.T) {
	testutil.AssertVersionFixtures(t, func(c client.HTTPClient, l *slog.Logger) prometheus.Collector { return NewCollector(c, l) }, "opensearch_neural", []string{
		"opensearch_neural_info_sparse_vector_indices",
		"opensearch_neural_info_sparse_vector_fields",
		"opensearch_neural_info_sparse_native_engine_indices",
		"opensearch_neural_info_sparse_native_engine_fields",
	})
}
