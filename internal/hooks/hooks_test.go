package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/testvault"
)

type env struct {
	project, clone, sim string
	o                   Options
}

// setup makes a vault, a clone whose origin is MyShop's myshop-api, and machine settings
// that know both.
func setup(t *testing.T) env {
	t.Helper()
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	clone := filepath.Join(t.TempDir(), "myshop-api")
	os.MkdirAll(filepath.Join(clone, "server", "orders"), 0o755)
	testvault.Git(t, clone, "init", "-q", "-b", "main")
	testvault.Git(t, clone, "remote", "add", "origin", "git@github.com:myshop/myshop-api.git")
	os.WriteFile(filepath.Join(clone, "server", "orders", "line.go"), []byte("package orders\n"), 0o644)
	cfg, _ := config.Load()
	cfg.AddProject(filepath.Join(root, "MyShop"))
	cfg.Repos["github.com/myshop/myshop-api"] = clone
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	sim := filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")
	return env{filepath.Join(root, "MyShop"), clone, sim, Options{Sim: sim, Now: func() time.Time { return time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC) }}}
}

func run(t *testing.T, e env, name string, in map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(in)
	var out bytes.Buffer
	if code := Run(name, bytes.NewReader(b), &out, e.o); code != 0 {
		t.Fatalf("%s exited %d", name, code)
	}
	if out.Len() == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("%s wrote %q", name, out.String())
	}
	return m
}

func hso(m map[string]any) map[string]any {
	h, _ := m["hookSpecificOutput"].(map[string]any)
	return h
}

func TestBeforeGivesTheRulesOnceForEachEntity(t *testing.T) {
	e := setup(t)
	in := map[string]any{"session_id": "s1", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(e.clone, "server", "orders", "line.go")}}
	out := run(t, e, "before", in)
	ctx, _ := hso(out)["additionalContext"].(string)
	for _, want := range []string{"belongs to shop-api", "SEC-1 (Security Policy)", "DS-4 (Shop Data Schema, goal Add currency to order lines)", "carto-check"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("the rules lack %q:\n%s", want, ctx)
		}
	}
	if again := run(t, e, "before", in); again != nil {
		t.Fatalf("the rules came twice: %v", again)
	}
	os.WriteFile(filepath.Join(e.clone, "tools.sh"), []byte("x"), 0o644)
	none := run(t, e, "before", map[string]any{"session_id": "s1", "tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(e.clone, "tools.sh")}})
	if ctx, _ := hso(none)["additionalContext"].(string); !strings.Contains(ctx, "no service or module owns this path") {
		t.Fatalf("an unowned path: %v", none)
	}
}

func TestGuard(t *testing.T) {
	e := setup(t)
	svc := filepath.Join(e.project, "Services", "shop-api.md")
	deny := func(in map[string]any) string {
		out := run(t, e, "guard", in)
		if out == nil {
			return ""
		}
		r, _ := hso(out)["permissionDecisionReason"].(string)
		return r
	}
	edit := func(from, to string) map[string]any {
		return map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": svc, "old_string": from, "new_string": to}}
	}
	if r := deny(edit(`adopts: ["[[Shop Data Schema]]", "[[API Standards]]"]`, `adopts: ["[[Shop Data Schema]]"]`)); !strings.Contains(r, "`adopts` is declared state") {
		t.Errorf("an edit of adopts: %q", r)
	}
	if r := deny(edit("Serves the REST API that shop-web calls.", "Serves the REST API of the shop.")); r != "" {
		t.Errorf("an edit of the Purpose was refused: %q", r)
	}
	if r := deny(edit("compliance: 12/13", "compliance: 13/13")); !strings.Contains(r, "written by the tool") {
		t.Errorf("an edit of a tool field: %q", r)
	}
	check := filepath.Join(e.project, "Checks", "2026-10", "2026-10-08 1550 shop-api · 08fd415.md")
	if r := deny(map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": check, "old_string": "inconclusive", "new_string": "met"}}); !strings.Contains(r, "never changed") {
		t.Errorf("an edit of a check: %q", r)
	}
	if r := deny(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "carto approve chg-abc123"}}); !strings.Contains(r, "the user's acts") {
		t.Errorf("an agent ran a user act: %q", r)
	}
	if r := deny(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "sed -i '' s/a/b/ '" + svc + "'"}}); r == "" {
		t.Error("a shell write in the project was allowed")
	}
	if r := deny(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "cat '" + svc + "'"}}); r != "" {
		t.Errorf("a read was refused: %q", r)
	}
	if r := deny(map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(e.clone, "x.go"), "content": "package x"}}); r != "" {
		t.Errorf("a code write was refused: %q", r)
	}
	goal := func(name, from, to string) map[string]any {
		return map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(e.project, "Goals", name+".md"), "old_string": from, "new_string": to}}
	}
	draft := "Make checkout survive a recommendation outage"
	if r := deny(goal(draft, "None. shop-api was attached", "- **adopt** [[API Standards]] · [[shop-web]]\n\nshop-api was attached")); r != "" {
		t.Errorf("an agent's revision of a draft goal was refused: %q", r)
	}
	if r := deny(goal(draft, "status: draft", "status: active")); !strings.Contains(r, "status") {
		t.Errorf("an agent started a draft goal: %q", r)
	}
	if r := deny(goal("Add currency to order lines", "- **add** [[Shop Data Schema#DS-4]]", "- **add** [[Shop Data Schema#DS-9]]")); !strings.Contains(r, "Map changes") {
		t.Errorf("an edit of an active goal's Map changes: %q", r)
	}
}

func TestStopRemindsOnce(t *testing.T) {
	e := setup(t)
	run(t, e, "before", map[string]any{"session_id": "s2", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(e.clone, "server", "orders", "line.go")}})
	out := run(t, e, "stop", map[string]any{"session_id": "s2"})
	if out["decision"] != "block" || !strings.Contains(out["reason"].(string), "shop-api") {
		t.Fatalf("stop: %v", out)
	}
	if again := run(t, e, "stop", map[string]any{"session_id": "s2"}); again != nil {
		t.Fatalf("stop reminded twice: %v", again)
	}
	if active := run(t, e, "stop", map[string]any{"session_id": "s3", "stop_hook_active": true}); active != nil {
		t.Fatal("stop blocked while a stop hook was active")
	}
}
