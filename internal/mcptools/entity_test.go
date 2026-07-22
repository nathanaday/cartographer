package mcptools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterEntityTool(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "SensorReading", "description": "a reading"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if _, err := st.GetEntity("edge-app", "SensorReading"); err != nil {
		t.Fatalf("entity not stored: %v", err)
	}
}

func TestGetEntityTool(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"},
	})
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), "SensorReading") {
		t.Fatalf("get_entity missing name: %s", data)
	}
}

func TestGetEntityToolUnknownErrors(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "Ghost"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for unknown entity")
	}
}
