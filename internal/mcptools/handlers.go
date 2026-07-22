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
		Description: "Register (upsert) an object in a project, creating the project on first use — and register its representations in the same call via `aspects` (each is create-or-replace by kind: raw source text plus normalized fields). An entity without aspects is an incomplete stub that renders as an empty shell on the canvas; always include the representations you found. Existing aspects of kinds you don't mention are preserved.",
	}, h.RegisterEntity)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_entity",
		Description: "Return one entity's full detail: every aspect with its kind, source_ref, raw text, and normalized fields.",
	}, h.GetEntity)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_aspect",
		Description: "Add or replace (by kind) ONE representation on an existing entity — the targeted-update tool for correcting or extending a single aspect after registration. For initial registration prefer register_entity with inline aspects.",
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
	mcp.AddTool(server, &mcp.Tool{
		Name:        "diff_entities",
		Description: "Compare two entities (each identified by project + name). Reports presence-level gaps (which aspect kinds each side has) and field-level differences within each shared kind. Use it across two projects to align the same object between environments.",
	}, h.DiffEntities)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_contract",
		Description: "Export the authority schema as YAML for one entity (pass entity) or an entire project (omit entity). The contract is the normalized shape (kinds, source_refs, fields), not a copy of raw source files.",
	}, h.ExportContract)
}
