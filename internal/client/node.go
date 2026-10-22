package client

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// LocalNodePath is the request path that returns the local node ID.
const LocalNodePath = "/_nodes/_local?filter_path=nodes.*.name"

var nodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

// LocalNodeID returns the ID of the node the client is connected to.
// The ID is validated because callers insert it into request paths.
func LocalNodeID(ctx context.Context, c HTTPClient) (string, error) {
	body, err := c.Get(ctx, LocalNodePath)
	if err != nil {
		return "", err
	}

	var resp struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal local node response: %w", err)
	}
	if len(resp.Nodes) != 1 {
		return "", fmt.Errorf("expected 1 local node, got %d", len(resp.Nodes))
	}
	for id := range resp.Nodes {
		if !nodeIDPattern.MatchString(id) {
			return "", fmt.Errorf("invalid local node ID %q", id)
		}
		return id, nil
	}
	return "", nil
}
