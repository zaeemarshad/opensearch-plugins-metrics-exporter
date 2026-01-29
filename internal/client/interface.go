// Package client provides HTTP client functionality for communicating with OpenSearch.
package client

import "context"

// HTTPClient defines the interface for making HTTP requests to OpenSearch.
// This interface enables dependency injection and simplifies testing by allowing
// mock implementations to be used in place of the concrete Client.
type HTTPClient interface {
	// Get performs an HTTP GET request to the specified path and returns the response body.
	// The path is appended to the base URL configured in the client.
	// Retries are handled internally according to the client's retry configuration.
	Get(ctx context.Context, path string) ([]byte, error)

	// Close releases any resources held by the client, such as idle connections.
	Close()
}

// Ensure Client implements HTTPClient at compile time.
var _ HTTPClient = (*Client)(nil)
