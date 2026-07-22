package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RemoveAspectInput struct {
	Project string `json:"project" jsonschema:"the project slug"`
	Entity  string `json:"entity" jsonschema:"the entity name"`
	Kind    string `json:"kind" jsonschema:"the aspect kind to remove"`
}

type RemoveResult struct {
	Removed bool `json:"removed"`
}

func (h *Handlers) RemoveAspect(ctx context.Context, req *mcp.CallToolRequest, in RemoveAspectInput) (*mcp.CallToolResult, RemoveResult, error) {
	if err := h.Store.RemoveAspect(in.Project, in.Entity, in.Kind); err != nil {
		return nil, RemoveResult{}, err
	}
	return nil, RemoveResult{Removed: true}, nil
}

type RemoveEntityInput struct {
	Project string `json:"project" jsonschema:"the project slug"`
	Name    string `json:"name" jsonschema:"the entity name"`
}

func (h *Handlers) RemoveEntity(ctx context.Context, req *mcp.CallToolRequest, in RemoveEntityInput) (*mcp.CallToolResult, RemoveResult, error) {
	if err := h.Store.RemoveEntity(in.Project, in.Name); err != nil {
		return nil, RemoveResult{}, err
	}
	return nil, RemoveResult{Removed: true}, nil
}
