package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/consent"
	"github.com/nathanaday/cartographer/internal/testvault"
)

var now = time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)

type client struct {
	t *testing.T
	s *mcp.ClientSession
}

func connect(t *testing.T) (*client, string) {
	t.Helper()
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	cfg, _ := config.Load()
	cfg.AddProject(filepath.Join(root, "MyShop"))
	cfg.Save()
	srv := New(Options{Version: "test", Sim: filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"), Now: func() time.Time { return now }, Cwd: root})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	s, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t, s}, filepath.Join(root, "MyShop")
}

// call returns the structured result, or the error text when the tool failed.
func (c *client) call(tool string, args map[string]any) (map[string]any, string) {
	c.t.Helper()
	res, err := c.s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		c.t.Fatal(err)
	}
	if res.IsError {
		var msg []string
		for _, ct := range res.Content {
			if tc, ok := ct.(*mcp.TextContent); ok {
				msg = append(msg, tc.Text)
			}
		}
		return nil, strings.Join(msg, " ")
	}
	b, _ := json.Marshal(res.StructuredContent)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m, ""
}

func TestReadTools(t *testing.T) {
	c, _ := connect(t)
	out, msg := c.call("attention", map[string]any{})
	if msg != "" || !strings.Contains(out["text"].(string), "**Answer** [[Should invoice-worker adopt the Shop Data Schema]]") {
		t.Fatalf("attention: %v %s", out, msg)
	}
	out, msg = c.call("map", map[string]any{"entity": "shop-api"})
	if msg != "" || !strings.Contains(out["text"].(string), "## Obligations") || !strings.Contains(out["text"].(string), "interfaces: [[Shop REST API]]") {
		t.Fatalf("map: %v %s", out, msg)
	}
	out, msg = c.call("impact", map[string]any{"target": "Shop Data Schema#DS-4"})
	if msg != "" || len(out["hits"].([]any)) != 4 {
		t.Fatalf("impact of DS-4 should reach its 4 adopters: %v %s", out, msg)
	}
	out, msg = c.call("lint", map[string]any{})
	if msg != "" || !strings.Contains(out["text"].(string), "lint L1") {
		t.Fatalf("lint: %v %s", out, msg)
	}
}

// approve accepts only the user's own words that name the change.
func TestApproveNeedsTheUsersWords(t *testing.T) {
	c, folder := connect(t)
	out, msg := c.call("propose", map[string]any{"title": "invoice-worker adopts the schema",
		"ops": []any{map[string]any{"op": "add", "doc": "invoice-worker", "field": "adopts", "value": "[[Shop Data Schema]]"}}})
	if msg != "" {
		t.Fatal(msg)
	}
	id := out["id"].(string)
	if _, msg := c.call("approve", map[string]any{"change": id}); !strings.Contains(msg, "has not named") {
		t.Fatalf("approve without the user's words: %q", msg)
	}
	consent.Save("s1", "Yes, approve "+id+" please", now.Add(-time.Minute))
	out, msg = c.call("approve", map[string]any{"change": id})
	if msg != "" || out["status"] != "applied" {
		t.Fatalf("approve: %v %s", out, msg)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "invoice-worker.md")), `adopts: ["[[Shop Data Schema]]"]`) {
		t.Fatal("the change did not apply")
	}
}

func TestWorkAndRecord(t *testing.T) {
	c, folder := connect(t)
	if _, msg := c.call("work", map[string]any{"action": "done", "goal": "Add currency to order lines", "task": "T4", "commit": "abc1234"}); msg != "" {
		t.Fatal(msg)
	}
	if _, msg := c.call("work", map[string]any{"action": "answer", "doc": "How do we prove API-1", "chosen": 1, "words": "go with the metric"}); !strings.Contains(msg, "not in a prompt") {
		t.Fatalf("an answer with words the user did not write: %q", msg)
	}
	out, msg := c.call("record", map[string]any{"action": "check", "report": map[string]any{
		"target": "inventory-service", "repository": "github.com/myshop/myshop-inventory", "commit": "4d5e6f7", "ref": "main",
		"rows": []any{map[string]any{"clause": "[[Shop Data Schema#DS-2]]", "rev": 1, "applies": "`unit_cost`", "result": "met",
			"evidence": "integer cents", "depends_on": []any{"myshop-inventory:inventory/stock/"}}}}})
	if msg != "" || len(out["resolved"].([]any)) != 1 {
		t.Fatalf("record: %v %s", out, msg)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Goals", "Add currency to order lines.md")), "- [x] T4 [[inventory-service]]") {
		t.Fatal("T4 is not done")
	}
}
