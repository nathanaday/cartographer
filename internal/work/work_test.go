package work

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/testvault"
	"github.com/nathanaday/cartographer/internal/vault"
)

var now = time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)

const goal = "Add currency to order lines"

func setup(t *testing.T) string {
	t.Helper()
	testvault.Identity(t)
	return filepath.Join(testvault.New(t), "MyShop")
}

func commit(t *testing.T, k *Work, msg string) {
	t.Helper()
	if _, err := k.Writer().Commit(msg); err != nil {
		t.Fatal(err)
	}
}

func TestTasks(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	if err := k.Done(goal, "T4", "9a8b7c6"); err != nil {
		t.Fatal(err)
	}
	if err := k.Done(goal, "T4", ""); err == nil {
		t.Fatal("a done task was done twice")
	}
	if err := k.Drop(goal, "T6", "not needed"); err == nil || !strings.Contains(err.Error(), "has not answered") {
		t.Fatalf("a task that waits on an open question was dropped: %v", err)
	}
	id, err := k.Add(goal, "shop-web", "Show the currency code on the receipt", map[string]string{"for": "DS-4", "after": "T5"})
	if err != nil || id != "T9" {
		t.Fatalf("added %q: %v", id, err)
	}
	if _, err := k.Add("Upkeep", "shop-web", "Bump a library", nil); err == nil {
		t.Fatal("an upkeep task with no source was added")
	}
	if err := k.Attempt(goal, "T5", "tests fail"); err != nil {
		t.Fatal(err)
	}
	if err := k.Attempt(goal, "T5", "tests fail again"); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "work")
	text := testvault.Read(t, k.Index().Paths[goal])
	for _, want := range []string{"- [x] T4 [[inventory-service]]", "[commit:: 9a8b7c6]", "- [ ] T9 [[shop-web]] Show the currency code on the receipt [for:: DS-4] [after:: T5] ^t9", "[attempts:: 2]", "It is stuck and waits for the user.", "[done:: 2026-10-09T11:00]", "updated: 2026-10-09T11:00:00"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the goal lacks %q:\n%s", want, text)
		}
	}
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(folder, env)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range p.Ready(goal) {
		if task.ID == "T5" {
			t.Fatal("a stuck task is still ready")
		}
	}
}

// An answer records the choice; the change it implies applies once, as the user's act.
func TestAnswerAndApplyItsChange(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	q := "Should invoice-worker adopt the Shop Data Schema"
	if err := k.Answer(q, 4, ""); err == nil {
		t.Fatal("an option that does not exist was chosen")
	}
	if err := k.Answer(q, 1, "Yes, adopt it."); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "answer")
	ops := []change.Op{{Op: "line", Doc: goal, Section: "Map changes", Line: "- **adopt** [[Shop Data Schema]] · [[invoice-worker]]"},
		{Op: "add", Doc: "invoice-worker", Field: "adopts", Value: "[[Shop Data Schema]]"}}
	apply := func() error {
		w := vault.NewWriter(vault.Root(folder))
		defer w.Abort()
		c, err := ApplyChoice(folder, w, q, ops, now)
		if err != nil {
			return err
		}
		if err := change.Seal(w, c, now); err != nil {
			return err
		}
		_, err = w.Commit("apply the choice")
		return err
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	if err := apply(); err == nil {
		t.Fatal("a choice applied twice")
	}
	d, _ := md.Load(filepath.Join(folder, "Services", "invoice-worker.md"))
	if got := md.Names(d.FM["adopts"]); len(got) != 1 {
		t.Fatalf("invoice-worker adopts %v", got)
	}
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, _ := core.Load(folder, env)
	// The adoption is a map change of the active goal, so its obligations are pending: 2 of 9.
	if m, n := p.GoalProgress(goal, time.Time{}); m != 2 || n != 9 {
		t.Fatalf("goal progress is %d of %d, want 2 of 9", m, n)
	}
	for _, o := range p.ActiveOf("invoice-worker") {
		if o.Clause.Owner == "Shop Data Schema" {
			t.Fatal("an adoption made by an active goal is active before the goal closes")
		}
	}
}

func TestChooseAFindingOption(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	f := "reindex job is in no entity"
	if err := k.Choose(f, 3, "It is temporary."); err != nil {
		t.Fatal(err)
	}
	commit(t, k, "choose")
	k2, _ := New(folder, now)
	if err := k2.Choose(f, 1, ""); err == nil {
		t.Fatal("a second choice was accepted")
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Findings", f+".md")), "[chosen:: 3] [by:: user]") {
		t.Fatal("the choice was not recorded")
	}
}

// An attempt counts only on an open task: a claim left behind by a finished session does
// not make a done task fail.
func TestAttemptOnADoneTask(t *testing.T) {
	folder := setup(t)
	k, _ := New(folder, now)
	defer k.Writer().Abort()
	if err := k.Attempt("Upkeep", "T1", "the runner ended"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("an attempt on a done task: %v", err)
	}
	if err := k.Attempt("Upkeep", "T3", "the runner ended"); err != nil {
		t.Fatalf("an attempt on an open task: %v", err)
	}
}

// The first upkeep task of a project makes its standing goal.
func TestTheFirstUpkeepTaskMakesUpkeep(t *testing.T) {
	folder := setup(t)
	os.Remove(filepath.Join(folder, "Goals", "Upkeep.md"))
	k, _ := New(folder, now)
	if _, err := k.Add("Upkeep", "shop-api", "Remove the old field", map[string]string{"by": "user"}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Writer().Commit("add"); err != nil {
		t.Fatal(err)
	}
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(folder, env)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Standing("Upkeep") || len(p.Tasks["Upkeep"]) != 1 || p.S("Upkeep", "status") != "active" {
		t.Fatalf("Upkeep: standing %v, %d tasks, %s", p.Standing("Upkeep"), len(p.Tasks["Upkeep"]), p.S("Upkeep", "status"))
	}
}
