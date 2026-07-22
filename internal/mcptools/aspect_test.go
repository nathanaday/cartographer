package mcptools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSetAspectTool(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "SensorReading"},
	})
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "set_aspect",
		Arguments: map[string]any{
			"project":    "edge-app",
			"entity":     "SensorReading",
			"kind":       "sql",
			"raw":        "CREATE TABLE sensor_reading (device_id VARCHAR(64));",
			"source_ref": "db/schema.sql:42",
			"fields": []map[string]any{
				{"name": "device_id", "type": "VARCHAR(64)", "nullable": false, "tags": []string{"primary_key"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	e, err := st.GetEntity("edge-app", "SensorReading")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	a, ok := e.Aspect("sql")
	if !ok || len(a.Fields) != 1 || a.Fields[0].Name != "device_id" || a.SourceRef != "db/schema.sql:42" {
		t.Fatalf("aspect not stored correctly: %+v", a)
	}
}

func TestSetAspectMissingEntityErrors(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "set_aspect",
		Arguments: map[string]any{
			"project": "edge-app", "entity": "Ghost", "kind": "sql", "raw": "x",
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing entity")
	}
}
