package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/store"
)

type ListEntitiesInput struct {
	Project string `json:"project,omitempty" jsonschema:"optional project slug to filter to"`
	Kind    string `json:"kind,omitempty" jsonschema:"optional aspect kind; only entities that have this kind are returned"`
}

type ListEntitiesOutput struct {
	Entities []store.EntitySummary `json:"entities"`
}

func (h *Handlers) ListEntities(ctx context.Context, req *mcp.CallToolRequest, in ListEntitiesInput) (*mcp.CallToolResult, ListEntitiesOutput, error) {
	return nil, ListEntitiesOutput{Entities: h.Store.ListEntities(in.Project, in.Kind)}, nil
}
