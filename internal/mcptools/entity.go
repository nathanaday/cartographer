package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/model"
)

// InlineAspectInput is one representation registered together with the entity.
type InlineAspectInput struct {
	Kind      string       `json:"kind" jsonschema:"the representation kind, e.g. sql, json, protobuf, cpp_struct, pydantic"`
	Raw       string       `json:"raw" jsonschema:"the verbatim source text of this representation"`
	SourceRef string       `json:"source_ref,omitempty" jsonschema:"optional provenance, e.g. db/schema.sql:42"`
	Fields    []FieldInput `json:"fields,omitempty" jsonschema:"normalized fields extracted from the source"`
}

type RegisterEntityInput struct {
	Project     string              `json:"project" jsonschema:"the project slug, e.g. edge-app"`
	Name        string              `json:"name" jsonschema:"the entity name, e.g. SensorReading"`
	Description string              `json:"description,omitempty" jsonschema:"optional human description of the entity"`
	Aspects     []InlineAspectInput `json:"aspects,omitempty" jsonschema:"the object's representations, registered in the same call; each is create-or-replace by kind"`
}

type RegisterEntityOutput struct {
	Project string `json:"project"`
	Name    string `json:"name"`
}

func (h *Handlers) RegisterEntity(ctx context.Context, req *mcp.CallToolRequest, in RegisterEntityInput) (*mcp.CallToolResult, RegisterEntityOutput, error) {
	if err := h.Store.RegisterEntity(in.Project, in.Name, in.Description); err != nil {
		return nil, RegisterEntityOutput{}, err
	}
	for _, a := range in.Aspects {
		aspect := model.Aspect{Kind: a.Kind, Raw: a.Raw, SourceRef: a.SourceRef, Fields: fieldsToModel(a.Fields)}
		if err := h.Store.SetAspect(in.Project, in.Name, aspect); err != nil {
			return nil, RegisterEntityOutput{}, err // aspects apply in order; first error returned
		}
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
