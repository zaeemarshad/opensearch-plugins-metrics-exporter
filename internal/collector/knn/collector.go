// Package knn provides a Prometheus collector for OpenSearch k-NN plugin metrics.
// It fetches statistics from the /_plugins/_knn/stats API endpoint and exposes
// them as Prometheus metrics with the opensearch_knn_ prefix.
package knn

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
)

const (
	namespace = "opensearch"
	subsystem = "knn"
)

// Collector implements prometheus.Collector for k-NN plugin metrics.
type Collector struct {
	client client.HTTPClient
	logger *slog.Logger

	// Mutex to protect metrics during collection
	mu sync.Mutex

	// Meta metrics
	up             *prometheus.Desc
	scrapeDuration *prometheus.Desc

	// Cluster-level metrics
	circuitBreakerTriggered *prometheus.Desc
	modelIndexStatus        *prometheus.Desc
	nodesTotal              *prometheus.Desc
	nodesSuccessful         *prometheus.Desc
	nodesFailed             *prometheus.Desc

	// Node-level core metrics
	graphMemoryUsageBytes      *prometheus.Desc
	graphMemoryUsagePercentage *prometheus.Desc
	cacheCapacityReached       *prometheus.Desc
	totalLoadTimeNanoseconds   *prometheus.Desc
	evictionCountTotal         *prometheus.Desc
	hitCountTotal              *prometheus.Desc
	missCountTotal             *prometheus.Desc
	loadSuccessCountTotal      *prometheus.Desc
	loadExceptionCountTotal    *prometheus.Desc

	// Graph query/index metrics
	graphQueryRequestsTotal *prometheus.Desc
	graphQueryErrorsTotal   *prometheus.Desc
	graphIndexRequestsTotal *prometheus.Desc
	graphIndexErrorsTotal   *prometheus.Desc

	// k-NN query metrics
	knnQueryRequestsTotal                   *prometheus.Desc
	knnQueryWithFilterRequestsTotal         *prometheus.Desc
	maxDistanceQueryRequestsTotal           *prometheus.Desc
	maxDistanceQueryWithFilterRequestsTotal *prometheus.Desc
	minScoreQueryRequestsTotal              *prometheus.Desc
	minScoreQueryWithFilterRequestsTotal    *prometheus.Desc

	// Script metrics
	scriptCompilationsTotal      *prometheus.Desc
	scriptCompilationErrorsTotal *prometheus.Desc
	scriptQueryRequestsTotal     *prometheus.Desc
	scriptQueryErrorsTotal       *prometheus.Desc

	// Engine initialization
	faissInitialized  *prometheus.Desc
	nmslibInitialized *prometheus.Desc
	luceneInitialized *prometheus.Desc

	// Training metrics
	trainingRequestsTotal         *prometheus.Desc
	trainingErrorsTotal           *prometheus.Desc
	trainingMemoryUsageBytes      *prometheus.Desc
	trainingMemoryUsagePercentage *prometheus.Desc
	indexingFromModelDegraded     *prometheus.Desc

	// Graph stats - refresh
	graphStatsRefreshTotal            *prometheus.Desc
	graphStatsRefreshTimeMilliseconds *prometheus.Desc

	// Graph stats - merge
	graphStatsMergeCurrent          *prometheus.Desc
	graphStatsMergeTotal            *prometheus.Desc
	graphStatsMergeTimeMilliseconds *prometheus.Desc
	graphStatsMergeCurrentDocs      *prometheus.Desc
	graphStatsMergeTotalDocs        *prometheus.Desc
	graphStatsMergeTotalSizeBytes   *prometheus.Desc
	graphStatsMergeCurrentSizeBytes *prometheus.Desc

	// Indices in cache
	indicesInCacheGraphCount       *prometheus.Desc
	indicesInCacheMemoryBytes      *prometheus.Desc
	indicesInCacheMemoryPercentage *prometheus.Desc

	// Remote build - repository
	remoteBuildRepoReadSuccessTotal      *prometheus.Desc
	remoteBuildRepoReadFailureTotal      *prometheus.Desc
	remoteBuildRepoReadTimeMilliseconds  *prometheus.Desc
	remoteBuildRepoWriteSuccessTotal     *prometheus.Desc
	remoteBuildRepoWriteFailureTotal     *prometheus.Desc
	remoteBuildRepoWriteTimeMilliseconds *prometheus.Desc

	// Remote build - client
	remoteBuildClientStatusRequestSuccessTotal *prometheus.Desc
	remoteBuildClientStatusRequestFailureTotal *prometheus.Desc
	remoteBuildClientBuildRequestSuccessTotal  *prometheus.Desc
	remoteBuildClientBuildRequestFailureTotal  *prometheus.Desc
	remoteBuildClientIndexBuildSuccessTotal    *prometheus.Desc
	remoteBuildClientIndexBuildFailureTotal    *prometheus.Desc
	remoteBuildClientWaitingTimeMilliseconds   *prometheus.Desc

	// Remote build - build
	remoteBuildFlushTimeMilliseconds  *prometheus.Desc
	remoteBuildMergeTimeMilliseconds  *prometheus.Desc
	remoteBuildCurrentMergeOperations *prometheus.Desc
	remoteBuildCurrentFlushOperations *prometheus.Desc
	remoteBuildCurrentMergeSizeBytes  *prometheus.Desc
	remoteBuildCurrentFlushSizeBytes  *prometheus.Desc
}

// NewCollector creates a new k-NN metrics collector.
func NewCollector(c client.HTTPClient, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}

	nodeLabels := []string{"cluster", "node"}
	clusterLabels := []string{"cluster"}
	indexLabels := []string{"cluster", "node", "index"}

	return &Collector{
		client: c,
		logger: logger,

		// Meta metrics
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Whether the last scrape of k-NN stats was successful (1 for success, 0 for failure)",
			clusterLabels, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "scrape_duration_seconds"),
			"Duration of the last scrape in seconds",
			clusterLabels, nil,
		),

		// Cluster-level metrics
		circuitBreakerTriggered: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "circuit_breaker_triggered"),
			"Whether the circuit breaker is triggered (1 for triggered, 0 for not triggered)",
			clusterLabels, nil,
		),
		modelIndexStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "model_index_status"),
			"Model index status (1=green, 0.5=yellow, 0=red, -1=null/unknown)",
			[]string{"cluster", "status"}, nil,
		),
		nodesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_total"),
			"Total number of nodes in the stats response",
			clusterLabels, nil,
		),
		nodesSuccessful: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_successful"),
			"Number of nodes that responded successfully",
			clusterLabels, nil,
		),
		nodesFailed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nodes_failed"),
			"Number of nodes that failed to respond",
			clusterLabels, nil,
		),

		// Node-level core metrics
		graphMemoryUsageBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_memory_usage_bytes"),
			"Native memory used by k-NN graphs in bytes",
			nodeLabels, nil,
		),
		graphMemoryUsagePercentage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_memory_usage_percentage"),
			"Native memory usage as a percentage of the cache capacity",
			nodeLabels, nil,
		),
		cacheCapacityReached: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "cache_capacity_reached"),
			"Whether the cache capacity has been reached (1 for reached, 0 for not reached)",
			nodeLabels, nil,
		),
		totalLoadTimeNanoseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "total_load_time_nanoseconds"),
			"Total time spent loading k-NN graphs into cache in nanoseconds",
			nodeLabels, nil,
		),
		evictionCountTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "eviction_count_total"),
			"Total number of graph evictions from cache",
			nodeLabels, nil,
		),
		hitCountTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "hit_count_total"),
			"Total number of cache hits",
			nodeLabels, nil,
		),
		missCountTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "miss_count_total"),
			"Total number of cache misses",
			nodeLabels, nil,
		),
		loadSuccessCountTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "load_success_count_total"),
			"Total number of successful graph loads",
			nodeLabels, nil,
		),
		loadExceptionCountTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "load_exception_count_total"),
			"Total number of graph load exceptions",
			nodeLabels, nil,
		),

		// Graph query/index metrics
		graphQueryRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_query_requests_total"),
			"Total number of graph query requests",
			nodeLabels, nil,
		),
		graphQueryErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_query_errors_total"),
			"Total number of graph query errors",
			nodeLabels, nil,
		),
		graphIndexRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_index_requests_total"),
			"Total number of graph index requests",
			nodeLabels, nil,
		),
		graphIndexErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_index_errors_total"),
			"Total number of graph index errors",
			nodeLabels, nil,
		),

		// k-NN query metrics
		knnQueryRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "knn_query_requests_total"),
			"Total number of k-NN query requests",
			nodeLabels, nil,
		),
		knnQueryWithFilterRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "knn_query_with_filter_requests_total"),
			"Total number of k-NN query requests with filters",
			nodeLabels, nil,
		),
		maxDistanceQueryRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "max_distance_query_requests_total"),
			"Total number of max distance query requests",
			nodeLabels, nil,
		),
		maxDistanceQueryWithFilterRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "max_distance_query_with_filter_requests_total"),
			"Total number of max distance query requests with filters",
			nodeLabels, nil,
		),
		minScoreQueryRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "min_score_query_requests_total"),
			"Total number of min score query requests",
			nodeLabels, nil,
		),
		minScoreQueryWithFilterRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "min_score_query_with_filter_requests_total"),
			"Total number of min score query requests with filters",
			nodeLabels, nil,
		),

		// Script metrics
		scriptCompilationsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "script_compilations_total"),
			"Total number of script compilations",
			nodeLabels, nil,
		),
		scriptCompilationErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "script_compilation_errors_total"),
			"Total number of script compilation errors",
			nodeLabels, nil,
		),
		scriptQueryRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "script_query_requests_total"),
			"Total number of script query requests",
			nodeLabels, nil,
		),
		scriptQueryErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "script_query_errors_total"),
			"Total number of script query errors",
			nodeLabels, nil,
		),

		// Engine initialization
		faissInitialized: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "faiss_initialized"),
			"Whether the FAISS engine is initialized (1 for initialized, 0 for not)",
			nodeLabels, nil,
		),
		nmslibInitialized: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "nmslib_initialized"),
			"Whether the NMSLIB engine is initialized (1 for initialized, 0 for not)",
			nodeLabels, nil,
		),
		luceneInitialized: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "lucene_initialized"),
			"Whether the Lucene engine is initialized (1 for initialized, 0 for not)",
			nodeLabels, nil,
		),

		// Training metrics
		trainingRequestsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "training_requests_total"),
			"Total number of training requests",
			nodeLabels, nil,
		),
		trainingErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "training_errors_total"),
			"Total number of training errors",
			nodeLabels, nil,
		),
		trainingMemoryUsageBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "training_memory_usage_bytes"),
			"Native memory used for training in bytes",
			nodeLabels, nil,
		),
		trainingMemoryUsagePercentage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "training_memory_usage_percentage"),
			"Training memory usage as a percentage",
			nodeLabels, nil,
		),
		indexingFromModelDegraded: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "indexing_from_model_degraded"),
			"Whether indexing from model is degraded (1 for degraded, 0 for not)",
			nodeLabels, nil,
		),

		// Graph stats - refresh
		graphStatsRefreshTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_refresh_total"),
			"Total number of graph refresh operations",
			nodeLabels, nil,
		),
		graphStatsRefreshTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_refresh_time_milliseconds"),
			"Total time spent on graph refresh operations in milliseconds",
			nodeLabels, nil,
		),

		// Graph stats - merge
		graphStatsMergeCurrent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_current"),
			"Current number of merge operations",
			nodeLabels, nil,
		),
		graphStatsMergeTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_total"),
			"Total number of merge operations",
			nodeLabels, nil,
		),
		graphStatsMergeTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_time_milliseconds"),
			"Total time spent on merge operations in milliseconds",
			nodeLabels, nil,
		),
		graphStatsMergeCurrentDocs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_current_docs"),
			"Current number of documents being merged",
			nodeLabels, nil,
		),
		graphStatsMergeTotalDocs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_total_docs"),
			"Total number of documents merged",
			nodeLabels, nil,
		),
		graphStatsMergeTotalSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_total_size_bytes"),
			"Total size of merged data in bytes",
			nodeLabels, nil,
		),
		graphStatsMergeCurrentSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "graph_stats_merge_current_size_bytes"),
			"Current size of data being merged in bytes",
			nodeLabels, nil,
		),

		// Indices in cache
		indicesInCacheGraphCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "indices_in_cache_graph_count"),
			"Number of graphs in cache for this index",
			indexLabels, nil,
		),
		indicesInCacheMemoryBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "indices_in_cache_memory_bytes"),
			"Memory usage in bytes for cached graphs of this index",
			indexLabels, nil,
		),
		indicesInCacheMemoryPercentage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "indices_in_cache_memory_percentage"),
			"Memory usage percentage for cached graphs of this index",
			indexLabels, nil,
		),

		// Remote build - repository
		remoteBuildRepoReadSuccessTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_read_success_total"),
			"Total number of successful repository read operations",
			nodeLabels, nil,
		),
		remoteBuildRepoReadFailureTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_read_failure_total"),
			"Total number of failed repository read operations",
			nodeLabels, nil,
		),
		remoteBuildRepoReadTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_read_time_milliseconds"),
			"Total time spent on successful repository read operations in milliseconds",
			nodeLabels, nil,
		),
		remoteBuildRepoWriteSuccessTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_write_success_total"),
			"Total number of successful repository write operations",
			nodeLabels, nil,
		),
		remoteBuildRepoWriteFailureTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_write_failure_total"),
			"Total number of failed repository write operations",
			nodeLabels, nil,
		),
		remoteBuildRepoWriteTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_repository_write_time_milliseconds"),
			"Total time spent on successful repository write operations in milliseconds",
			nodeLabels, nil,
		),

		// Remote build - client
		remoteBuildClientStatusRequestSuccessTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_status_request_success_total"),
			"Total number of successful status requests",
			nodeLabels, nil,
		),
		remoteBuildClientStatusRequestFailureTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_status_request_failure_total"),
			"Total number of failed status requests",
			nodeLabels, nil,
		),
		remoteBuildClientBuildRequestSuccessTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_build_request_success_total"),
			"Total number of successful build requests",
			nodeLabels, nil,
		),
		remoteBuildClientBuildRequestFailureTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_build_request_failure_total"),
			"Total number of failed build requests",
			nodeLabels, nil,
		),
		remoteBuildClientIndexBuildSuccessTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_index_build_success_total"),
			"Total number of successful index builds",
			nodeLabels, nil,
		),
		remoteBuildClientIndexBuildFailureTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_index_build_failure_total"),
			"Total number of failed index builds",
			nodeLabels, nil,
		),
		remoteBuildClientWaitingTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_client_waiting_time_milliseconds"),
			"Time spent waiting for remote builds in milliseconds",
			nodeLabels, nil,
		),

		// Remote build - build
		remoteBuildFlushTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_flush_time_milliseconds"),
			"Total time spent on remote build flush operations in milliseconds",
			nodeLabels, nil,
		),
		remoteBuildMergeTimeMilliseconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_merge_time_milliseconds"),
			"Total time spent on remote build merge operations in milliseconds",
			nodeLabels, nil,
		),
		remoteBuildCurrentMergeOperations: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_current_merge_operations"),
			"Current number of remote build merge operations",
			nodeLabels, nil,
		),
		remoteBuildCurrentFlushOperations: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_current_flush_operations"),
			"Current number of remote build flush operations",
			nodeLabels, nil,
		),
		remoteBuildCurrentMergeSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_current_merge_size_bytes"),
			"Current size of remote build merge operations in bytes",
			nodeLabels, nil,
		),
		remoteBuildCurrentFlushSizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "remote_build_current_flush_size_bytes"),
			"Current size of remote build flush operations in bytes",
			nodeLabels, nil,
		),
	}
}

// Describe sends the descriptors of all metrics to the provided channel.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration

	ch <- c.circuitBreakerTriggered
	ch <- c.modelIndexStatus
	ch <- c.nodesTotal
	ch <- c.nodesSuccessful
	ch <- c.nodesFailed

	ch <- c.graphMemoryUsageBytes
	ch <- c.graphMemoryUsagePercentage
	ch <- c.cacheCapacityReached
	ch <- c.totalLoadTimeNanoseconds
	ch <- c.evictionCountTotal
	ch <- c.hitCountTotal
	ch <- c.missCountTotal
	ch <- c.loadSuccessCountTotal
	ch <- c.loadExceptionCountTotal

	ch <- c.graphQueryRequestsTotal
	ch <- c.graphQueryErrorsTotal
	ch <- c.graphIndexRequestsTotal
	ch <- c.graphIndexErrorsTotal

	ch <- c.knnQueryRequestsTotal
	ch <- c.knnQueryWithFilterRequestsTotal
	ch <- c.maxDistanceQueryRequestsTotal
	ch <- c.maxDistanceQueryWithFilterRequestsTotal
	ch <- c.minScoreQueryRequestsTotal
	ch <- c.minScoreQueryWithFilterRequestsTotal

	ch <- c.scriptCompilationsTotal
	ch <- c.scriptCompilationErrorsTotal
	ch <- c.scriptQueryRequestsTotal
	ch <- c.scriptQueryErrorsTotal

	ch <- c.faissInitialized
	ch <- c.nmslibInitialized
	ch <- c.luceneInitialized

	ch <- c.trainingRequestsTotal
	ch <- c.trainingErrorsTotal
	ch <- c.trainingMemoryUsageBytes
	ch <- c.trainingMemoryUsagePercentage
	ch <- c.indexingFromModelDegraded

	ch <- c.graphStatsRefreshTotal
	ch <- c.graphStatsRefreshTimeMilliseconds
	ch <- c.graphStatsMergeCurrent
	ch <- c.graphStatsMergeTotal
	ch <- c.graphStatsMergeTimeMilliseconds
	ch <- c.graphStatsMergeCurrentDocs
	ch <- c.graphStatsMergeTotalDocs
	ch <- c.graphStatsMergeTotalSizeBytes
	ch <- c.graphStatsMergeCurrentSizeBytes

	ch <- c.indicesInCacheGraphCount
	ch <- c.indicesInCacheMemoryBytes
	ch <- c.indicesInCacheMemoryPercentage

	ch <- c.remoteBuildRepoReadSuccessTotal
	ch <- c.remoteBuildRepoReadFailureTotal
	ch <- c.remoteBuildRepoReadTimeMilliseconds
	ch <- c.remoteBuildRepoWriteSuccessTotal
	ch <- c.remoteBuildRepoWriteFailureTotal
	ch <- c.remoteBuildRepoWriteTimeMilliseconds

	ch <- c.remoteBuildClientStatusRequestSuccessTotal
	ch <- c.remoteBuildClientStatusRequestFailureTotal
	ch <- c.remoteBuildClientBuildRequestSuccessTotal
	ch <- c.remoteBuildClientBuildRequestFailureTotal
	ch <- c.remoteBuildClientIndexBuildSuccessTotal
	ch <- c.remoteBuildClientIndexBuildFailureTotal
	ch <- c.remoteBuildClientWaitingTimeMilliseconds

	ch <- c.remoteBuildFlushTimeMilliseconds
	ch <- c.remoteBuildMergeTimeMilliseconds
	ch <- c.remoteBuildCurrentMergeOperations
	ch <- c.remoteBuildCurrentFlushOperations
	ch <- c.remoteBuildCurrentMergeSizeBytes
	ch <- c.remoteBuildCurrentFlushSizeBytes
}

// Collect fetches metrics from OpenSearch and sends them to the provided channel.
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

	// Always emit scrape duration
	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration, clusterName)

	if err != nil {
		c.logger.Error("failed to fetch k-NN stats", "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, clusterName)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, clusterName)

	c.collectClusterMetrics(ch, stats)
	c.collectNodeMetrics(ch, stats)
}

func (c *Collector) fetchStats(ctx context.Context) (*StatsResponse, error) {
	body, err := c.client.Get(ctx, "/_plugins/_knn/stats")
	if err != nil {
		return nil, err
	}

	var stats StatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

func (c *Collector) collectClusterMetrics(ch chan<- prometheus.Metric, stats *StatsResponse) {
	cluster := stats.ClusterName

	// Circuit breaker
	ch <- prometheus.MustNewConstMetric(c.circuitBreakerTriggered, prometheus.GaugeValue,
		boolToFloat64(stats.CircuitBreakerTriggered), cluster)

	// Model index status
	statusValue, statusLabel := parseModelIndexStatus(stats.ModelIndexStatus)
	ch <- prometheus.MustNewConstMetric(c.modelIndexStatus, prometheus.GaugeValue,
		statusValue, cluster, statusLabel)

	// Nodes info
	ch <- prometheus.MustNewConstMetric(c.nodesTotal, prometheus.GaugeValue,
		float64(stats.Nodes.Total), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesSuccessful, prometheus.GaugeValue,
		float64(stats.Nodes.Successful), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesFailed, prometheus.GaugeValue,
		float64(stats.Nodes.Failed), cluster)
}

func (c *Collector) collectNodeMetrics(ch chan<- prometheus.Metric, stats *StatsResponse) {
	cluster := stats.ClusterName

	for nodeID, node := range stats.NodeStats {
		labels := []string{cluster, nodeID}

		// Core metrics (convert KB to bytes where applicable)
		ch <- prometheus.MustNewConstMetric(c.graphMemoryUsageBytes, prometheus.GaugeValue,
			float64(node.GraphMemoryUsage*1024), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphMemoryUsagePercentage, prometheus.GaugeValue,
			node.GraphMemoryUsagePercentage, labels...)
		ch <- prometheus.MustNewConstMetric(c.cacheCapacityReached, prometheus.GaugeValue,
			boolToFloat64(node.CacheCapacityReached), labels...)
		ch <- prometheus.MustNewConstMetric(c.totalLoadTimeNanoseconds, prometheus.CounterValue,
			float64(node.TotalLoadTime), labels...)
		ch <- prometheus.MustNewConstMetric(c.evictionCountTotal, prometheus.CounterValue,
			float64(node.EvictionCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.hitCountTotal, prometheus.CounterValue,
			float64(node.HitCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.missCountTotal, prometheus.CounterValue,
			float64(node.MissCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.loadSuccessCountTotal, prometheus.CounterValue,
			float64(node.LoadSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.loadExceptionCountTotal, prometheus.CounterValue,
			float64(node.LoadExceptionCount), labels...)

		// Graph query/index metrics
		ch <- prometheus.MustNewConstMetric(c.graphQueryRequestsTotal, prometheus.CounterValue,
			float64(node.GraphQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphQueryErrorsTotal, prometheus.CounterValue,
			float64(node.GraphQueryErrors), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphIndexRequestsTotal, prometheus.CounterValue,
			float64(node.GraphIndexRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphIndexErrorsTotal, prometheus.CounterValue,
			float64(node.GraphIndexErrors), labels...)

		// k-NN query metrics
		ch <- prometheus.MustNewConstMetric(c.knnQueryRequestsTotal, prometheus.CounterValue,
			float64(node.KNNQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.knnQueryWithFilterRequestsTotal, prometheus.CounterValue,
			float64(node.KNNQueryWithFilterRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.maxDistanceQueryRequestsTotal, prometheus.CounterValue,
			float64(node.MaxDistanceQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.maxDistanceQueryWithFilterRequestsTotal, prometheus.CounterValue,
			float64(node.MaxDistanceQueryWithFilterRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.minScoreQueryRequestsTotal, prometheus.CounterValue,
			float64(node.MinScoreQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.minScoreQueryWithFilterRequestsTotal, prometheus.CounterValue,
			float64(node.MinScoreQueryWithFilterRequests), labels...)

		// Script metrics
		ch <- prometheus.MustNewConstMetric(c.scriptCompilationsTotal, prometheus.CounterValue,
			float64(node.ScriptCompilations), labels...)
		ch <- prometheus.MustNewConstMetric(c.scriptCompilationErrorsTotal, prometheus.CounterValue,
			float64(node.ScriptCompilationErrors), labels...)
		ch <- prometheus.MustNewConstMetric(c.scriptQueryRequestsTotal, prometheus.CounterValue,
			float64(node.ScriptQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.scriptQueryErrorsTotal, prometheus.CounterValue,
			float64(node.ScriptQueryErrors), labels...)

		// Engine initialization
		ch <- prometheus.MustNewConstMetric(c.faissInitialized, prometheus.GaugeValue,
			boolToFloat64(node.FaissInitialized), labels...)
		ch <- prometheus.MustNewConstMetric(c.nmslibInitialized, prometheus.GaugeValue,
			boolToFloat64(node.NmslibInitialized), labels...)
		ch <- prometheus.MustNewConstMetric(c.luceneInitialized, prometheus.GaugeValue,
			boolToFloat64(node.LuceneInitialized), labels...)

		// Training metrics
		ch <- prometheus.MustNewConstMetric(c.trainingRequestsTotal, prometheus.CounterValue,
			float64(node.TrainingRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.trainingErrorsTotal, prometheus.CounterValue,
			float64(node.TrainingErrors), labels...)
		ch <- prometheus.MustNewConstMetric(c.trainingMemoryUsageBytes, prometheus.GaugeValue,
			float64(node.TrainingMemoryUsage*1024), labels...)
		ch <- prometheus.MustNewConstMetric(c.trainingMemoryUsagePercentage, prometheus.GaugeValue,
			node.TrainingMemoryUsagePercentage, labels...)
		ch <- prometheus.MustNewConstMetric(c.indexingFromModelDegraded, prometheus.GaugeValue,
			boolToFloat64(node.IndexingFromModelDegraded), labels...)

		// Graph stats - refresh
		ch <- prometheus.MustNewConstMetric(c.graphStatsRefreshTotal, prometheus.CounterValue,
			float64(node.GraphStats.Refresh.Total), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsRefreshTimeMilliseconds, prometheus.CounterValue,
			float64(node.GraphStats.Refresh.TotalTimeInMillis), labels...)

		// Graph stats - merge
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeCurrent, prometheus.GaugeValue,
			float64(node.GraphStats.Merge.Current), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeTotal, prometheus.CounterValue,
			float64(node.GraphStats.Merge.Total), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeTimeMilliseconds, prometheus.CounterValue,
			float64(node.GraphStats.Merge.TotalTimeInMillis), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeCurrentDocs, prometheus.GaugeValue,
			float64(node.GraphStats.Merge.CurrentDocs), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeTotalDocs, prometheus.CounterValue,
			float64(node.GraphStats.Merge.TotalDocs), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeTotalSizeBytes, prometheus.CounterValue,
			float64(node.GraphStats.Merge.TotalSizeInBytes), labels...)
		ch <- prometheus.MustNewConstMetric(c.graphStatsMergeCurrentSizeBytes, prometheus.GaugeValue,
			float64(node.GraphStats.Merge.CurrentSizeInBytes), labels...)

		// Indices in cache
		for indexName, indexStats := range node.IndicesInCache {
			indexLabels := []string{cluster, nodeID, indexName}
			ch <- prometheus.MustNewConstMetric(c.indicesInCacheGraphCount, prometheus.GaugeValue,
				float64(indexStats.GraphCount), indexLabels...)
			ch <- prometheus.MustNewConstMetric(c.indicesInCacheMemoryBytes, prometheus.GaugeValue,
				float64(indexStats.GraphMemoryUsage*1024), indexLabels...)
			ch <- prometheus.MustNewConstMetric(c.indicesInCacheMemoryPercentage, prometheus.GaugeValue,
				indexStats.GraphMemoryUsagePercentage, indexLabels...)
		}

		// Remote build - repository
		repo := node.RemoteVectorIndexBuildStats.RepositoryStats
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoReadSuccessTotal, prometheus.CounterValue,
			float64(repo.ReadSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoReadFailureTotal, prometheus.CounterValue,
			float64(repo.ReadFailureCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoReadTimeMilliseconds, prometheus.CounterValue,
			float64(repo.SuccessfulReadTimeInMillis), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoWriteSuccessTotal, prometheus.CounterValue,
			float64(repo.WriteSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoWriteFailureTotal, prometheus.CounterValue,
			float64(repo.WriteFailureCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildRepoWriteTimeMilliseconds, prometheus.CounterValue,
			float64(repo.SuccessfulWriteTimeInMillis), labels...)

		// Remote build - client
		client := node.RemoteVectorIndexBuildStats.ClientStats
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientStatusRequestSuccessTotal, prometheus.CounterValue,
			float64(client.StatusRequestSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientStatusRequestFailureTotal, prometheus.CounterValue,
			float64(client.StatusRequestFailureCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientBuildRequestSuccessTotal, prometheus.CounterValue,
			float64(client.BuildRequestSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientBuildRequestFailureTotal, prometheus.CounterValue,
			float64(client.BuildRequestFailureCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientIndexBuildSuccessTotal, prometheus.CounterValue,
			float64(client.IndexBuildSuccessCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientIndexBuildFailureTotal, prometheus.CounterValue,
			float64(client.IndexBuildFailureCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildClientWaitingTimeMilliseconds, prometheus.GaugeValue,
			float64(client.WaitingTimeInMs), labels...)

		// Remote build - build
		build := node.RemoteVectorIndexBuildStats.BuildStats
		ch <- prometheus.MustNewConstMetric(c.remoteBuildFlushTimeMilliseconds, prometheus.CounterValue,
			float64(build.RemoteIndexBuildFlushTimeInMillis), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildMergeTimeMilliseconds, prometheus.CounterValue,
			float64(build.RemoteIndexBuildMergeTimeInMillis), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildCurrentMergeOperations, prometheus.GaugeValue,
			float64(build.RemoteIndexBuildCurrentMergeOperations), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildCurrentFlushOperations, prometheus.GaugeValue,
			float64(build.RemoteIndexBuildCurrentFlushOperations), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildCurrentMergeSizeBytes, prometheus.GaugeValue,
			float64(build.RemoteIndexBuildCurrentMergeSize), labels...)
		ch <- prometheus.MustNewConstMetric(c.remoteBuildCurrentFlushSizeBytes, prometheus.GaugeValue,
			float64(build.RemoteIndexBuildCurrentFlushSize), labels...)
	}
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func parseModelIndexStatus(status *string) (float64, string) {
	if status == nil {
		return -1, "null"
	}

	switch strings.ToLower(*status) {
	case "green":
		return 1, "green"
	case "yellow":
		return 0.5, "yellow"
	case "red":
		return 0, "red"
	default:
		return -1, *status
	}
}
