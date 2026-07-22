// Package mcptools adapts the store into MCP tools. Handlers are methods on
// Handlers, which closes over a single *store.Store loaded once at startup.
package mcptools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/store"
)

// Handlers holds the shared store the tool handlers operate on.
type Handlers struct {
	Store *store.Store
}

// Register wires every Cartographer tool onto the server.
func Register(server *mcp.Server, st *store.Store) {
	h := &Handlers{Store: st}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "register_entity",
		Description: "Upsert an entity shell in a project (creating the project on first use). Existing aspects are preserved; only the description is updated. Call this before set_aspect.",
	}, h.RegisterEntity)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_entity",
		Description: "Return one entity's full detail: every aspect with its kind, source_ref, raw text, and normalized fields.",
	}, h.GetEntity)
}
