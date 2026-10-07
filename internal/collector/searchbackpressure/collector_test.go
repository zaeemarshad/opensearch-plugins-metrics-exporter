package searchbackpressure

import (
	"errors"
	"maps"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/testutil"
)

func loadTestData(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/stats_response.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}
	return data
}

// gather registers the collector and returns all samples keyed by metric name.
func gather(t *testing.T, c *Collector) map[string][]*dto.Metric {
	t.Helper()
	registry := prometheus.NewRegistry()
	if err := registry.Register(c); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	out := make(map[string][]*dto.Metric)
	for _, mf := range families {
		out[mf.GetName()] = mf.GetMetric()
	}
	return out
}

func labelsMatch(m *dto.Metric, want map[string]string) bool {
	matched := 0
	for _, lp := range m.GetLabel() {
		if v, ok := want[lp.GetName()]; ok {
			if v != lp.GetValue() {
				return false
			}
			matched++
		}
	}
	return matched == len(want)
}

func find(metrics []*dto.Metric, want map[string]string) []*dto.Metric {
	var out []*dto.Metric
	for _, m := range metrics {
		if labelsMatch(m, want) {
			out = append(out, m)
		}
	}
	return out
}

func value(m *dto.Metric) float64 {
	if m.Gauge != nil {
		return m.Gauge.GetValue()
	}
	return m.Counter.GetValue()
}

func mustValue(t *testing.T, metrics map[string][]*dto.Metric, name string, labels map[string]string) float64 {
	t.Helper()
	found := find(metrics[name], labels)
	if len(found) != 1 {
		t.Fatalf("%s%v: expected 1 sample, got %d", name, labels, len(found))
	}
	return value(found[0])
}

// Scoping to the local node and the search_backpressure metric keeps each scrape off the other nodes.
func TestCollectorRequestsFilteredEndpoint(t *testing.T) {
	mock := &testutil.MockClient{Response: loadTestData(t)}
	gather(t, NewCollector(mock, nil))

	if len(mock.Paths) != 1 || mock.Paths[0] != "/_nodes/_local/stats/search_backpressure" {
		t.Errorf("expected one request to /_nodes/_local/stats/search_backpressure, got %v", mock.Paths)
	}
}

// Alerts on "mode != enforced" need exactly one active mode per node.
func TestModeIsOneHotPerNode(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: loadTestData(t)}, nil))

	want := map[string]string{
		"node-data-1":   "enforced",
		"node-data-2":   "monitor_only",
		"node-client-1": "disabled",
	}
	for node, active := range want {
		for _, m := range modes {
			expected := 0.0
			if m == active {
				expected = 1
			}
			got := mustValue(t, metrics, "opensearch_search_backpressure_mode", map[string]string{"node": node, "mode": m})
			if got != expected {
				t.Errorf("node %s mode %s: expected %v, got %v", node, m, expected, got)
			}
		}
	}
}

func TestUnknownModeIsStillReported(t *testing.T) {
	body := []byte(`{"cluster_name":"c","nodes":{"n1":{"search_backpressure":{"mode":"future_mode"}}}}`)
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: body}, nil))

	if got := mustValue(t, metrics, "opensearch_search_backpressure_mode", map[string]string{"node": "n1", "mode": "future_mode"}); got != 1 {
		t.Errorf("expected unknown mode to be reported as 1, got %v", got)
	}
}

// Task-level cancellations and per-tracker cancellations are separate counters; mixing them
// up would hide which resource triggered the cancellation.
func TestCancellationCountersMapToCorrectSeries(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: loadTestData(t)}, nil))
	shard := map[string]string{"node": "node-data-1", "task_type": "search_shard_task"}

	cases := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"opensearch_search_backpressure_task_cancellations_total", shard, 7},
		{"opensearch_search_backpressure_task_cancellation_limit_reached_total", shard, 3},
		{"opensearch_search_backpressure_task_completions_total", shard, 44471760},
		{"opensearch_search_backpressure_tracker_cancellations_total", with(shard, "tracker", "heap_usage"), 4},
		{"opensearch_search_backpressure_tracker_cancellations_total", with(shard, "tracker", "elapsed_time"), 2},
		{"opensearch_search_backpressure_tracker_cancellations_total", with(shard, "tracker", "cpu_usage"), 1},
	}
	for _, tc := range cases {
		if got := mustValue(t, metrics, tc.name, tc.labels); got != tc.want {
			t.Errorf("%s%v: expected %v, got %v", tc.name, tc.labels, tc.want, got)
		}
	}
}

func TestTrackerResourceGauges(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: loadTestData(t)}, nil))
	heap := map[string]string{"node": "node-data-1", "task_type": "search_shard_task", "tracker": "heap_usage"}
	cpu := with(heap, "tracker", "cpu_usage")

	cases := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"opensearch_search_backpressure_tracker_current_max_bytes", heap, 732617560},
		{"opensearch_search_backpressure_tracker_current_avg_bytes", heap, 382295610},
		{"opensearch_search_backpressure_tracker_rolling_avg_bytes", heap, 47743713},
		{"opensearch_search_backpressure_tracker_current_max_milliseconds", cpu, 210},
		{"opensearch_search_backpressure_tracker_current_avg_milliseconds", cpu, 89},
	}
	for _, tc := range cases {
		if got := mustValue(t, metrics, tc.name, tc.labels); got != tc.want {
			t.Errorf("%s%v: expected %v, got %v", tc.name, tc.labels, tc.want, got)
		}
	}

	// Time-based trackers have no byte values, so no byte series is emitted for them.
	if found := find(metrics["opensearch_search_backpressure_tracker_current_max_bytes"], cpu); len(found) != 0 {
		t.Errorf("expected no byte series for cpu_usage tracker, got %d", len(found))
	}
}

// A null or absent native memory tracker must not produce zero-valued series that look like real data.
func TestMissingNativeMemoryTrackerEmitsNoSeries(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: loadTestData(t)}, nil))
	name := "opensearch_search_backpressure_tracker_cancellations_total"

	for _, labels := range []map[string]string{
		{"node": "node-data-1", "task_type": "search_task", "tracker": "native_memory_usage"},
		{"node": "node-data-2", "task_type": "search_task", "tracker": "native_memory_usage"},
		{"node": "node-client-1", "task_type": "search_shard_task", "tracker": "native_memory_usage"},
	} {
		if found := find(metrics[name], labels); len(found) != 0 {
			t.Errorf("%v: expected no series, got %d", labels, len(found))
		}
	}

	present := map[string]string{"node": "node-client-1", "task_type": "search_task", "tracker": "native_memory_usage"}
	if got := mustValue(t, metrics, name, present); got != 5 {
		t.Errorf("expected native memory cancellations 5, got %v", got)
	}
	if got := mustValue(t, metrics, "opensearch_search_backpressure_tracker_current_max_bytes", present); got != 2048 {
		t.Errorf("expected native memory current max bytes 2048, got %v", got)
	}
}

func TestNodeWithoutSearchBackpressureIsSkipped(t *testing.T) {
	body := []byte(`{"cluster_name":"c","nodes":{"n1":{"name":"old-node"}}}`)
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: body}, nil))

	if len(metrics["opensearch_search_backpressure_mode"]) != 0 {
		t.Errorf("expected no mode series for a node without search_backpressure")
	}
	if got := mustValue(t, metrics, "opensearch_search_backpressure_up", nil); got != 1 {
		t.Errorf("expected up 1, got %v", got)
	}
}

func TestUpIsZeroOnFetchError(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Err: errors.New("connection refused")}, nil))

	if got := mustValue(t, metrics, "opensearch_search_backpressure_up", map[string]string{"cluster": "unknown"}); got != 0 {
		t.Errorf("expected up 0 on error, got %v", got)
	}
	if len(metrics["opensearch_search_backpressure_mode"]) != 0 {
		t.Errorf("expected no node metrics on error")
	}
}

func TestUpIsZeroOnInvalidJSON(t *testing.T) {
	metrics := gather(t, NewCollector(&testutil.MockClient{Response: []byte(`not json`)}, nil))

	if got := mustValue(t, metrics, "opensearch_search_backpressure_up", nil); got != 0 {
		t.Errorf("expected up 0 on invalid JSON, got %v", got)
	}
}

func with(base map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(base)+1)
	maps.Copy(out, base)
	out[k] = v
	return out
}
