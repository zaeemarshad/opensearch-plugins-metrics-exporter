#!/bin/bash
set -e

OPENSEARCH_URL="${OPENSEARCH_URL:-http://localhost:9200}"
EXPORTER_URL="${EXPORTER_URL:-http://localhost:9206}"

echo "=== OpenSearch Plugins Metrics Exporter Integration Test ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() {
    echo -e "${GREEN}✓ $1${NC}"
}

fail() {
    echo -e "${RED}✗ $1${NC}"
    exit 1
}

warn() {
    echo -e "${YELLOW}! $1${NC}"
}

info() {
    echo -e "  $1"
}

# Wait for OpenSearch to be ready
echo "1. Waiting for OpenSearch to be ready..."
for i in {1..30}; do
    if curl -s "$OPENSEARCH_URL/_cluster/health" > /dev/null 2>&1; then
        pass "OpenSearch is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        fail "OpenSearch did not become ready in time"
    fi
    sleep 2
done

# Get cluster info
echo ""
echo "2. Checking OpenSearch cluster info..."
CLUSTER_INFO=$(curl -s "$OPENSEARCH_URL")
VERSION=$(echo "$CLUSTER_INFO" | grep -o '"number" : "[^"]*"' | head -1 | cut -d'"' -f4)
CLUSTER_NAME=$(echo "$CLUSTER_INFO" | grep -o '"cluster_name" : "[^"]*"' | cut -d'"' -f4)
info "Cluster: $CLUSTER_NAME"
info "Version: $VERSION"
pass "Cluster info retrieved"

# Check k-NN plugin
echo ""
echo "3. Checking k-NN plugin..."
KNN_STATS=$(curl -s "$OPENSEARCH_URL/_plugins/_knn/stats" 2>&1)
if echo "$KNN_STATS" | grep -q "cluster_name"; then
    pass "k-NN plugin is available"
else
    warn "k-NN plugin stats not available (may need to create an index first)"
fi

# Create a test k-NN index
echo ""
echo "4. Creating test k-NN index..."
curl -s -X PUT "$OPENSEARCH_URL/test-knn-index" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "index": {
        "knn": true,
        "number_of_shards": 1,
        "number_of_replicas": 0
      }
    },
    "mappings": {
      "properties": {
        "embedding": {
          "type": "knn_vector",
          "dimension": 3,
          "method": {
            "name": "hnsw",
            "space_type": "l2",
            "engine": "lucene"
          }
        },
        "text": {
          "type": "text"
        }
      }
    }
  }' > /dev/null 2>&1

# Index some test documents
curl -s -X POST "$OPENSEARCH_URL/test-knn-index/_doc/1" \
  -H "Content-Type: application/json" \
  -d '{"embedding": [1.0, 2.0, 3.0], "text": "test document 1"}' > /dev/null 2>&1

curl -s -X POST "$OPENSEARCH_URL/test-knn-index/_doc/2" \
  -H "Content-Type: application/json" \
  -d '{"embedding": [4.0, 5.0, 6.0], "text": "test document 2"}' > /dev/null 2>&1

# Refresh index
curl -s -X POST "$OPENSEARCH_URL/test-knn-index/_refresh" > /dev/null 2>&1
pass "Created and populated test-knn-index"

# Run a k-NN query to generate stats
echo ""
echo "5. Running k-NN query to generate stats..."
curl -s -X POST "$OPENSEARCH_URL/test-knn-index/_search" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "knn": {
        "embedding": {
          "vector": [1.0, 2.0, 3.0],
          "k": 2
        }
      }
    }
  }' > /dev/null 2>&1
pass "k-NN query executed"

# Verify k-NN stats are now available
echo ""
echo "6. Verifying k-NN stats..."
KNN_STATS=$(curl -s "$OPENSEARCH_URL/_plugins/_knn/stats")
if echo "$KNN_STATS" | grep -q "knn_query_requests"; then
    KNN_QUERIES=$(echo "$KNN_STATS" | grep -o '"knn_query_requests":[0-9]*' | head -1 | cut -d':' -f2)
    info "k-NN queries recorded: $KNN_QUERIES"
    pass "k-NN stats are working"
else
    warn "k-NN query stats not yet available"
fi

# Enable Neural Search stats (if not already enabled)
echo ""
echo "7. Enabling Neural Search stats..."
curl -s -X PUT "$OPENSEARCH_URL/_cluster/settings" \
  -H "Content-Type: application/json" \
  -d '{
    "persistent": {
      "plugins.neural_search.stats_enabled": true
    }
  }' > /dev/null 2>&1
pass "Neural Search stats enabled"

# Check Neural Search stats
echo ""
echo "8. Checking Neural Search stats..."
NEURAL_STATS=$(curl -s "$OPENSEARCH_URL/_plugins/_neural/stats" 2>&1)
if echo "$NEURAL_STATS" | grep -q "cluster_name"; then
    pass "Neural Search stats are available"
else
    warn "Neural Search stats not available (plugin may not be installed)"
fi

# Wait for exporter to be ready
echo ""
echo "9. Waiting for exporter to be ready..."
for i in {1..15}; do
    if curl -s "$EXPORTER_URL/health" > /dev/null 2>&1; then
        pass "Exporter is ready"
        break
    fi
    if [ $i -eq 15 ]; then
        fail "Exporter did not become ready in time"
    fi
    sleep 1
done

# Fetch and validate metrics
echo ""
echo "10. Fetching metrics from exporter..."
METRICS=$(curl -s "$EXPORTER_URL/metrics")

# Validate k-NN metrics
echo ""
echo "11. Validating k-NN metrics..."
if echo "$METRICS" | grep -q "opensearch_knn_up"; then
    KNN_UP=$(echo "$METRICS" | grep "^opensearch_knn_up{" | grep -o '[0-9.]*$')
    if [ "$KNN_UP" = "1" ]; then
        pass "opensearch_knn_up = 1 (scrape successful)"
    else
        warn "opensearch_knn_up = $KNN_UP"
    fi
else
    fail "opensearch_knn_up metric not found"
fi

if echo "$METRICS" | grep -q "opensearch_knn_graph_memory_usage_bytes"; then
    pass "opensearch_knn_graph_memory_usage_bytes present"
else
    warn "opensearch_knn_graph_memory_usage_bytes not found"
fi

if echo "$METRICS" | grep -q "opensearch_knn_knn_query_requests_total"; then
    pass "opensearch_knn_knn_query_requests_total present"
else
    warn "opensearch_knn_knn_query_requests_total not found"
fi

if echo "$METRICS" | grep -q "opensearch_knn_indices_in_cache_graph_count"; then
    pass "opensearch_knn_indices_in_cache_graph_count present (per-index metrics)"
else
    warn "opensearch_knn_indices_in_cache_graph_count not found"
fi

# Validate Neural metrics
echo ""
echo "12. Validating Neural Search metrics..."
if echo "$METRICS" | grep -q "opensearch_neural_up"; then
    NEURAL_UP=$(echo "$METRICS" | grep "^opensearch_neural_up{" | grep -o '[0-9.]*$')
    if [ "$NEURAL_UP" = "1" ]; then
        pass "opensearch_neural_up = 1 (scrape successful)"
    else
        warn "opensearch_neural_up = $NEURAL_UP (Neural Search may not be installed or stats disabled)"
    fi
else
    warn "opensearch_neural_up metric not found"
fi

if echo "$METRICS" | grep -q "opensearch_neural_"; then
    NEURAL_COUNT=$(echo "$METRICS" | grep -c "^opensearch_neural_" || true)
    info "Found $NEURAL_COUNT Neural Search metric lines"
else
    warn "No Neural Search metrics found"
fi

# Summary
echo ""
echo "=== Test Summary ==="
TOTAL_KNN=$(echo "$METRICS" | grep -c "^opensearch_knn_" || true)
TOTAL_NEURAL=$(echo "$METRICS" | grep -c "^opensearch_neural_" || true)
info "Total k-NN metric lines: $TOTAL_KNN"
info "Total Neural Search metric lines: $TOTAL_NEURAL"
echo ""
pass "Integration tests completed!"

# Cleanup (optional)
echo ""
echo "Cleaning up test index..."
curl -s -X DELETE "$OPENSEARCH_URL/test-knn-index" > /dev/null 2>&1
pass "Cleanup complete"
