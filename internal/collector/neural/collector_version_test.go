package neural

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// Captured from opensearchproject/opensearch images. Fields added in a release must be exported
// for that release and must not appear as fake zero series on older releases.
func TestOpenSearchVersions(t *testing.T) {
	added39 := []string{"opensearch_neural_info_sparse_vector_indices", "opensearch_neural_info_sparse_vector_fields", "opensearch_neural_info_sparse_native_engine_indices", "opensearch_neural_info_sparse_native_engine_fields"}

	for _, tc := range []struct {
		version   string
		wantAdded bool
	}{
		{"3.8.0", false},
		{"3.9.0", true},
	} {
		t.Run(tc.version, func(t *testing.T) {
			body, err := os.ReadFile("testdata/stats_response_" + tc.version + ".json")
			if err != nil {
				t.Fatalf("failed to read test data: %v", err)
			}
			reg := prometheus.NewRegistry()
			reg.MustRegister(NewCollector(&mockClient{response: body}, slog.New(slog.NewTextHandler(io.Discard, nil))))
			mfs, err := reg.Gather()
			if err != nil {
				t.Fatalf("gather failed: %v", err)
			}

			names := make(map[string]bool)
			for _, mf := range mfs {
				names[mf.GetName()] = true
				if mf.GetName() == "opensearch_neural_up" && mf.GetMetric()[0].GetGauge().GetValue() != 1 {
					t.Errorf("expected opensearch_neural_up 1")
				}
			}
			if !names["opensearch_neural_up"] {
				t.Fatal("opensearch_neural_up not found")
			}
			nodeSeries := 0
			for name := range names {
				if strings.HasPrefix(name, "opensearch_neural_") {
					nodeSeries++
				}
			}
			if nodeSeries < 5 {
				t.Errorf("expected metrics beyond meta metrics, got %d families", nodeSeries)
			}
			for _, name := range added39 {
				if names[name] != tc.wantAdded {
					t.Errorf("%s present=%v, expected %v", name, names[name], tc.wantAdded)
				}
			}
		})
	}
}
