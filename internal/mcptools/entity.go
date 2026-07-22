package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/model"
)

type RegisterEntityInput struct {
	Project     string `json:"project" jsonschema:"the project slug, e.g. edge-app"`
	Name        string `json:"name" jsonschema:"the entity name, e.g. SensorReading"`
	Description string `json:"description,omitempty" jsonschema:"optional human description of the entity"`
}

type RegisterEntityOutput struct {
	Project string `json:"project"`
	Name    string `json:"name"`
}

func (h *Handlers) RegisterEntity(ctx context.Context, req *mcp.CallToolRequest, in RegisterEntityInput) (*mcp.CallToolResult, RegisterEntityOutput, error) {
	if err := h.Store.RegisterEntity(in.Project, in.Name, in.Description); err != nil {
		return nil, RegisterEntityOutput{}, err
	}
	return nil, RegisterEntityOutput{Project: in.Project, Name: in.Name}, nil
}

type GetEntityInput struct {
	Project string `json:"project" jsonschema:"the project slug"`
	Name    string `json:"name" jsonschema:"the entity name"`
}

func (h *Handlers) GetEntity(ctx context.Context, req *mcp.CallToolRequest, in GetEntityInput) (*mcp.CallToolResult, model.Entity, error) {
	e, err := h.Store.GetEntity(in.Project, in.Name)
	if err != nil {
		return nil, model.Entity{}, err
	}
	return nil, e, nil
}
