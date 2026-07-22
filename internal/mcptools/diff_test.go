package mcptools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDiffEntitiesTool(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	// edge has sql + protobuf; cloud has only sql
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "sql", "raw": "x"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "protobuf", "raw": "y"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "cloud-app", "name": "SensorReading"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "cloud-app", "entity": "SensorReading", "kind": "sql", "raw": "x"}})

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "diff_entities", Arguments: map[string]any{
		"a": map[string]any{"project": "edge-app", "name": "SensorReading"},
		"b": map[string]any{"project": "cloud-app", "name": "SensorReading"},
	}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), "protobuf") {
		t.Fatalf("expected protobuf in only_in_a, got %s", data)
	}
}
