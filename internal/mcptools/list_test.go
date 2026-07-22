package mcptools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListEntitiesTool(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "protobuf", "raw": "message X {}"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "DeviceConfig"}})

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_entities", Arguments: map[string]any{"kind": "protobuf"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	data, _ := json.Marshal(res)
	s := string(data)
	if !strings.Contains(s, "SensorReading") || strings.Contains(s, "DeviceConfig") {
		t.Fatalf("kind filter wrong, got %s", s)
	}
}
