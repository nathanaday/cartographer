package land

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
)

func TestLandAnUpkeepTask(t *testing.T) {
	testvault.Identity(t)
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	sim, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(folder, sim)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	fake := func(dir, name string, args ...string) (string, error) {
		calls = append(calls, dir+": "+name+" "+strings.Join(args, " "))
		if strings.Contains(strings.Join(args, " "), "mergeCommit") {
			return "3c4d5e6f00112233", nil
		}
		return "", nil
	}
	if _, _, err := Targets(p, "Add currency to order lines"); err == nil {
		t.Fatal("a goal with open tasks is ready to land")
	}
	landed, err := Land(folder, p, "Upkeep#T2", func(repo string) string { return "/clones/" + repo }, fake, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if err != nil || len(landed) != 1 {
		t.Fatalf("landed %v: %v", landed, err)
	}
	if !strings.Contains(strings.Join(calls, "\n"), "/clones/myshop-inventory: gh pr merge upkeep/t2 --merge") {
		t.Fatalf("calls: %v", calls)
	}
	p2, _ := core.Load(folder, sim)
	if !p2.Landed("myshop-inventory", "upkeep/t2") {
		t.Fatal("the landing was not recorded")
	}
	if ready, _ := p2.ReadyToLand("Upkeep"); len(ready) != 0 {
		t.Fatalf("Upkeep still has %d tasks to land", len(ready))
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Goals", "Upkeep.md")), "landed T2 myshop-inventory 3c4d5e6") {
		t.Fatal("no landed line in the Log")
	}
}

// A local merge lands a branch on the default branch, whether the clone has the default
// branch checked out or not, and refuses a tree with uncommitted changes.
func TestMergeLocally(t *testing.T) {
	testvault.Identity(t)
	for _, checkedOut := range []bool{true, false} {
		dir := t.TempDir()
		git := func(args ...string) string { return testvault.Git(t, dir, args...) }
		git("init", "-q", "-b", "main")
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
		git("add", "-A")
		git("commit", "-qm", "start")
		git("checkout", "-q", "-b", "goal/x")
		os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644)
		git("add", "-A")
		git("commit", "-qm", "work")
		if checkedOut {
			git("checkout", "-q", "main")
			os.WriteFile(filepath.Join(dir, "a.txt"), []byte("edited\n"), 0o644)
			if _, err := Merge(dir, "main", "goal/x", "Merge goal/x", System); err == nil || !strings.Contains(err.Error(), "uncommitted") {
				t.Fatalf("a merge into a dirty tree: %v", err)
			}
			git("checkout", "--", "a.txt")
		}
		sha, err := Merge(dir, "main", "goal/x", "Merge goal/x (Goal)", System)
		if err != nil {
			t.Fatalf("checked out %v: %v", checkedOut, err)
		}
		if got := strings.TrimSpace(git("rev-parse", "main")); got != sha {
			t.Fatalf("main is %s, the merge is %s", got, sha)
		}
		if msg := git("log", "-1", "--format=%s %P", "main"); !strings.HasPrefix(msg, "Merge goal/x (Goal) ") || len(strings.Fields(msg)) != 5 {
			t.Fatalf("the merge commit: %s", msg)
		}
		if list := strings.TrimSpace(git("worktree", "list")); strings.Contains(list, "\n") {
			t.Fatalf("a worktree was left: %s", list)
		}
	}
}

// A goal lands only after its review passed: the last review line of its Log says so.
func TestAGoalLandsAfterItsReview(t *testing.T) {
	testvault.Identity(t)
	folder := filepath.Join(testvault.New(t), "MyShop")
	sim, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	path := filepath.Join(folder, "Goals", "Add currency to order lines.md")
	text := strings.ReplaceAll(testvault.Read(t, path), "- [ ] T", "- [x] T")
	for _, c := range []struct {
		log  string
		want bool
	}{
		{"", false},
		{"- 2026-10-09 12:00 not ready to land: the review added T9.\n- 2026-10-09 12:00 T9 added: a fix.\n", false},
		{"- 2026-10-09 13:00 ready to land: goal/add-currency at 1a2b3c4.\n", true},
		{"- 2026-10-09 14:00 T10 added: another fix.\n", false},
	} {
		text += c.log
		os.WriteFile(path, []byte(text), 0o644)
		p, err := core.Load(folder, sim)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Targets(p, "Add currency to order lines")
		if p.Reviewed("Add currency to order lines") != c.want || (err == nil) != c.want {
			t.Fatalf("after %q: reviewed %v, targets: %v", c.log, p.Reviewed("Add currency to order lines"), err)
		}
	}
}
