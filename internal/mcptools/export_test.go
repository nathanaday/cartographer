package mcptools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExportContractEntity(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "sql", "raw": "x", "fields": []map[string]any{{"name": "id", "type": "INT"}}}})

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "export_contract", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), "SensorReading") {
		t.Fatalf("export missing entity: %s", data)
	}
}

func TestExportContractProject(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "export_contract", Arguments: map[string]any{"project": "edge-app"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), "edge-app") {
		t.Fatalf("project export missing name: %s", data)
	}
}
