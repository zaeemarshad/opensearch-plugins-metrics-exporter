// Package testutil provides helpers shared by collector tests.
package testutil

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
)

// LocalNodeID is the node ID that MockClient returns for client.LocalNodePath.
const LocalNodeID = "Z7eXl9nKRnmJP1GG22lqng"

// MockClient returns Err, the local node for client.LocalNodePath, or Response for any other path.
type MockClient struct {
	Response []byte
	Err      error
	Paths    []string
}

func (m *MockClient) Get(_ context.Context, path string) ([]byte, error) {
	m.Paths = append(m.Paths, path)
	if m.Err != nil {
		return nil, m.Err
	}
	if path == client.LocalNodePath {
		return []byte(`{"nodes":{"` + LocalNodeID + `":{"name":"test-node"}}}`), nil
	}
	return m.Response, nil
}

func (m *MockClient) Close() {}

// AssertVersionFixtures scrapes testdata/stats_response_<version>.json for each supported release.
// Metrics in added39 must be present on 3.9.0 and absent on 3.8.0.
func AssertVersionFixtures(t *testing.T, newCollector func(client.HTTPClient, *slog.Logger) prometheus.Collector, prefix string, added39 []string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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
			reg.MustRegister(newCollector(&MockClient{Response: body}, logger))
			mfs, err := reg.Gather()
			if err != nil {
				t.Fatalf("gather failed: %v", err)
			}

			names := make(map[string]bool)
			for _, mf := range mfs {
				names[mf.GetName()] = true
				if mf.GetName() == prefix+"_up" && mf.GetMetric()[0].GetGauge().GetValue() != 1 {
					t.Errorf("expected %s_up 1", prefix)
				}
			}
			if !names[prefix+"_up"] {
				t.Fatalf("%s_up not found", prefix)
			}
			for _, name := range added39 {
				if names[name] != tc.wantAdded {
					t.Errorf("%s present=%v, expected %v", name, names[name], tc.wantAdded)
				}
			}
		})
	}
}
