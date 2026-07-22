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
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_aspect",
		Description: "Create or replace (by kind) one representation aspect on an existing entity: its raw source text plus normalized fields (name, type as written, nullable, tags like primary_key/foreign_key, and references for foreign keys). Replace semantics — re-registering a kind overwrites it.",
	}, h.SetAspect)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_aspect",
		Description: "Remove one representation aspect (and its raw sidecar) from an entity, so a re-scan produces an accurate map.",
	}, h.RemoveAspect)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_entity",
		Description: "Remove an entity and all its aspects from a project.",
	}, h.RemoveEntity)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_entities",
		Description: "List entities with their kinds. Optionally filter to one project and/or to entities that have a given aspect kind (e.g. kind=protobuf shows only entities with a protobuf representation).",
	}, h.ListEntities)
}
