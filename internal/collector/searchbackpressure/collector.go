// Package searchbackpressure provides a Prometheus collector for OpenSearch search backpressure metrics.
// It fetches statistics for the local node from the /_nodes/_local/stats/search_backpressure API endpoint and exposes
// them as Prometheus metrics with the opensearch_search_backpressure_ prefix.
package searchbackpressure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
)

const (
	namespace = "opensearch"
	subsystem = "search_backpressure"
)

var modes = []string{"disabled", "monitor_only", "enforced"}

// Collector implements prometheus.Collector for search backpressure metrics.
type Collector struct {
	client client.HTTPClient
	logger *slog.Logger
	mu     sync.Mutex

	// Meta metrics
	up             *prometheus.Desc
	scrapeDuration *prometheus.Desc

	// Cluster info
	nodesTotal      *prometheus.Desc
	nodesSuccessful *prometheus.Desc
	nodesFailed     *prometheus.Desc

	// Node mode
	mode *prometheus.Desc

	// Per task type
	taskCompletions              *prometheus.Desc
	taskCancellations            *prometheus.Desc
	taskCancellationLimitReached *prometheus.Desc

	// Per task type and resource tracker
	trackerCancellations    *prometheus.Desc
	trackerCurrentMaxBytes  *prometheus.Desc
	trackerCurrentAvgBytes  *prometheus.Desc
	trackerRollingAvgBytes  *prometheus.Desc
	trackerCurrentMaxMillis *prometheus.Desc
	trackerCurrentAvgMillis *prometheus.Desc
}

func NewCollector(c client.HTTPClient, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}

	clusterLabels := []string{"cluster"}
	modeLabels := []string{"cluster", "node", "mode"}
	taskLabels := []string{"cluster", "node", "task_type"}
	trackerLabels := []string{"cluster", "node", "task_type", "tracker"}

	return &Collector{
		client: c,
		logger: logger,

		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Whether the last scrape was successful (1=success, 0=failure)",
			clusterLabels, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "scrape_duration_seconds"),
			"Duration of the last scrape in seconds",
			clusterLabels, nil,
		),

		nodesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_total"),
			"Total nodes in response",
			clusterLabels, nil,
		),
		nodesSuccessful: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_successful"),
			"Successful node responses",
			clusterLabels, nil,
		),
		nodesFailed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_failed"),
			"Failed node responses",
			clusterLabels, nil,
		),

		mode: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "mode"),
			"Search backpressure mode of the node (1 for the active mode, 0 otherwise)",
			modeLabels, nil,
		),

		taskCompletions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "task_completions_total"),
			"Total completed tasks",
			taskLabels, nil,
		),
		taskCancellations: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "task_cancellations_total"),
			"Total tasks canceled by search backpressure",
			taskLabels, nil,
		),
		taskCancellationLimitReached: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "task_cancellation_limit_reached_total"),
			"Total times a cancellation was skipped because the cancellation rate limit was reached",
			taskLabels, nil,
		),

		trackerCancellations: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_cancellations_total"),
			"Total tasks canceled by a resource tracker",
			trackerLabels, nil,
		),
		trackerCurrentMaxBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_current_max_bytes"),
			"Maximum memory usage of currently running tasks in bytes",
			trackerLabels, nil,
		),
		trackerCurrentAvgBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_current_avg_bytes"),
			"Average memory usage of currently running tasks in bytes",
			trackerLabels, nil,
		),
		trackerRollingAvgBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_rolling_avg_bytes"),
			"Rolling average memory usage of completed tasks in bytes",
			trackerLabels, nil,
		),
		trackerCurrentMaxMillis: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_current_max_milliseconds"),
			"Maximum usage of currently running tasks in milliseconds",
			trackerLabels, nil,
		),
		trackerCurrentAvgMillis: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "tracker_current_avg_milliseconds"),
			"Average usage of currently running tasks in milliseconds",
			trackerLabels, nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.nodesTotal
	ch <- c.nodesSuccessful
	ch <- c.nodesFailed
	ch <- c.mode
	ch <- c.taskCompletions
	ch <- c.taskCancellations
	ch <- c.taskCancellationLimitReached
	ch <- c.trackerCancellations
	ch <- c.trackerCurrentMaxBytes
	ch <- c.trackerCurrentAvgBytes
	ch <- c.trackerRollingAvgBytes
	ch <- c.trackerCurrentMaxMillis
	ch <- c.trackerCurrentAvgMillis
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := c.fetchStats(ctx)
	duration := time.Since(start).Seconds()

	clusterName := "unknown"
	if stats != nil {
		clusterName = stats.ClusterName
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration, clusterName)

	if err != nil {
		c.logger.Error("failed to fetch search backpressure stats", "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, clusterName)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, clusterName)

	c.collectClusterMetrics(ch, stats)
	c.collectNodeMetrics(ch, stats)
}

func (c *Collector) fetchStats(ctx context.Context) (*StatsResponse, error) {
	body, err := c.client.Get(ctx, "/_nodes/_local/stats/search_backpressure")
	if err != nil {
		return nil, err
	}

	var stats StatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search backpressure stats response: %w", err)
	}

	return &stats, nil
}

func (c *Collector) collectClusterMetrics(ch chan<- prometheus.Metric, stats *StatsResponse) {
	cluster := stats.ClusterName

	ch <- prometheus.MustNewConstMetric(c.nodesTotal, prometheus.GaugeValue, float64(stats.Nodes.Total), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesSuccessful, prometheus.GaugeValue, float64(stats.Nodes.Successful), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesFailed, prometheus.GaugeValue, float64(stats.Nodes.Failed), cluster)
}

func (c *Collector) collectNodeMetrics(ch chan<- prometheus.Metric, stats *StatsResponse) {
	cluster := stats.ClusterName

	for nodeID, node := range stats.NodeStats {
		sbp := node.SearchBackpressure
		if sbp == nil {
			continue
		}

		c.collectMode(ch, cluster, nodeID, sbp.Mode)
		c.collectTask(ch, cluster, nodeID, "search_task", sbp.SearchTask)
		c.collectTask(ch, cluster, nodeID, "search_shard_task", sbp.SearchShardTask)
	}
}

func (c *Collector) collectMode(ch chan<- prometheus.Metric, cluster, nodeID, active string) {
	known := false
	for _, m := range modes {
		v := 0.0
		if m == active {
			v = 1
			known = true
		}
		ch <- prometheus.MustNewConstMetric(c.mode, prometheus.GaugeValue, v, cluster, nodeID, m)
	}
	if !known && active != "" {
		ch <- prometheus.MustNewConstMetric(c.mode, prometheus.GaugeValue, 1, cluster, nodeID, active)
	}
}

func (c *Collector) collectTask(ch chan<- prometheus.Metric, cluster, nodeID, taskType string, task TaskStats) {
	labels := []string{cluster, nodeID, taskType}

	ch <- prometheus.MustNewConstMetric(c.taskCompletions, prometheus.CounterValue, float64(task.CompletionCount), labels...)
	ch <- prometheus.MustNewConstMetric(c.taskCancellations, prometheus.CounterValue, float64(task.CancellationStats.CancellationCount), labels...)
	ch <- prometheus.MustNewConstMetric(c.taskCancellationLimitReached, prometheus.CounterValue, float64(task.CancellationStats.CancellationLimitReachedCount), labels...)

	for name, t := range task.ResourceTrackerStats {
		if t == nil {
			continue
		}
		tracker := strings.TrimSuffix(name, "_tracker")
		tl := []string{cluster, nodeID, taskType, tracker}

		ch <- prometheus.MustNewConstMetric(c.trackerCancellations, prometheus.CounterValue, float64(t.CancellationCount), tl...)

		switch tracker {
		case "heap_usage":
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentMaxBytes, prometheus.GaugeValue, float64(t.CurrentMaxBytes), tl...)
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentAvgBytes, prometheus.GaugeValue, float64(t.CurrentAvgBytes), tl...)
			ch <- prometheus.MustNewConstMetric(c.trackerRollingAvgBytes, prometheus.GaugeValue, float64(t.RollingAvgBytes), tl...)
		case "native_memory_usage":
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentMaxBytes, prometheus.GaugeValue, float64(t.CurrentMaxBytes), tl...)
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentAvgBytes, prometheus.GaugeValue, float64(t.CurrentAvgBytes), tl...)
		case "cpu_usage", "elapsed_time":
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentMaxMillis, prometheus.GaugeValue, float64(t.CurrentMaxMillis), tl...)
			ch <- prometheus.MustNewConstMetric(c.trackerCurrentAvgMillis, prometheus.GaugeValue, float64(t.CurrentAvgMillis), tl...)
		}
	}
}
