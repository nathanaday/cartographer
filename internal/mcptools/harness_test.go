package mcptools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/store"
)

// newSession spins up an in-process MCP server backed by a fresh temp store and
// returns a connected client session.
func newSession(t *testing.T) (*mcp.ClientSession, *store.Store) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(store.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "cartographer", Version: "test"}, nil)
	Register(server, st)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session, st
}
