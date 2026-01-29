# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /opensearch-plugins-metrics-exporter \
    ./cmd/exporter

# Final stage
FROM alpine:3.19

# Install CA certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 exporter && \
    adduser -u 1000 -G exporter -s /bin/sh -D exporter

WORKDIR /app

# Copy binary from builder
COPY --from=builder /opensearch-plugins-metrics-exporter /app/opensearch-plugins-metrics-exporter

# Change ownership
RUN chown -R exporter:exporter /app

# Switch to non-root user
USER exporter

# Expose metrics port
EXPOSE 9206

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9206/health || exit 1

ENTRYPOINT ["/app/opensearch-plugins-metrics-exporter"]
