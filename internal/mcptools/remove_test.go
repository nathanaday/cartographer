package mcptools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/store"
)

func TestRemoveAspectTool(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "set_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "sql", "raw": "x"}})
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "remove_aspect", Arguments: map[string]any{"project": "edge-app", "entity": "SensorReading", "kind": "sql"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	e, _ := st.GetEntity("edge-app", "SensorReading")
	if _, ok := e.Aspect("sql"); ok {
		t.Fatal("sql aspect should be gone")
	}
}

func TestRemoveEntityTool(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: "register_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "remove_entity", Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	if _, err := st.GetEntity("edge-app", "SensorReading"); !errors.Is(err, store.ErrEntityNotFound) {
		t.Fatalf("want ErrEntityNotFound, got %v", err)
	}
}
