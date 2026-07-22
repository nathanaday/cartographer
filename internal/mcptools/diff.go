package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/diff"
)

type EntityRef struct {
	Project string `json:"project" jsonschema:"the project slug"`
	Name    string `json:"name" jsonschema:"the entity name"`
}

type DiffEntitiesInput struct {
	A    EntityRef `json:"a" jsonschema:"the first entity"`
	B    EntityRef `json:"b" jsonschema:"the second entity"`
	Kind string    `json:"kind,omitempty" jsonschema:"optional: restrict field-level diff to this kind"`
}

func (h *Handlers) DiffEntities(ctx context.Context, req *mcp.CallToolRequest, in DiffEntitiesInput) (*mcp.CallToolResult, diff.Result, error) {
	ea, err := h.Store.GetEntity(in.A.Project, in.A.Name)
	if err != nil {
		return nil, diff.Result{}, err
	}
	eb, err := h.Store.GetEntity(in.B.Project, in.B.Name)
	if err != nil {
		return nil, diff.Result{}, err
	}
	return nil, diff.Entities(ea, eb, in.Kind), nil
}
