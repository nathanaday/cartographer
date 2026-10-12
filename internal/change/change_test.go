package change

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/testvault"
	"github.com/nathanaday/cartographer/internal/vault"
)

var now = time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)

func setup(t *testing.T) (string, *Index) {
	t.Helper()
	testvault.Identity(t)
	root := testvault.New(t)
	ix, err := NewIndex(filepath.Join(root, "MyShop"))
	if err != nil {
		t.Fatal(err)
	}
	return root, ix
}

// commit runs one act on a writer, seals it, and commits it, as app.Commit does without
// the refresh.
func commit(t *testing.T, root string, f func(w *vault.Writer) (*Doc, error)) (*Doc, error) {
	t.Helper()
	w := newWriter(root)
	defer w.Abort()
	c, err := f(w)
	if err != nil {
		return nil, err
	}
	if err := Seal(w, c, now); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("act"); err != nil {
		t.Fatal(err)
	}
	return c, nil
}

func adopts(t *testing.T, ix *Index, doc string) []string {
	d, err := md.Load(ix.Paths[doc])
	if err != nil {
		t.Fatal(err)
	}
	return md.Names(d.FM["adopts"])
}

// A proposed change applies only when the user approves it, and undo takes it back.
func TestProposeApproveUndo(t *testing.T) {
	root, ix := setup(t)
	c, err := Propose(ix, "invoice-worker adopts the Shop Data Schema",
		[]Op{{Op: "add", Doc: "invoice-worker", Field: "adopts", Value: "[[Shop Data Schema]]"}}, "s1", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := adopts(t, ix, "invoice-worker"); len(got) != 0 {
		t.Fatalf("a proposal changed the map: %v", got)
	}
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Approve(ix, w, c.ID, "approve "+c.ID, now) }); err != nil {
		t.Fatal(err)
	}
	if got := adopts(t, ix, "invoice-worker"); len(got) != 1 || got[0] != "Shop Data Schema" {
		t.Fatalf("approval did not apply: %v", got)
	}
	again, _ := Find(ix, c.ID)
	if again.Status != "applied" || again.Acts[0].Base == "" || again.Acts[0].Files["MyShop/Services/invoice-worker.md"] == "" {
		t.Fatalf("the change is %s with act %+v", again.Status, again.Acts[0])
	}
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Approve(ix, w, c.ID, "again", now) }); err == nil {
		t.Fatal("an applied change applied twice")
	}
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Undo(ix, w, now) }); err != nil {
		t.Fatal(err)
	}
	if got := adopts(t, ix, "invoice-worker"); len(got) != 0 {
		t.Fatalf("undo left %v", got)
	}
	if again, _ := Find(ix, c.ID); again.Status != "undone" {
		t.Fatalf("after undo the change is %s", again.Status)
	}
	if out := testvault.Git(t, root, "status", "--porcelain", "--", "MyShop"); out != "" {
		t.Fatalf("writes left the vault dirty:\n%s", out)
	}
}

// A proposal that cannot apply is refused when it is proposed.
func TestProposalThatCannotApplyIsRefused(t *testing.T) {
	_, ix := setup(t)
	if _, err := Propose(ix, "x", []Op{{Op: "add", Doc: "no-such-service", Field: "adopts", Value: "[[x]]"}}, "", now); err == nil {
		t.Fatal("a proposal on a missing document was accepted")
	}
}

// Acts undo newest first, each in turn; an act whose file a later write changed is refused.
func TestUndoInTurnAndRefuseWhenBuiltOn(t *testing.T) {
	root, ix := setup(t)
	user := func(title string, op Op, at time.Time) {
		t.Helper()
		if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return UserAct(ix, w, title, []Op{op}, at) }); err != nil {
			t.Fatal(err)
		}
	}
	undo := func() error {
		_, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Undo(ix, w, now) })
		return err
	}
	user("adopt API Standards on shop-web", Op{Op: "add", Doc: "shop-web", Field: "adopts", Value: "[[API Standards]]"}, now)
	user("set runtime", Op{Op: "set", Doc: "shop-web", Field: "runtime", Value: "Node 22"}, now.Add(time.Minute))
	if err := undo(); err != nil {
		t.Fatalf("the last act should undo: %v", err)
	}
	if err := undo(); err != nil {
		t.Fatalf("the act before it should undo too: %v", err)
	}
	if got := adopts(t, ix, "shop-web"); len(got) != 1 {
		t.Fatalf("shop-web adopts %v after two undos", got)
	}
	if err := undo(); err == nil || !strings.Contains(err.Error(), "no act to undo") {
		t.Fatalf("a third undo: %v", err)
	}

	user("adopt API Standards on shop-web again", Op{Op: "add", Doc: "shop-web", Field: "adopts", Value: "[[API Standards]]"}, now.Add(2*time.Minute))
	path := filepath.Join(ix.Folder, "Services", "shop-web.md")
	os.WriteFile(path, []byte(testvault.Read(t, path)+"\nA note by hand.\n"), 0o644)
	if err := undo(); err == nil || !strings.Contains(err.Error(), "MyShop/Services/shop-web.md changed after") {
		t.Fatalf("undo over a later edit: %v", err)
	}
	if !strings.Contains(testvault.Read(t, path), "A note by hand.") {
		t.Fatal("a refused undo changed the file")
	}
	if out := testvault.Git(t, root, "log", "--format=%s", "-n", "6"); strings.Count(out, "carto: act") != 5 {
		t.Fatalf("each act and undo is one commit:\n%s", out)
	}
}

// Approving a draft goal numbers its new clause, writes it as pending, applies its
// relations, and makes it active; closing it makes the clause active.
func TestGoalApproveAndClose(t *testing.T) {
	root, ix := setup(t)
	goal := `---
id: goal-t3st00
type: carto-goal
description: Tax rates on order lines.
tags: []
created: 2026-10-09T10:00:00
updated: 2026-10-09T10:00:00
status: draft
priority: medium
after: []
defer: ""
branch: goal/tax-rate
resolves: []
affects: []
progress: ""
tasks: ""
landed: ""
opened: ""
closed: ""
---

## Intent

> Orders need a tax rate.

## Map changes

- **add** [[Shop Data Schema#DS-new]] · Each order line carries a tax rate in basis points. [verify:: test]
- **adopt** [[API Standards]] · [[shop-web]]

## Requirements

## Tasks

- [ ] T1 [[shop-api]] Add the tax rate ^t1

## Log
`
	path := filepath.Join(root, "MyShop", "Goals", "Add tax rates.md")
	if err := os.WriteFile(path, []byte(goal), 0o644); err != nil {
		t.Fatal(err)
	}
	ix, _ = NewIndex(filepath.Join(root, "MyShop"))
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) {
		return UserAct(ix, w, "approve Add tax rates", []Op{{Op: "goal.approve", Doc: "Add tax rates"}}, now)
	}); err != nil {
		t.Fatal(err)
	}
	contract := testvault.Read(t, ix.Paths["Shop Data Schema"])
	if !strings.Contains(contract, "### DS-6\nEach order line carries a tax rate in basis points.\n[verify:: test] [state:: pending] [goal:: [[Add tax rates]]]") {
		t.Fatalf("the new clause is not DS-6, pending:\n%s", contract)
	}
	g := testvault.Read(t, path)
	if !strings.Contains(g, "[[Shop Data Schema#DS-6]]") || !strings.Contains(g, "status: active") {
		t.Fatalf("the goal was not rewritten and activated:\n%s", g)
	}
	if got := adopts(t, ix, "shop-web"); len(got) != 2 {
		t.Fatalf("the adoption did not apply: %v", got)
	}
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(filepath.Join(root, "MyShop"), env)
	if err != nil {
		t.Fatal(err)
	}
	pending := 0
	for _, o := range p.GoalObligations("Add tax rates") {
		if o.Pending() {
			pending++
		}
	}
	// DS-6 on 4 adopters + shop-web's new adoption: API-1..3 pending, and DS-6 on shop-web counted once.
	if pending < 5 {
		t.Fatalf("the goal has %d pending obligations", pending)
	}
	w := newWriter(root)
	if _, err := CloseGoal(ix, w, "Add tax rates", now); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("close"); err != nil {
		t.Fatal(err)
	}
	contract = testvault.Read(t, ix.Paths["Shop Data Schema"])
	if !strings.Contains(contract, "### DS-6\nEach order line carries a tax rate in basis points.\n[verify:: test]\n") {
		t.Fatalf("closing did not make DS-6 active:\n%s", contract)
	}
	if !strings.Contains(testvault.Read(t, path), "status: done") {
		t.Fatal("the goal is not done")
	}
}

// An edit by hand that is not committed survives an act on the same file and its undo.
func TestUndoKeepsAnEditByHand(t *testing.T) {
	root, ix := setup(t)
	path := filepath.Join(ix.Folder, "Services", "shop-web.md")
	os.WriteFile(path, []byte(strings.Replace(testvault.Read(t, path), "## Purpose\n", "## Purpose\n\nA line by hand.\n", 1)), 0o644)
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) {
		return UserAct(ix, w, "adopt", []Op{{Op: "add", Doc: "shop-web", Field: "adopts", Value: "[[API Standards]]"}}, now)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Undo(ix, w, now) }); err != nil {
		t.Fatal(err)
	}
	text := testvault.Read(t, path)
	if !strings.Contains(text, "A line by hand.") || strings.Contains(text, "API Standards") {
		t.Fatalf("after undo:\n%s", text)
	}
}

// The tool stamps a new document's created and updated times when the act applies it.
func TestCreateStampsTheTimes(t *testing.T) {
	root, ix := setup(t)
	text := "---\nid: svc-zzzzzz\ntype: carto-service\ndescription: A worker.\ncreated: 2030-01-01T00:00:00\nupdated: 2030-01-01T00:00:00\nrepository: github.com/myshop/myshop-api\npath: worker/\n---\n"
	c, err := Propose(ix, "Add a worker", []Op{{Op: "create", Path: "Services/worker.md", Text: text}}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	later := now.Add(time.Hour)
	if _, err := commit(t, root, func(w *vault.Writer) (*Doc, error) { return Approve(ix, w, c.ID, "approve", later) }); err != nil {
		t.Fatal(err)
	}
	got := testvault.Read(t, filepath.Join(ix.Folder, "Services", "worker.md"))
	if !strings.Contains(got, "created: 2026-10-09T12:00:00") || !strings.Contains(got, "updated: 2026-10-09T12:00:00") {
		t.Fatalf("the times:\n%s", got)
	}
}

func TestFileTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Suggested: P3 systems": "Suggested - P3 systems",
		"a/b [x] #1":            "a-b -x- -1",
		strings.Repeat("é", 90): strings.Repeat("é", 80),
		"Exclude cobra help and completion from HK-9, with -h and __completeNoDesc (replaces chg-4l0n71)": "Exclude cobra help and completion from HK-9, with -h and __completeNoDesc",
	} {
		if got := fileTitle(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

// A corrected proposal withdraws the one it replaces, so the user decides on one.
func TestReplaceWithdrawsTheOld(t *testing.T) {
	root, ix := setup(t)
	op := []Op{{Op: "add", Doc: "invoice-worker", Field: "adopts", Value: "[[Shop Data Schema]]"}}
	old, err := Propose(ix, "invoice-worker adopts the schema", op, "", now)
	if err != nil {
		t.Fatal(err)
	}
	ix, _ = NewIndex(ix.Folder)
	c, err := Replace(ix, old.ID, "invoice-worker adopts the Shop Data Schema", op, "", now)
	if err != nil {
		t.Fatal(err)
	}
	ix, _ = NewIndex(ix.Folder)
	if got, _ := Find(ix, old.ID); got.Status != "withdrawn" {
		t.Fatalf("the old change is %s", got.Status)
	}
	if !strings.Contains(testvault.Read(t, c.Path), "in place of [["+old.Name+"]]") {
		t.Fatal("the new change does not name the old one")
	}
	if _, err := Replace(ix, old.ID, "again", op, "", now); err == nil {
		t.Fatal("a withdrawn change was replaced")
	}
	if out := testvault.Git(t, root, "status", "--porcelain", "--", "MyShop"); out != "" {
		t.Fatalf("the replacement left the vault dirty:\n%s", out)
	}
}
