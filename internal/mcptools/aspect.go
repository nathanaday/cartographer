package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/model"
)

type FieldInput struct {
	Name       string   `json:"name" jsonschema:"the field name"`
	Type       string   `json:"type" jsonschema:"the type as written in the source, e.g. VARCHAR(255) or Optional[int]"`
	Nullable   bool     `json:"nullable,omitempty" jsonschema:"whether the field may be null/absent"`
	Tags       []string `json:"tags,omitempty" jsonschema:"free-form tags such as primary_key, foreign_key, unique"`
	References string   `json:"references,omitempty" jsonschema:"for a foreign key, the target as Entity.field"`
	Note       string   `json:"note,omitempty" jsonschema:"optional note"`
}

type SetAspectInput struct {
	Project   string       `json:"project" jsonschema:"the project slug"`
	Entity    string       `json:"entity" jsonschema:"the entity name; it must already be registered"`
	Kind      string       `json:"kind" jsonschema:"the representation kind, e.g. sql, json, protobuf, pydantic"`
	Raw       string       `json:"raw" jsonschema:"the verbatim source text of this representation"`
	SourceRef string       `json:"source_ref,omitempty" jsonschema:"optional provenance, e.g. db/schema.sql:42"`
	Fields    []FieldInput `json:"fields,omitempty" jsonschema:"normalized fields extracted from the source"`
}

type SetAspectOutput struct {
	Project string `json:"project"`
	Entity  string `json:"entity"`
	Kind    string `json:"kind"`
}

func (h *Handlers) SetAspect(ctx context.Context, req *mcp.CallToolRequest, in SetAspectInput) (*mcp.CallToolResult, SetAspectOutput, error) {
	fields := make([]model.Field, 0, len(in.Fields))
	for _, f := range in.Fields {
		fields = append(fields, model.Field{
			Name: f.Name, Type: f.Type, Nullable: f.Nullable,
			Tags: f.Tags, References: f.References, Note: f.Note,
		})
	}
	aspect := model.Aspect{Kind: in.Kind, Raw: in.Raw, SourceRef: in.SourceRef, Fields: fields}
	if err := h.Store.SetAspect(in.Project, in.Entity, aspect); err != nil {
		return nil, SetAspectOutput{}, err
	}
	return nil, SetAspectOutput{Project: in.Project, Entity: in.Entity, Kind: in.Kind}, nil
}
