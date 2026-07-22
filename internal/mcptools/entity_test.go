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

func TestRegisterEntityWithInlineAspects(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "register_entity",
		Arguments: map[string]any{
			"project": "edge-app", "name": "VideoClip", "description": "a clip",
			"aspects": []map[string]any{
				{"kind": "cpp_struct", "raw": "struct VideoClip {};", "source_ref": "video_clip.hpp",
					"fields": []map[string]any{{"name": "path", "type": "std::string", "nullable": false}}},
				{"kind": "sql", "raw": "CREATE TABLE video_clips ();",
					"fields": []map[string]any{{"name": "path", "type": "TEXT", "nullable": false}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	e, err := st.GetEntity("edge-app", "VideoClip")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(e.Aspects) != 2 {
		t.Fatalf("want 2 aspects, got %d", len(e.Aspects))
	}
	cpp, ok := e.Aspect("cpp_struct")
	if !ok || cpp.Raw != "struct VideoClip {};" || cpp.SourceRef != "video_clip.hpp" ||
		len(cpp.Fields) != 1 || cpp.Fields[0].Type != "std::string" {
		t.Fatalf("cpp aspect wrong: %+v", cpp)
	}
}

func TestRegisterEntityInlineAspectReplacesByKind(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	reg := func(raw string) {
		_, _ = session.CallTool(ctx, &mcp.CallToolParams{
			Name: "register_entity",
			Arguments: map[string]any{
				"project": "edge-app", "name": "VideoClip",
				"aspects": []map[string]any{{"kind": "sql", "raw": raw}},
			},
		})
	}
	reg("v1")
	reg("v2")
	e, _ := st.GetEntity("edge-app", "VideoClip")
	a, _ := e.Aspect("sql")
	if len(e.Aspects) != 1 || a.Raw != "v2" {
		t.Fatalf("replace by kind failed: %+v", e.Aspects)
	}
}

func TestRegisterEntityWithoutAspectsUnchangedBehavior(t *testing.T) {
	ctx := context.Background()
	session, st := newSession(t)
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "VideoClip"},
	})
	// re-register with an aspect, then again WITHOUT aspects: aspect must survive
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "VideoClip",
			"aspects": []map[string]any{{"kind": "sql", "raw": "x"}}},
	})
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "VideoClip", "description": "d2"},
	})
	e, _ := st.GetEntity("edge-app", "VideoClip")
	if len(e.Aspects) != 1 || e.Description != "d2" {
		t.Fatalf("omitting aspects must preserve existing ones: %+v", e)
	}
}

func TestRegisterEntityInlineAspectEmptyKindErrors(t *testing.T) {
	ctx := context.Background()
	session, _ := newSession(t)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "register_entity",
		Arguments: map[string]any{"project": "edge-app", "name": "VideoClip",
			"aspects": []map[string]any{{"kind": "", "raw": "x"}}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("empty kind must produce an error result")
	}
}
