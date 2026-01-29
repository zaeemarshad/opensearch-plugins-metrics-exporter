// Package neural provides a Prometheus collector for OpenSearch Neural Search plugin metrics.
// It fetches statistics from the /_plugins/_neural/stats API endpoint and exposes
// them as Prometheus metrics with the opensearch_neural_ prefix.
// Note: Neural Search stats must be enabled via cluster setting:
// PUT /_cluster/settings {"persistent":{"plugins.neural_search.stats_enabled":true}}
package neural

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensearch-project/opensearch-plugins-metrics-exporter/internal/client"
)

const (
	namespace = "opensearch"
	subsystem = "neural"
)

// Collector implements prometheus.Collector for Neural Search plugin metrics.
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

	// Info: Ingest processor counts
	infoSparseEncodingProcessors               *prometheus.Desc
	infoSkipExistingProcessors                 *prometheus.Desc
	infoTextImageEmbeddingProcessors           *prometheus.Desc
	infoTextChunkingDelimiterProcessors        *prometheus.Desc
	infoTextEmbeddingProcessorsInPipelines     *prometheus.Desc
	infoTextChunkingFixedTokenLengthProcessors *prometheus.Desc
	infoTextChunkingFixedCharLengthProcessors  *prometheus.Desc
	infoTextChunkingProcessors                 *prometheus.Desc

	// Info: Search processor counts
	infoRerankMLProcessors             *prometheus.Desc
	infoRerankByFieldProcessors        *prometheus.Desc
	infoNeuralSparseTwoPhaseProcessors *prometheus.Desc
	infoNeuralQueryEnricherProcessors  *prometheus.Desc

	// Info: Hybrid processor counts
	infoNormalizationProcessors          *prometheus.Desc
	infoNormMinMaxProcessors             *prometheus.Desc
	infoNormL2Processors                 *prometheus.Desc
	infoNormZScoreProcessors             *prometheus.Desc
	infoCombArithmeticProcessors         *prometheus.Desc
	infoCombGeometricProcessors          *prometheus.Desc
	infoCombHarmonicProcessors           *prometheus.Desc
	infoRankBasedNormalizationProcessors *prometheus.Desc
	infoCombRRFProcessors                *prometheus.Desc

	// Query metrics
	hybridQueryRequests               *prometheus.Desc
	hybridQueryWithPaginationRequests *prometheus.Desc
	hybridQueryWithFilterRequests     *prometheus.Desc
	hybridQueryWithInnerHitsRequests  *prometheus.Desc

	neuralQueryRequests                      *prometheus.Desc
	neuralQueryAgainstKNNRequests            *prometheus.Desc
	neuralQueryAgainstSemanticDenseRequests  *prometheus.Desc
	neuralQueryAgainstSemanticSparseRequests *prometheus.Desc

	neuralSparseQueryRequests *prometheus.Desc
	seismicQueryRequests      *prometheus.Desc

	// Semantic highlighting
	semanticHighlightingRequestCount      *prometheus.Desc
	semanticHighlightingBatchRequestCount *prometheus.Desc

	// Search processor executions
	neuralSparseTwoPhaseExecutions *prometheus.Desc
	rerankByFieldExecutions        *prometheus.Desc
	neuralQueryEnricherExecutions  *prometheus.Desc
	rerankMLExecutions             *prometheus.Desc

	// Hybrid processor executions
	normalizationProcessorExecutions          *prometheus.Desc
	rankBasedNormalizationProcessorExecutions *prometheus.Desc
	combHarmonicExecutions                    *prometheus.Desc
	normZScoreExecutions                      *prometheus.Desc
	combRRFExecutions                         *prometheus.Desc
	normL2Executions                          *prometheus.Desc
	combArithmeticExecutions                  *prometheus.Desc
	combGeometricExecutions                   *prometheus.Desc
	normMinMaxExecutions                      *prometheus.Desc

	// Ingest processor executions
	skipExistingExecutions                 *prometheus.Desc
	textChunkingFixedTokenLengthExecutions *prometheus.Desc
	sparseEncodingExecutions               *prometheus.Desc
	textChunkingFixedCharLengthExecutions  *prometheus.Desc
	textChunkingExecutions                 *prometheus.Desc
	textEmbeddingExecutions                *prometheus.Desc
	semanticFieldExecutions                *prometheus.Desc
	semanticFieldChunkingExecutions        *prometheus.Desc
	textChunkingDelimiterExecutions        *prometheus.Desc
	textImageEmbeddingExecutions           *prometheus.Desc

	// Memory
	sparseMemoryUsageBytes      *prometheus.Desc
	sparseMemoryUsagePercentage *prometheus.Desc
	clusteredPostingUsageBytes  *prometheus.Desc
	forwardIndexUsageBytes      *prometheus.Desc
}

func NewCollector(c client.HTTPClient, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}

	clusterLabels := []string{"cluster"}
	nodeLabels := []string{"cluster", "node"}

	return &Collector{
		client: c,
		logger: logger,

		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Whether the last scrape was successful (1=success, 0=failure, -1=stats disabled)",
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

		// Info: Ingest processors
		infoSparseEncodingProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_sparse_encoding_processors"),
			"Number of sparse_encoding processors in ingest pipelines",
			clusterLabels, nil,
		),
		infoSkipExistingProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_skip_existing_processors"),
			"Number of processors with skip_existing=true",
			clusterLabels, nil,
		),
		infoTextImageEmbeddingProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_image_embedding_processors"),
			"Number of text_image_embedding processors",
			clusterLabels, nil,
		),
		infoTextChunkingDelimiterProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_chunking_delimiter_processors"),
			"Number of text_chunking processors using delimiter algorithm",
			clusterLabels, nil,
		),
		infoTextEmbeddingProcessorsInPipelines: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_embedding_processors_in_pipelines"),
			"Number of text_embedding processors in pipelines",
			clusterLabels, nil,
		),
		infoTextChunkingFixedTokenLengthProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_chunking_fixed_token_length_processors"),
			"Number of text_chunking processors using fixed_token_length",
			clusterLabels, nil,
		),
		infoTextChunkingFixedCharLengthProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_chunking_fixed_char_length_processors"),
			"Number of text_chunking processors using fixed_character_length",
			clusterLabels, nil,
		),
		infoTextChunkingProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_text_chunking_processors"),
			"Total number of text_chunking processors",
			clusterLabels, nil,
		),

		// Info: Search processors
		infoRerankMLProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_rerank_ml_processors"),
			"Number of rerank processors of ml_opensearch type",
			clusterLabels, nil,
		),
		infoRerankByFieldProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_rerank_by_field_processors"),
			"Number of rerank processors of by_field type",
			clusterLabels, nil,
		),
		infoNeuralSparseTwoPhaseProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_neural_sparse_two_phase_processors"),
			"Number of neural_sparse_two_phase_processor processors",
			clusterLabels, nil,
		),
		infoNeuralQueryEnricherProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_neural_query_enricher_processors"),
			"Number of neural_query_enricher processors",
			clusterLabels, nil,
		),

		// Info: Hybrid processors
		infoNormalizationProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_normalization_processors"),
			"Total number of normalization-processor processors",
			clusterLabels, nil,
		),
		infoNormMinMaxProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_norm_minmax_processors"),
			"Number of normalization processors with min_max technique",
			clusterLabels, nil,
		),
		infoNormL2Processors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_norm_l2_processors"),
			"Number of normalization processors with l2 technique",
			clusterLabels, nil,
		),
		infoNormZScoreProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_norm_zscore_processors"),
			"Number of normalization processors with z_score technique",
			clusterLabels, nil,
		),
		infoCombArithmeticProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_comb_arithmetic_processors"),
			"Number of normalization processors with arithmetic_mean combination",
			clusterLabels, nil,
		),
		infoCombGeometricProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_comb_geometric_processors"),
			"Number of normalization processors with geometric_mean combination",
			clusterLabels, nil,
		),
		infoCombHarmonicProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_comb_harmonic_processors"),
			"Number of normalization processors with harmonic_mean combination",
			clusterLabels, nil,
		),
		infoRankBasedNormalizationProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_rank_based_normalization_processors"),
			"Number of score-ranker-processor processors",
			clusterLabels, nil,
		),
		infoCombRRFProcessors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info_comb_rrf_processors"),
			"Number of score-ranker processors with rrf combination",
			clusterLabels, nil,
		),

		// Query metrics
		hybridQueryRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "hybrid_query_requests_total"),
			"Total hybrid query requests",
			nodeLabels, nil,
		),
		hybridQueryWithPaginationRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "hybrid_query_with_pagination_requests_total"),
			"Hybrid query requests with pagination",
			nodeLabels, nil,
		),
		hybridQueryWithFilterRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "hybrid_query_with_filter_requests_total"),
			"Hybrid query requests with filters",
			nodeLabels, nil,
		),
		hybridQueryWithInnerHitsRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "hybrid_query_with_inner_hits_requests_total"),
			"Hybrid query requests with inner hits",
			nodeLabels, nil,
		),

		neuralQueryRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_query_requests_total"),
			"Total neural query requests",
			nodeLabels, nil,
		),
		neuralQueryAgainstKNNRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_query_against_knn_requests_total"),
			"Neural query requests against k-NN fields",
			nodeLabels, nil,
		),
		neuralQueryAgainstSemanticDenseRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_query_against_semantic_dense_requests_total"),
			"Neural query requests against semantic dense fields",
			nodeLabels, nil,
		),
		neuralQueryAgainstSemanticSparseRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_query_against_semantic_sparse_requests_total"),
			"Neural query requests against semantic sparse fields",
			nodeLabels, nil,
		),

		neuralSparseQueryRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_sparse_query_requests_total"),
			"Neural sparse query requests against rank_features fields",
			nodeLabels, nil,
		),
		seismicQueryRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "seismic_query_requests_total"),
			"Neural sparse ANN queries using SEISMIC algorithm",
			nodeLabels, nil,
		),

		// Semantic highlighting
		semanticHighlightingRequestCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "semantic_highlighting_requests_total"),
			"Single inference semantic highlighting requests",
			nodeLabels, nil,
		),
		semanticHighlightingBatchRequestCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "semantic_highlighting_batch_requests_total"),
			"Batch inference semantic highlighting requests",
			nodeLabels, nil,
		),

		// Search processor executions
		neuralSparseTwoPhaseExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_sparse_two_phase_executions_total"),
			"neural_sparse_two_phase_processor executions",
			nodeLabels, nil,
		),
		rerankByFieldExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "rerank_by_field_executions_total"),
			"rerank by_field processor executions",
			nodeLabels, nil,
		),
		neuralQueryEnricherExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "neural_query_enricher_executions_total"),
			"neural_query_enricher processor executions",
			nodeLabels, nil,
		),
		rerankMLExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "rerank_ml_executions_total"),
			"rerank ml_opensearch processor executions",
			nodeLabels, nil,
		),

		// Hybrid processor executions
		normalizationProcessorExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "normalization_processor_executions_total"),
			"normalization-processor executions",
			nodeLabels, nil,
		),
		rankBasedNormalizationProcessorExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "rank_based_normalization_processor_executions_total"),
			"score-ranker-processor executions",
			nodeLabels, nil,
		),
		combHarmonicExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "comb_harmonic_executions_total"),
			"Normalization processor executions with harmonic_mean combination",
			nodeLabels, nil,
		),
		normZScoreExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "norm_zscore_executions_total"),
			"Normalization processor executions with z_score technique",
			nodeLabels, nil,
		),
		combRRFExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "comb_rrf_executions_total"),
			"score-ranker-processor executions with rrf combination",
			nodeLabels, nil,
		),
		normL2Executions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "norm_l2_executions_total"),
			"Normalization processor executions with l2 technique",
			nodeLabels, nil,
		),
		combArithmeticExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "comb_arithmetic_executions_total"),
			"Normalization processor executions with arithmetic_mean combination",
			nodeLabels, nil,
		),
		combGeometricExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "comb_geometric_executions_total"),
			"Normalization processor executions with geometric_mean combination",
			nodeLabels, nil,
		),
		normMinMaxExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "norm_minmax_executions_total"),
			"Normalization processor executions with min_max technique",
			nodeLabels, nil,
		),

		// Ingest processor executions
		skipExistingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "skip_existing_executions_total"),
			"Processor executions with skip_existing=true",
			nodeLabels, nil,
		),
		textChunkingFixedTokenLengthExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_chunking_fixed_token_length_executions_total"),
			"text_chunking processor executions with fixed_token_length",
			nodeLabels, nil,
		),
		sparseEncodingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "sparse_encoding_executions_total"),
			"sparse_encoding processor executions",
			nodeLabels, nil,
		),
		textChunkingFixedCharLengthExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_chunking_fixed_char_length_executions_total"),
			"text_chunking processor executions with fixed_character_length",
			nodeLabels, nil,
		),
		textChunkingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_chunking_executions_total"),
			"text_chunking processor executions",
			nodeLabels, nil,
		),
		textEmbeddingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_embedding_executions_total"),
			"text_embedding processor executions",
			nodeLabels, nil,
		),
		semanticFieldExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "semantic_field_executions_total"),
			"semantic field system processor executions",
			nodeLabels, nil,
		),
		semanticFieldChunkingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "semantic_field_chunking_executions_total"),
			"semantic field system chunking processor executions",
			nodeLabels, nil,
		),
		textChunkingDelimiterExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_chunking_delimiter_executions_total"),
			"text_chunking processor executions with delimiter algorithm",
			nodeLabels, nil,
		),
		textImageEmbeddingExecutions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "text_image_embedding_executions_total"),
			"text_image_embedding processor executions",
			nodeLabels, nil,
		),

		// Memory
		sparseMemoryUsageBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "sparse_memory_usage_bytes"),
			"JVM heap memory used for sparse data (bytes)",
			nodeLabels, nil,
		),
		sparseMemoryUsagePercentage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "sparse_memory_usage_percentage"),
			"Percentage of JVM heap used for sparse data",
			nodeLabels, nil,
		),
		clusteredPostingUsageBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "clustered_posting_usage_bytes"),
			"JVM heap memory used for clustered posting (bytes)",
			nodeLabels, nil,
		),
		forwardIndexUsageBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "forward_index_usage_bytes"),
			"JVM heap memory used for forward index (bytes)",
			nodeLabels, nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.nodesTotal
	ch <- c.nodesSuccessful
	ch <- c.nodesFailed

	// Info metrics
	ch <- c.infoSparseEncodingProcessors
	ch <- c.infoSkipExistingProcessors
	ch <- c.infoTextImageEmbeddingProcessors
	ch <- c.infoTextChunkingDelimiterProcessors
	ch <- c.infoTextEmbeddingProcessorsInPipelines
	ch <- c.infoTextChunkingFixedTokenLengthProcessors
	ch <- c.infoTextChunkingFixedCharLengthProcessors
	ch <- c.infoTextChunkingProcessors
	ch <- c.infoRerankMLProcessors
	ch <- c.infoRerankByFieldProcessors
	ch <- c.infoNeuralSparseTwoPhaseProcessors
	ch <- c.infoNeuralQueryEnricherProcessors
	ch <- c.infoNormalizationProcessors
	ch <- c.infoNormMinMaxProcessors
	ch <- c.infoNormL2Processors
	ch <- c.infoNormZScoreProcessors
	ch <- c.infoCombArithmeticProcessors
	ch <- c.infoCombGeometricProcessors
	ch <- c.infoCombHarmonicProcessors
	ch <- c.infoRankBasedNormalizationProcessors
	ch <- c.infoCombRRFProcessors

	// Query metrics
	ch <- c.hybridQueryRequests
	ch <- c.hybridQueryWithPaginationRequests
	ch <- c.hybridQueryWithFilterRequests
	ch <- c.hybridQueryWithInnerHitsRequests
	ch <- c.neuralQueryRequests
	ch <- c.neuralQueryAgainstKNNRequests
	ch <- c.neuralQueryAgainstSemanticDenseRequests
	ch <- c.neuralQueryAgainstSemanticSparseRequests
	ch <- c.neuralSparseQueryRequests
	ch <- c.seismicQueryRequests

	// Semantic highlighting
	ch <- c.semanticHighlightingRequestCount
	ch <- c.semanticHighlightingBatchRequestCount

	// Processor executions
	ch <- c.neuralSparseTwoPhaseExecutions
	ch <- c.rerankByFieldExecutions
	ch <- c.neuralQueryEnricherExecutions
	ch <- c.rerankMLExecutions
	ch <- c.normalizationProcessorExecutions
	ch <- c.rankBasedNormalizationProcessorExecutions
	ch <- c.combHarmonicExecutions
	ch <- c.normZScoreExecutions
	ch <- c.combRRFExecutions
	ch <- c.normL2Executions
	ch <- c.combArithmeticExecutions
	ch <- c.combGeometricExecutions
	ch <- c.normMinMaxExecutions
	ch <- c.skipExistingExecutions
	ch <- c.textChunkingFixedTokenLengthExecutions
	ch <- c.sparseEncodingExecutions
	ch <- c.textChunkingFixedCharLengthExecutions
	ch <- c.textChunkingExecutions
	ch <- c.textEmbeddingExecutions
	ch <- c.semanticFieldExecutions
	ch <- c.semanticFieldChunkingExecutions
	ch <- c.textChunkingDelimiterExecutions
	ch <- c.textImageEmbeddingExecutions

	// Memory
	ch <- c.sparseMemoryUsageBytes
	ch <- c.sparseMemoryUsagePercentage
	ch <- c.clusteredPostingUsageBytes
	ch <- c.forwardIndexUsageBytes
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
		c.logger.Error("failed to fetch neural stats", "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, clusterName)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, clusterName)

	c.collectClusterMetrics(ch, stats)
	c.collectNodeMetrics(ch, stats)
}

func (c *Collector) fetchStats(ctx context.Context) (*StatsResponse, error) {
	body, err := c.client.Get(ctx, "/_plugins/_neural/stats")
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

	ch <- prometheus.MustNewConstMetric(c.nodesTotal, prometheus.GaugeValue, float64(stats.Nodes.Total), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesSuccessful, prometheus.GaugeValue, float64(stats.Nodes.Successful), cluster)
	ch <- prometheus.MustNewConstMetric(c.nodesFailed, prometheus.GaugeValue, float64(stats.Nodes.Failed), cluster)

	// Info: Ingest processors
	ingest := stats.Info.Processors.Ingest
	ch <- prometheus.MustNewConstMetric(c.infoSparseEncodingProcessors, prometheus.GaugeValue, float64(ingest.SparseEncodingProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoSkipExistingProcessors, prometheus.GaugeValue, float64(ingest.SkipExistingProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextImageEmbeddingProcessors, prometheus.GaugeValue, float64(ingest.TextImageEmbeddingProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextChunkingDelimiterProcessors, prometheus.GaugeValue, float64(ingest.TextChunkingDelimiterProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextEmbeddingProcessorsInPipelines, prometheus.GaugeValue, float64(ingest.TextEmbeddingProcessorsInPipelines), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextChunkingFixedTokenLengthProcessors, prometheus.GaugeValue, float64(ingest.TextChunkingFixedTokenLengthProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextChunkingFixedCharLengthProcessors, prometheus.GaugeValue, float64(ingest.TextChunkingFixedCharLengthProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoTextChunkingProcessors, prometheus.GaugeValue, float64(ingest.TextChunkingProcessors), cluster)

	// Info: Search processors
	search := stats.Info.Processors.Search
	ch <- prometheus.MustNewConstMetric(c.infoRerankMLProcessors, prometheus.GaugeValue, float64(search.RerankMLProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoRerankByFieldProcessors, prometheus.GaugeValue, float64(search.RerankByFieldProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoNeuralSparseTwoPhaseProcessors, prometheus.GaugeValue, float64(search.NeuralSparseTwoPhaseProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoNeuralQueryEnricherProcessors, prometheus.GaugeValue, float64(search.NeuralQueryEnricherProcessors), cluster)

	// Info: Hybrid processors
	hybrid := search.Hybrid
	ch <- prometheus.MustNewConstMetric(c.infoNormalizationProcessors, prometheus.GaugeValue, float64(hybrid.NormalizationProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoNormMinMaxProcessors, prometheus.GaugeValue, float64(hybrid.NormMinMaxProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoNormL2Processors, prometheus.GaugeValue, float64(hybrid.NormL2Processors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoNormZScoreProcessors, prometheus.GaugeValue, float64(hybrid.NormZScoreProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoCombArithmeticProcessors, prometheus.GaugeValue, float64(hybrid.CombArithmeticProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoCombGeometricProcessors, prometheus.GaugeValue, float64(hybrid.CombGeometricProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoCombHarmonicProcessors, prometheus.GaugeValue, float64(hybrid.CombHarmonicProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoRankBasedNormalizationProcessors, prometheus.GaugeValue, float64(hybrid.RankBasedNormalizationProcessors), cluster)
	ch <- prometheus.MustNewConstMetric(c.infoCombRRFProcessors, prometheus.GaugeValue, float64(hybrid.CombRRFProcessors), cluster)
}

func (c *Collector) collectNodeMetrics(ch chan<- prometheus.Metric, stats *StatsResponse) {
	cluster := stats.ClusterName

	for nodeID, node := range stats.NodeStats {
		labels := []string{cluster, nodeID}

		// Query metrics
		ch <- prometheus.MustNewConstMetric(c.hybridQueryRequests, prometheus.CounterValue, float64(node.Query.Hybrid.HybridQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.hybridQueryWithPaginationRequests, prometheus.CounterValue, float64(node.Query.Hybrid.HybridQueryWithPaginationRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.hybridQueryWithFilterRequests, prometheus.CounterValue, float64(node.Query.Hybrid.HybridQueryWithFilterRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.hybridQueryWithInnerHitsRequests, prometheus.CounterValue, float64(node.Query.Hybrid.HybridQueryWithInnerHitsRequests), labels...)

		ch <- prometheus.MustNewConstMetric(c.neuralQueryRequests, prometheus.CounterValue, float64(node.Query.Neural.NeuralQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.neuralQueryAgainstKNNRequests, prometheus.CounterValue, float64(node.Query.Neural.NeuralQueryAgainstKNNRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.neuralQueryAgainstSemanticDenseRequests, prometheus.CounterValue, float64(node.Query.Neural.NeuralQueryAgainstSemanticDenseRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.neuralQueryAgainstSemanticSparseRequests, prometheus.CounterValue, float64(node.Query.Neural.NeuralQueryAgainstSemanticSparseRequests), labels...)

		ch <- prometheus.MustNewConstMetric(c.neuralSparseQueryRequests, prometheus.CounterValue, float64(node.Query.NeuralSparse.NeuralSparseQueryRequests), labels...)
		ch <- prometheus.MustNewConstMetric(c.seismicQueryRequests, prometheus.CounterValue, float64(node.Query.NeuralSparse.SeismicQueryRequests), labels...)

		// Semantic highlighting
		ch <- prometheus.MustNewConstMetric(c.semanticHighlightingRequestCount, prometheus.CounterValue, float64(node.SemanticHighlighting.SemanticHighlightingRequestCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.semanticHighlightingBatchRequestCount, prometheus.CounterValue, float64(node.SemanticHighlighting.SemanticHighlightingBatchRequestCount), labels...)

		// Search processor executions
		ch <- prometheus.MustNewConstMetric(c.neuralSparseTwoPhaseExecutions, prometheus.CounterValue, float64(node.Processors.Search.NeuralSparseTwoPhaseExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.rerankByFieldExecutions, prometheus.CounterValue, float64(node.Processors.Search.RerankByFieldExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.neuralQueryEnricherExecutions, prometheus.CounterValue, float64(node.Processors.Search.NeuralQueryEnricherExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.rerankMLExecutions, prometheus.CounterValue, float64(node.Processors.Search.RerankMLExecutions), labels...)

		// Hybrid processor executions
		h := node.Processors.Search.Hybrid
		ch <- prometheus.MustNewConstMetric(c.normalizationProcessorExecutions, prometheus.CounterValue, float64(h.NormalizationProcessorExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.rankBasedNormalizationProcessorExecutions, prometheus.CounterValue, float64(h.RankBasedNormalizationProcessorExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.combHarmonicExecutions, prometheus.CounterValue, float64(h.CombHarmonicExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.normZScoreExecutions, prometheus.CounterValue, float64(h.NormZScoreExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.combRRFExecutions, prometheus.CounterValue, float64(h.CombRRFExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.normL2Executions, prometheus.CounterValue, float64(h.NormL2Executions), labels...)
		ch <- prometheus.MustNewConstMetric(c.combArithmeticExecutions, prometheus.CounterValue, float64(h.CombArithmeticExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.combGeometricExecutions, prometheus.CounterValue, float64(h.CombGeometricExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.normMinMaxExecutions, prometheus.CounterValue, float64(h.NormMinMaxExecutions), labels...)

		// Ingest processor executions
		i := node.Processors.Ingest
		ch <- prometheus.MustNewConstMetric(c.skipExistingExecutions, prometheus.CounterValue, float64(i.SkipExistingExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textChunkingFixedTokenLengthExecutions, prometheus.CounterValue, float64(i.TextChunkingFixedTokenLengthExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.sparseEncodingExecutions, prometheus.CounterValue, float64(i.SparseEncodingExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textChunkingFixedCharLengthExecutions, prometheus.CounterValue, float64(i.TextChunkingFixedCharLengthExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textChunkingExecutions, prometheus.CounterValue, float64(i.TextChunkingExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textEmbeddingExecutions, prometheus.CounterValue, float64(i.TextEmbeddingExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.semanticFieldExecutions, prometheus.CounterValue, float64(i.SemanticFieldExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.semanticFieldChunkingExecutions, prometheus.CounterValue, float64(i.SemanticFieldChunkingExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textChunkingDelimiterExecutions, prometheus.CounterValue, float64(i.TextChunkingDelimiterExecutions), labels...)
		ch <- prometheus.MustNewConstMetric(c.textImageEmbeddingExecutions, prometheus.CounterValue, float64(i.TextImageEmbeddingExecutions), labels...)

		// Memory (convert KB to bytes)
		m := node.Memory.Sparse
		ch <- prometheus.MustNewConstMetric(c.sparseMemoryUsageBytes, prometheus.GaugeValue, m.SparseMemoryUsage*1024, labels...)
		ch <- prometheus.MustNewConstMetric(c.sparseMemoryUsagePercentage, prometheus.GaugeValue, m.SparseMemoryUsagePercentage, labels...)
		ch <- prometheus.MustNewConstMetric(c.clusteredPostingUsageBytes, prometheus.GaugeValue, m.ClusteredPostingUsage*1024, labels...)
		ch <- prometheus.MustNewConstMetric(c.forwardIndexUsageBytes, prometheus.GaugeValue, m.ForwardIndexUsage*1024, labels...)
	}
}
