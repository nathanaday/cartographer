package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/contract"
	"github.com/nathanaday/cartographer/internal/model"
)

type ExportContractInput struct {
	Project string `json:"project" jsonschema:"the project slug"`
	Entity  string `json:"entity,omitempty" jsonschema:"optional entity name; omit to export the whole project"`
}

type ExportContractOutput struct {
	YAML string `json:"yaml"`
}

func (h *Handlers) ExportContract(ctx context.Context, req *mcp.CallToolRequest, in ExportContractInput) (*mcp.CallToolResult, ExportContractOutput, error) {
	if in.Entity != "" {
		e, err := h.Store.GetEntity(in.Project, in.Entity)
		if err != nil {
			return nil, ExportContractOutput{}, err
		}
		data, err := contract.Entity(e)
		if err != nil {
			return nil, ExportContractOutput{}, err
		}
		return nil, ExportContractOutput{YAML: string(data)}, nil
	}
	summaries := h.Store.ListEntities(in.Project, "")
	ents := make([]model.Entity, 0, len(summaries))
	for _, s := range summaries {
		e, err := h.Store.GetEntity(s.Project, s.Name)
		if err != nil {
			return nil, ExportContractOutput{}, err
		}
		ents = append(ents, e)
	}
	data, err := contract.Project(in.Project, ents)
	if err != nil {
		return nil, ExportContractOutput{}, err
	}
	return nil, ExportContractOutput{YAML: string(data)}, nil
}
