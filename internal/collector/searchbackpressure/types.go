package searchbackpressure

// StatsResponse represents the node stats API response filtered to search backpressure.
// Endpoint: GET /_nodes/_local/stats/search_backpressure
type StatsResponse struct {
	Nodes       NodesInfo            `json:"_nodes"`
	ClusterName string               `json:"cluster_name"`
	NodeStats   map[string]NodeStats `json:"nodes"`
}

type NodesInfo struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

type NodeStats struct {
	Name               string                   `json:"name"`
	SearchBackpressure *SearchBackpressureStats `json:"search_backpressure"`
}

type SearchBackpressureStats struct {
	SearchTask      TaskStats `json:"search_task"`
	SearchShardTask TaskStats `json:"search_shard_task"`
	Mode            string    `json:"mode"`
}

type TaskStats struct {
	// Keyed by tracker name, e.g. heap_usage_tracker. A tracker can be null or absent.
	ResourceTrackerStats map[string]*TrackerStats `json:"resource_tracker_stats"`
	CompletionCount      int64                    `json:"completion_count"`
	CancellationStats    CancellationStats        `json:"cancellation_stats"`
}

type TrackerStats struct {
	CancellationCount int64 `json:"cancellation_count"`
	CurrentMaxBytes   int64 `json:"current_max_bytes"`
	CurrentAvgBytes   int64 `json:"current_avg_bytes"`
	RollingAvgBytes   int64 `json:"rolling_avg_bytes"`
	CurrentMaxMillis  int64 `json:"current_max_millis"`
	CurrentAvgMillis  int64 `json:"current_avg_millis"`
}

type CancellationStats struct {
	CancellationCount             int64 `json:"cancellation_count"`
	CancellationLimitReachedCount int64 `json:"cancellation_limit_reached_count"`
}
