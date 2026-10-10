package work

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
)

func load(t *testing.T, folder string) *core.Project {
	t.Helper()
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	env.ClaimList = nil // no agent holds a task
	p, err := core.Load(folder, env)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func task(p *core.Project, g, id string) *core.Task {
	for _, t := range p.Tasks[g] {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// A stuck task: retry clears its attempts and leaves the hint; take gives it to the user.
func TestRetryAndTake(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	for i := 0; i < 2; i++ {
		if err := k.Attempt(goal, "T5", "tests fail"); err != nil {
			t.Fatal(err)
		}
	}
	commit(t, k, "stuck")
	ready := func() bool {
		for _, x := range load(t, folder).Ready(goal) {
			if x.ID == "T5" {
				return true
			}
		}
		return false
	}
	if ready() {
		t.Fatal("a stuck task is still ready")
	}
	k, _ = New(folder, now)
	if err := k.Retry(goal+"#T5", ""); err == nil {
		t.Fatal("a retry with no hint")
	}
	if err := k.Retry(goal+"#T5", "Run npm ci first."); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "retry")
	if core.Attempts(task(load(t, folder), goal, "T5")) != 0 || !ready() {
		t.Fatal("the retried task is not ready again")
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Goals", goal+".md")), "### T5\n\nHint from the user: Run npm ci first.") {
		t.Fatal("no hint in Task details")
	}
	k, _ = New(folder, now)
	if err := k.Take(goal + "#T5"); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "take")
	if ready() {
		t.Fatal("a task the user took is still ready")
	}
}

// Request changes on an upkeep task opens it again; Not now hides only that task's line.
func TestRequestAndDeferATask(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	if err := k.DeferTask("Upkeep#T2", "2026-10-20"); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "defer")
	if f := task(load(t, folder), "Upkeep", "T2").F["defer"]; f != "2026-10-20" {
		t.Fatalf("defer is %q", f)
	}
	k, _ = New(folder, now)
	if err := k.Request("Upkeep#T2", "Keep the old column for one release.", ""); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "request")
	p := load(t, folder)
	tk := task(p, "Upkeep", "T2")
	if tk.Box != " " || tk.F["commit"] != "" {
		t.Fatalf("T2 is not open again: %+v", tk)
	}
	if ready, _ := p.ReadyToLand("Upkeep"); len(ready) != 0 {
		t.Fatal("an opened task is still ready to land")
	}
	k, _ = New(folder, now)
	if err := k.Request(goal, "Add a changelog entry.", ""); err == nil {
		t.Fatal("a request on a goal with no entity")
	}
	if err := k.Request(goal, "Add a changelog entry.", "shop-api"); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "request on a goal")
	if task(load(t, folder), goal, "T9") == nil {
		t.Fatal("no new task T9")
	}
}

// A choice whose option carries a change waits for an agent until something carries it out.
func TestAChoiceWaitsToBeApplied(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	if err := k.Choose("reindex job is in no entity", 3, ""); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "choose")
	if got := load(t, folder).ToApply(); len(got) != 1 || got[0] != "reindex job is in no entity" {
		t.Fatalf("to apply: %v", got)
	}
	k, _ = New(folder, now)
	if _, err := k.Add("Upkeep", "shop-api", "Map the reindex job", map[string]string{"finding": "[[reindex job is in no entity]]"}); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "add")
	if got := load(t, folder).ToApply(); len(got) != 0 {
		t.Fatalf("still to apply: %v", got)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Findings", "reindex job is in no entity.md")), "[apply:: [[Upkeep#^t4]]]") {
		t.Fatal("the finding does not link what carried out the choice")
	}
	if got := load(t, folder).ToApply(); len(got) != 0 {
		t.Fatal(got)
	}
}

// A decision on a lint result becomes an answered question that holds it.
func TestDecideLint(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	name, err := k.DecideLint("L2", "shop-web", "shop-web has no obligations.", 2)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, k, "lint")
	text := testvault.Read(t, filepath.Join(folder, "Questions", name+".md"))
	for _, want := range []string{"status: answered", "holds: lint L2 [[shop-web]]", "asked_by: user", "[chosen:: 2]", "[apply:: pending]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the question lacks %q:\n%s", want, text)
		}
	}
}
