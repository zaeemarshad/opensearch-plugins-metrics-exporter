# OpenSearch Plugins Metrics Exporter

A Prometheus exporter for OpenSearch plugin metrics. This exporter collects metrics from OpenSearch plugins that don't natively expose Prometheus metrics and makes them available for scraping.

## Supported Plugins

- **k-NN (Vector Search)** - `/_plugins/_knn/_local/stats`
- **Neural Search (Semantic/Hybrid Search)** - `/_plugins/_neural/{nodeId}/stats`

## Supported Core Metrics

- **Search Backpressure** - `/_nodes/_local/stats/search_backpressure`

## Node Scope

Every query returns stats for the node at `OPENSEARCH_URL` only. Run one exporter per node, with `OPENSEARCH_URL` set to that node. The neural stats API ignores `_local`, so the exporter gets the local node ID from `/_nodes/_local` on each scrape.

Cluster-level metrics are the same on every node, e.g. `opensearch_knn_circuit_breaker_triggered` and `opensearch_neural_info_*`. Aggregate them with `max by (cluster)`.

The exporter is tested against OpenSearch 3.8.0 and 3.9.0.

## Features

- Prometheus metrics format with `opensearch_knn_` and `opensearch_neural_` prefixes
- Configurable authentication (basic auth)
- TLS/SSL support with CA certificates and client certificates
- Retry logic with exponential backoff
- Context cancellation and graceful shutdown
- Health and readiness endpoints

## Prerequisites

### Neural Search Stats

Neural Search statistics collection is disabled by default. Enable it with:

```bash
PUT /_cluster/settings
{
  "persistent": {
    "plugins.neural_search.stats_enabled": true
  }
}
```

## Quick Start

### Using Docker

```bash
docker run -p 9206:9206 \
  -e OPENSEARCH_URL=https://your-opensearch:9200 \
  -e OPENSEARCH_USERNAME=admin \
  -e OPENSEARCH_PASSWORD=admin \
  -e OPENSEARCH_TLS_INSECURE=true \
  opensearch-plugins-metrics-exporter
```

### Using Binary

```bash
# Build
go build -o opensearch-plugins-metrics-exporter ./cmd/exporter

# Run
./opensearch-plugins-metrics-exporter \
  --url https://localhost:9200 \
  --username admin \
  --password admin \
  --tls-insecure
```

## Configuration

Configuration can be provided via environment variables or CLI flags.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENSEARCH_URL` | OpenSearch URL | `http://localhost:9200` |
| `OPENSEARCH_USERNAME` | Basic auth username | - |
| `OPENSEARCH_PASSWORD` | Basic auth password | - |
| `OPENSEARCH_TLS_INSECURE` | Skip TLS verification | `false` |
| `OPENSEARCH_TLS_CA_CERT` | Path to CA certificate | - |
| `OPENSEARCH_TLS_CLIENT_CERT` | Path to client certificate | - |
| `OPENSEARCH_TLS_CLIENT_KEY` | Path to client key | - |
| `OPENSEARCH_TIMEOUT` | Request timeout | `10s` |
| `OPENSEARCH_RETRY_COUNT` | Number of retries | `3` |
| `OPENSEARCH_RETRY_DELAY` | Delay between retries | `1s` |
| `EXPORTER_PORT` | Port to expose metrics | `9206` |
| `METRICS_PATH` | Metrics endpoint path | `/metrics` |
| `ENABLE_KNN` | Enable k-NN plugin metrics | `true` |
| `ENABLE_NEURAL` | Enable Neural Search plugin metrics | `true` |
| `ENABLE_SEARCH_BACKPRESSURE` | Enable search backpressure metrics | `true` |

### CLI Flags

```
--url string              OpenSearch URL (default "http://localhost:9200")
--username string         OpenSearch username
--password string         OpenSearch password
--tls-insecure           Skip TLS certificate verification
--tls-ca-cert string     Path to CA certificate
--tls-client-cert string Path to client certificate
--tls-client-key string  Path to client key
--timeout duration       Request timeout (default 10s)
--retry-count int        Number of retries for failed requests (default 3)
--retry-delay duration   Delay between retries (default 1s)
--port int               Port to expose metrics (default 9206)
--metrics-path string    Path to expose metrics (default "/metrics")
--log-level string       Log level: debug, info, warn, error (default "info")
--log-format string      Log format: text, json (default "text")
--enable-knn             Enable k-NN plugin metrics (default true)
--enable-neural          Enable Neural Search plugin metrics (default true)
--enable-search-backpressure  Enable search backpressure metrics (default true)
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/metrics` | Prometheus metrics |
| `/health` | Health check |
| `/ready` | Readiness check |
| `/` | Index page with links |

## Metrics

### k-NN Plugin Metrics (`opensearch_knn_*`)

#### Meta Metrics
- `opensearch_knn_up` - Whether the last scrape was successful
- `opensearch_knn_scrape_duration_seconds` - Duration of the last scrape

#### Cluster-Level Metrics
- `circuit_breaker_triggered`, `model_index_status`, `nodes_total/successful/failed`

#### Node-Level Metrics (labels: cluster, node)
- Cache: `graph_memory_usage_bytes`, `cache_capacity_reached`, `hit_count_total`, `miss_count_total`
- Query: `knn_query_requests_total`, `graph_query_requests_total`, `max_distance_query_requests_total`
- Training: `training_requests_total`, `training_memory_usage_bytes`
- Engine: `faiss_initialized`, `nmslib_initialized`, `lucene_initialized`
- Graph stats: merge and refresh operation metrics
- Remote build stats: repository, client, and build metrics
- Remote build exceptions (OpenSearch 3.9+): `remote_build_client_merge_abort_exceptions_total`, `remote_build_client_terminal_exceptions_total`

#### Index-Level Metrics (labels: cluster, node, index)
- `indices_in_cache_graph_count`, `indices_in_cache_memory_bytes`

### Neural Search Plugin Metrics (`opensearch_neural_*`)

#### Meta Metrics
- `opensearch_neural_up` - Whether the last scrape was successful (0=failed, 1=success)
- `opensearch_neural_scrape_duration_seconds` - Duration of the last scrape

#### Cluster-Level Info Metrics
Processor counts configured in pipelines:
- Ingest: `info_text_embedding_processors_in_pipelines`, `info_sparse_encoding_processors`, `info_text_chunking_processors`
- Search: `info_rerank_ml_processors`, `info_neural_query_enricher_processors`
- Agentic: `info_agentic_context_processors`, `info_agentic_query_translator_processors`
- Hybrid: `info_normalization_processors`, `info_comb_rrf_processors`, `info_norm_*_processors`

Sparse index counts (OpenSearch 3.9+):
- `info_sparse_vector_indices`, `info_sparse_vector_fields`, `info_sparse_native_engine_indices`, `info_sparse_native_engine_fields`

Metrics marked 3.9+ are emitted only when OpenSearch reports them. On 3.8 the exporter emits no series for them.

#### Node-Level Metrics (labels: cluster, node)

**Query Metrics:**
- `hybrid_query_requests_total`, `hybrid_query_with_filter_requests_total`, `hybrid_query_with_pagination_requests_total`
- `neural_query_requests_total`, `neural_query_against_knn_requests_total`, `neural_query_against_semantic_dense_requests_total`
- `neural_sparse_query_requests_total`, `seismic_query_requests_total`
- `agentic_query_requests_total`

**Processor Execution Metrics:**
- Ingest: `text_embedding_executions_total`, `sparse_encoding_executions_total`, `sparse_encoding_seismic_executions_total`, `text_chunking_executions_total`
- Search: `rerank_ml_executions_total`, `neural_query_enricher_executions_total`, `mmr_neural_query_transformer_executions_total`
- Agentic: `agentic_query_translator_executions_total`, `agentic_context_executions_total`
- Hybrid: `normalization_processor_executions_total`, `comb_rrf_executions_total`

**Memory Metrics:**
- `sparse_memory_usage_bytes`, `sparse_memory_usage_percentage`
- `clustered_posting_usage_bytes`, `forward_index_usage_bytes`

**Semantic Highlighting:**
- `semantic_highlighting_requests_total`, `semantic_highlighting_batch_requests_total`

### Search Backpressure Metrics (`opensearch_search_backpressure_*`)

#### Meta Metrics
- `opensearch_search_backpressure_up` - Whether the last scrape was successful (0=failed, 1=success)
- `opensearch_search_backpressure_scrape_duration_seconds` - Duration of the last scrape
- `nodes_total`, `nodes_successful`, `nodes_failed`

#### Node-Level Metrics

**Mode (labels: cluster, node, mode):**
- `mode` - 1 for the active mode (`disabled`, `monitor_only` or `enforced`), 0 for the other modes

**Task Metrics (labels: cluster, node, task_type):**
- `task_type` is `search_task` or `search_shard_task`
- `task_completions_total`, `task_cancellations_total`, `task_cancellation_limit_reached_total`

**Resource Tracker Metrics (labels: cluster, node, task_type, tracker):**
- `tracker` is `heap_usage`, `cpu_usage`, `elapsed_time` or `native_memory_usage`
- `tracker_cancellations_total` - all trackers
- `tracker_current_max_bytes`, `tracker_current_avg_bytes` - `heap_usage` and `native_memory_usage`
- `tracker_rolling_avg_bytes` - `heap_usage`
- `tracker_current_max_milliseconds`, `tracker_current_avg_milliseconds` - `cpu_usage` and `elapsed_time`

OpenSearch reports a disabled tracker as `null` or omits it. The exporter emits no series for that tracker.

## Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'opensearch-plugins'
    scrape_interval: 30s
    static_configs:
      - targets: ['exporter-host:9206']
```

## Building

```bash
# Build binary
go build -o opensearch-plugins-metrics-exporter ./cmd/exporter

# Build Docker image
docker build -t opensearch-plugins-metrics-exporter \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  .
```

## Testing

### Unit Tests

```bash
go test ./...
```

### Integration Tests

Run integration tests against a real OpenSearch instance. The default image is 3.9.0. CI runs the tests against 3.8.0 and 3.9.0. Set `OPENSEARCH_VERSION` to select another release, e.g. `OPENSEARCH_VERSION=3.8.0 docker compose up -d opensearch`.

```bash
# Start OpenSearch
docker compose up -d opensearch

# Wait for it to be ready, then run tests
./scripts/integration-test.sh

# Stop OpenSearch
docker compose down
```

Or run everything with docker-compose:

```bash
docker compose up -d
curl http://localhost:9206/metrics
```

## License

Apache License 2.0
