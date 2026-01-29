# Product Requirements Document (PRD)

## OpenSearch Plugins Metrics Exporter

### Overview

OpenSearch provides various plugins (k-NN, Neural Search, etc.) that expose metrics through REST APIs but do not natively support Prometheus format. This exporter bridges that gap by fetching metrics from plugin APIs and exposing them in Prometheus format.

### Problem Statement

- OpenSearch plugins like k-NN and Neural Search have valuable operational metrics
- These metrics are only accessible via REST APIs, not in Prometheus format
- Operators need these metrics for monitoring, alerting, and capacity planning
- The existing [opensearch-prometheus-exporter](https://github.com/opensearch-project/opensearch-prometheus-exporter) is a Java plugin that runs inside OpenSearch, not a standalone exporter

### Goals

1. Provide a standalone Prometheus exporter for OpenSearch plugin metrics
2. Start with k-NN plugin support, with architecture to add more plugins
3. Support enterprise deployment requirements (TLS, authentication)
4. Be lightweight and easy to deploy (single binary, Docker image)

### Non-Goals

- Replace the existing opensearch-prometheus-exporter Java plugin
- Export core OpenSearch metrics (cluster health, indices, etc.)
- Support for OpenSearch versions older than 2.x

### Technical Requirements

#### Functional Requirements

1. **k-NN Plugin Metrics** (Phase 1)
   - Fetch metrics from `/_plugins/_knn/stats` API
   - Parse and expose all available metrics including:
     - Cluster-level: circuit breaker, model index status
     - Node-level: cache metrics, query metrics, training metrics
     - Index-level: cached indices statistics
     - Remote build: repository, client, and build stats
     - Graph stats: refresh and merge operations

2. **Authentication**
   - Support basic authentication (username/password)
   - Support TLS client certificates
   - Support skipping TLS verification for development

3. **Reliability**
   - Configurable retry logic with exponential backoff
   - Timeout handling with context cancellation
   - Graceful shutdown on SIGINT/SIGTERM

4. **Observability**
   - Expose exporter health via `/health` endpoint
   - Expose exporter readiness via `/ready` endpoint
   - Include `opensearch_knn_up` metric for scrape success
   - Include `opensearch_knn_scrape_duration_seconds` for scrape timing

#### Non-Functional Requirements

1. **Performance**
   - Single scrape should complete within 30 seconds
   - Memory footprint under 50MB
   - Support scraping clusters with 100+ nodes

2. **Deployment**
   - Single static binary (no external dependencies)
   - Multi-arch Docker image (amd64, arm64)
   - Run as non-root user in container

3. **Configuration**
   - Environment variables for containerized deployments
   - CLI flags for local development
   - Sensible defaults for all optional settings

### Metrics Format

All metrics follow Prometheus naming conventions:
- Prefix: `opensearch_<plugin>_` (e.g., `opensearch_knn_`)
- Counters end with `_total`
- Histograms use `_seconds` or `_bytes` suffixes
- Boolean values are gauges (0 or 1)

### Labels

- `cluster` - OpenSearch cluster name
- `node` - Node ID for node-level metrics
- `index` - Index name for index-level metrics
- `status` - Status value for status metrics

### Future Plugins (Phase 2+)

- Neural Search plugin metrics
- ML Commons plugin metrics
- Anomaly Detection plugin metrics

### Success Metrics

1. All k-NN stats API fields are exposed as Prometheus metrics
2. Exporter can run continuously for 30+ days without restart
3. Scrape latency < 5 seconds for typical clusters
4. Zero data loss during OpenSearch restarts (graceful degradation)
