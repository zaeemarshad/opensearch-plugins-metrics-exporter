package neural

// StatsResponse represents the Neural Search stats API response.
// Endpoint: GET /_plugins/_neural/stats
// Note: Stats collection must be enabled via cluster setting:
// PUT /_cluster/settings {"persistent":{"plugins.neural_search.stats_enabled":true}}
type StatsResponse struct {
	Nodes       NodesInfo                 `json:"_nodes"`
	ClusterName string                    `json:"cluster_name"`
	Info        InfoStats                 `json:"info"`
	AllNodes    NodeLevelStats            `json:"all_nodes"`
	NodeStats   map[string]NodeLevelStats `json:"nodes"`
}

type NodesInfo struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

type InfoStats struct {
	ClusterVersion string          `json:"cluster_version"`
	Processors     ProcessorCounts `json:"processors"`
}

type ProcessorCounts struct {
	Search SearchProcessorCounts `json:"search"`
	Ingest IngestProcessorCounts `json:"ingest"`
}

type SearchProcessorCounts struct {
	Hybrid                         HybridProcessorCounts `json:"hybrid"`
	RerankMLProcessors             int64                 `json:"rerank_ml_processors"`
	RerankByFieldProcessors        int64                 `json:"rerank_by_field_processors"`
	NeuralSparseTwoPhaseProcessors int64                 `json:"neural_sparse_two_phase_processors"`
	NeuralQueryEnricherProcessors  int64                 `json:"neural_query_enricher_processors"`
}

type HybridProcessorCounts struct {
	CombGeometricProcessors          int64 `json:"comb_geometric_processors"`
	CombRRFProcessors                int64 `json:"comb_rrf_processors"`
	NormL2Processors                 int64 `json:"norm_l2_processors"`
	NormMinMaxProcessors             int64 `json:"norm_minmax_processors"`
	CombHarmonicProcessors           int64 `json:"comb_harmonic_processors"`
	CombArithmeticProcessors         int64 `json:"comb_arithmetic_processors"`
	NormZScoreProcessors             int64 `json:"norm_zscore_processors"`
	RankBasedNormalizationProcessors int64 `json:"rank_based_normalization_processors"`
	NormalizationProcessors          int64 `json:"normalization_processors"`
}

type IngestProcessorCounts struct {
	SparseEncodingProcessors               int64 `json:"sparse_encoding_processors"`
	SkipExistingProcessors                 int64 `json:"skip_existing_processors"`
	TextImageEmbeddingProcessors           int64 `json:"text_image_embedding_processors"`
	TextChunkingDelimiterProcessors        int64 `json:"text_chunking_delimiter_processors"`
	TextEmbeddingProcessorsInPipelines     int64 `json:"text_embedding_processors_in_pipelines"`
	TextChunkingFixedTokenLengthProcessors int64 `json:"text_chunking_fixed_token_length_processors"`
	TextChunkingFixedCharLengthProcessors  int64 `json:"text_chunking_fixed_char_length_processors"`
	TextChunkingProcessors                 int64 `json:"text_chunking_processors"`
}

type NodeLevelStats struct {
	Query                QueryStats                `json:"query"`
	SemanticHighlighting SemanticHighlightingStats `json:"semantic_highlighting"`
	Processors           ProcessorExecutions       `json:"processors"`
	Memory               MemoryStats               `json:"memory"`
}

type QueryStats struct {
	Hybrid       HybridQueryStats       `json:"hybrid"`
	Neural       NeuralQueryStats       `json:"neural"`
	NeuralSparse NeuralSparseQueryStats `json:"neural_sparse"`
}

type HybridQueryStats struct {
	HybridQueryWithPaginationRequests int64 `json:"hybrid_query_with_pagination_requests"`
	HybridQueryWithFilterRequests     int64 `json:"hybrid_query_with_filter_requests"`
	HybridQueryWithInnerHitsRequests  int64 `json:"hybrid_query_with_inner_hits_requests"`
	HybridQueryRequests               int64 `json:"hybrid_query_requests"`
}

type NeuralQueryStats struct {
	NeuralQueryAgainstSemanticSparseRequests int64 `json:"neural_query_against_semantic_sparse_requests"`
	NeuralQueryRequests                      int64 `json:"neural_query_requests"`
	NeuralQueryAgainstSemanticDenseRequests  int64 `json:"neural_query_against_semantic_dense_requests"`
	NeuralQueryAgainstKNNRequests            int64 `json:"neural_query_against_knn_requests"`
}

type NeuralSparseQueryStats struct {
	NeuralSparseQueryRequests int64 `json:"neural_sparse_query_requests"`
	SeismicQueryRequests      int64 `json:"seismic_query_requests"`
}

type SemanticHighlightingStats struct {
	SemanticHighlightingRequestCount      int64 `json:"semantic_highlighting_request_count"`
	SemanticHighlightingBatchRequestCount int64 `json:"semantic_highlighting_batch_request_count"`
}

type ProcessorExecutions struct {
	Search SearchProcessorExecutions `json:"search"`
	Ingest IngestProcessorExecutions `json:"ingest"`
}

type SearchProcessorExecutions struct {
	NeuralSparseTwoPhaseExecutions int64                     `json:"neural_sparse_two_phase_executions"`
	Hybrid                         HybridProcessorExecutions `json:"hybrid"`
	RerankByFieldExecutions        int64                     `json:"rerank_by_field_executions"`
	NeuralQueryEnricherExecutions  int64                     `json:"neural_query_enricher_executions"`
	RerankMLExecutions             int64                     `json:"rerank_ml_executions"`
}

type HybridProcessorExecutions struct {
	CombHarmonicExecutions                    int64 `json:"comb_harmonic_executions"`
	NormZScoreExecutions                      int64 `json:"norm_zscore_executions"`
	CombRRFExecutions                         int64 `json:"comb_rrf_executions"`
	NormL2Executions                          int64 `json:"norm_l2_executions"`
	RankBasedNormalizationProcessorExecutions int64 `json:"rank_based_normalization_processor_executions"`
	CombArithmeticExecutions                  int64 `json:"comb_arithmetic_executions"`
	NormalizationProcessorExecutions          int64 `json:"normalization_processor_executions"`
	CombGeometricExecutions                   int64 `json:"comb_geometric_executions"`
	NormMinMaxExecutions                      int64 `json:"norm_minmax_executions"`
}

type IngestProcessorExecutions struct {
	SkipExistingExecutions                 int64 `json:"skip_existing_executions"`
	TextChunkingFixedTokenLengthExecutions int64 `json:"text_chunking_fixed_token_length_executions"`
	SparseEncodingExecutions               int64 `json:"sparse_encoding_executions"`
	TextChunkingFixedCharLengthExecutions  int64 `json:"text_chunking_fixed_char_length_executions"`
	TextChunkingExecutions                 int64 `json:"text_chunking_executions"`
	TextEmbeddingExecutions                int64 `json:"text_embedding_executions"`
	SemanticFieldExecutions                int64 `json:"semantic_field_executions"`
	SemanticFieldChunkingExecutions        int64 `json:"semantic_field_chunking_executions"`
	TextChunkingDelimiterExecutions        int64 `json:"text_chunking_delimiter_executions"`
	TextImageEmbeddingExecutions           int64 `json:"text_image_embedding_executions"`
}

type MemoryStats struct {
	Sparse SparseMemoryStats `json:"sparse"`
}

type SparseMemoryStats struct {
	SparseMemoryUsagePercentage float64 `json:"sparse_memory_usage_percentage"` // node-level only
	SparseMemoryUsage           float64 `json:"sparse_memory_usage"`            // KB
	ClusteredPostingUsage       float64 `json:"clustered_posting_usage"`        // KB
	ForwardIndexUsage           float64 `json:"forward_index_usage"`            // KB
}
