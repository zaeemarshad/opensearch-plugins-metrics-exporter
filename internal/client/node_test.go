package client

import (
	"context"
	"testing"
)

type stubClient struct{ body string }

func (s stubClient) Get(_ context.Context, _ string) ([]byte, error) { return []byte(s.body), nil }
func (s stubClient) Close()                                          {}

func TestLocalNodeID(t *testing.T) {
	id, err := LocalNodeID(context.Background(), stubClient{`{"nodes":{"Z7eXl9nKRnmJP1GG22lqng":{"name":"n"}}}`})
	if err != nil || id != "Z7eXl9nKRnmJP1GG22lqng" {
		t.Errorf("expected Z7eXl9nKRnmJP1GG22lqng, got %q (err %v)", id, err)
	}
}

// The ID goes into a request path, so a response that could redirect the
// authenticated request to another endpoint must be rejected.
func TestLocalNodeIDRejectsInvalidResponses(t *testing.T) {
	for name, body := range map[string]string{
		"no nodes":       `{"nodes":{}}`,
		"multiple nodes": `{"nodes":{"Z7eXl9nKRnmJP1GG22lqng":{},"lj4nSNBxQEuQEkA8u0eAnA":{}}}`,
		"invalid json":   `not json`,
		"path traversal": `{"nodes":{"../../_cluster/settings":{}}}`,
		"query string":   `{"nodes":{"Z7eXl9nKRnmJP1GG22?q=x":{}}}`,
		"wrong length":   `{"nodes":{"abc":{}}}`,
	} {
		if id, err := LocalNodeID(context.Background(), stubClient{body}); err == nil {
			t.Errorf("%s: expected error, got ID %q", name, id)
		}
	}
}
