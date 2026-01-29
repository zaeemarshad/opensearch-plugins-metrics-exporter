.PHONY: build test test-unit test-integration bench lint clean docker-build docker-up docker-down help

BINARY_NAME := opensearch-plugins-metrics-exporter
BUILD_DIR := bin
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/exporter

build-linux:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/exporter

test: test-unit

test-unit:
	go test -race -cover ./...

test-integration: docker-up-opensearch
	@echo "Waiting for OpenSearch to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -s http://localhost:9200/_cluster/health 2>/dev/null | grep -q "cluster_name"; then \
			echo "OpenSearch is ready"; \
			break; \
		fi; \
		sleep 2; \
	done
	go test -tags=integration -race -v ./internal/integration/...
	@$(MAKE) docker-down

test-all: test-unit test-integration

bench:
	go test -bench=. -benchmem ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt:
	gofmt -s -w $(GO_FILES)

vet:
	go vet ./...

tidy:
	go mod tidy

docker-build:
	docker build -t $(BINARY_NAME):latest .

docker-up:
	docker compose up -d

docker-up-opensearch:
	docker compose up -d opensearch

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

run-dev: build
	OPENSEARCH_URL=http://localhost:9200 LOG_LEVEL=debug ./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

help:
	@echo "Available targets:"
	@echo "  build             - Build the binary"
	@echo "  build-linux       - Build for Linux amd64"
	@echo "  test              - Run unit tests (alias for test-unit)"
	@echo "  test-unit         - Run unit tests with race detection"
	@echo "  test-integration  - Run integration tests (starts OpenSearch)"
	@echo "  test-all          - Run all tests"
	@echo "  bench             - Run benchmark tests"
	@echo "  coverage          - Generate coverage report"
	@echo "  lint              - Run golangci-lint"
	@echo "  fmt               - Format Go files"
	@echo "  vet               - Run go vet"
	@echo "  tidy              - Run go mod tidy"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-up         - Start all services"
	@echo "  docker-up-opensearch - Start OpenSearch only"
	@echo "  docker-down       - Stop all services"
	@echo "  docker-logs       - Follow Docker logs"
	@echo "  run               - Build and run locally"
	@echo "  run-dev           - Build and run with dev settings"
	@echo "  clean             - Remove build artifacts"
