package knn

type StatsResponse struct {
	Nodes                   NodesInfo            `json:"_nodes"`
	ClusterName             string               `json:"cluster_name"`
	CircuitBreakerTriggered bool                 `json:"circuit_breaker_triggered"`
	ModelIndexStatus        *string              `json:"model_index_status"` // null, "green", "yellow", or "red"
	NodeStats               map[string]NodeStats `json:"nodes"`
}

type NodesInfo struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

type NodeStats struct {
	GraphMemoryUsage           int64   `json:"graph_memory_usage"` // KB
	GraphMemoryUsagePercentage float64 `json:"graph_memory_usage_percentage"`
	CacheCapacityReached       bool    `json:"cache_capacity_reached"`
	TotalLoadTime              int64   `json:"total_load_time"` // nanoseconds
	EvictionCount              int64   `json:"eviction_count"`
	HitCount                   int64   `json:"hit_count"`
	MissCount                  int64   `json:"miss_count"`
	LoadSuccessCount           int64   `json:"load_success_count"`
	LoadExceptionCount         int64   `json:"load_exception_count"`

	GraphQueryRequests int64 `json:"graph_query_requests"`
	GraphQueryErrors   int64 `json:"graph_query_errors"`
	GraphIndexRequests int64 `json:"graph_index_requests"`
	GraphIndexErrors   int64 `json:"graph_index_errors"`

	KNNQueryRequests                   int64 `json:"knn_query_requests"`
	KNNQueryWithFilterRequests         int64 `json:"knn_query_with_filter_requests"`
	MaxDistanceQueryRequests           int64 `json:"max_distance_query_requests"`
	MaxDistanceQueryWithFilterRequests int64 `json:"max_distance_query_with_filter_requests"`
	MinScoreQueryRequests              int64 `json:"min_score_query_requests"`
	MinScoreQueryWithFilterRequests    int64 `json:"min_score_query_with_filter_requests"`

	ScriptCompilations      int64 `json:"script_compilations"`
	ScriptCompilationErrors int64 `json:"script_compilation_errors"`
	ScriptQueryRequests     int64 `json:"script_query_requests"`
	ScriptQueryErrors       int64 `json:"script_query_errors"`

	FaissInitialized  bool `json:"faiss_initialized"`
	NmslibInitialized bool `json:"nmslib_initialized"`
	LuceneInitialized bool `json:"lucene_initialized"`

	TrainingRequests              int64   `json:"training_requests"`
	TrainingErrors                int64   `json:"training_errors"`
	TrainingMemoryUsage           int64   `json:"training_memory_usage"` // KB
	TrainingMemoryUsagePercentage float64 `json:"training_memory_usage_percentage"`
	IndexingFromModelDegraded     bool    `json:"indexing_from_model_degraded"`

	GraphStats                  GraphStats                 `json:"graph_stats"`
	IndicesInCache              map[string]IndexCacheStats `json:"indices_in_cache"`
	RemoteVectorIndexBuildStats RemoteBuildStats           `json:"remote_vector_index_build_stats"`
}

type GraphStats struct {
	Refresh RefreshStats `json:"refresh"`
	Merge   MergeStats   `json:"merge"`
}

type RefreshStats struct {
	Total             int64 `json:"total"`
	TotalTimeInMillis int64 `json:"total_time_in_millis"`
}

type MergeStats struct {
	Current            int64 `json:"current"`
	Total              int64 `json:"total"`
	TotalTimeInMillis  int64 `json:"total_time_in_millis"`
	CurrentDocs        int64 `json:"current_docs"`
	TotalDocs          int64 `json:"total_docs"`
	TotalSizeInBytes   int64 `json:"total_size_in_bytes"`
	CurrentSizeInBytes int64 `json:"current_size_in_bytes"`
}

type IndexCacheStats struct {
	GraphMemoryUsage           int64   `json:"graph_memory_usage"` // KB
	GraphMemoryUsagePercentage float64 `json:"graph_memory_usage_percentage"`
	GraphCount                 int64   `json:"graph_count"`
}

type RemoteBuildStats struct {
	RepositoryStats RepositoryStats `json:"repository_stats"`
	ClientStats     ClientStats     `json:"client_stats"`
	BuildStats      BuildStats      `json:"build_stats"`
}

type RepositoryStats struct {
	ReadSuccessCount            int64 `json:"read_success_count"`
	ReadFailureCount            int64 `json:"read_failure_count"`
	SuccessfulReadTimeInMillis  int64 `json:"successful_read_time_in_millis"`
	WriteSuccessCount           int64 `json:"write_success_count"`
	WriteFailureCount           int64 `json:"write_failure_count"`
	SuccessfulWriteTimeInMillis int64 `json:"successful_write_time_in_millis"`
}

type ClientStats struct {
	StatusRequestSuccessCount int64 `json:"status_request_success_count"`
	StatusRequestFailureCount int64 `json:"status_request_failure_count"`
	BuildRequestSuccessCount  int64 `json:"build_request_success_count"`
	BuildRequestFailureCount  int64 `json:"build_request_failure_count"`
	IndexBuildSuccessCount    int64 `json:"index_build_success_count"`
	IndexBuildFailureCount    int64 `json:"index_build_failure_count"`
	WaitingTimeInMs           int64 `json:"waiting_time_in_ms"`
}

type BuildStats struct {
	RemoteIndexBuildFlushTimeInMillis      int64 `json:"remote_index_build_flush_time_in_millis"`
	RemoteIndexBuildMergeTimeInMillis      int64 `json:"remote_index_build_merge_time_in_millis"`
	RemoteIndexBuildCurrentMergeOperations int64 `json:"remote_index_build_current_merge_operations"`
	RemoteIndexBuildCurrentFlushOperations int64 `json:"remote_index_build_current_flush_operations"`
	RemoteIndexBuildCurrentMergeSize       int64 `json:"remote_index_build_current_merge_size"`
	RemoteIndexBuildCurrentFlushSize       int64 `json:"remote_index_build_current_flush_size"`
}
