package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
)

const goal = "Add currency to order lines"

// fakeAgent completes a task by ticking its box in the goal, as the work tool would, and
// fails T5 every time. It exits 0 for a sweep.
const fakeAgent = `#!/bin/sh
[ -n "$BIN_OUT" ] && printf '%s' "$CARTO_BIN" > "$BIN_OUT"
task=$(printf '%s' "$1" | sed -n 's/.*Task: \(T[0-9]*\),.*/\1/p')
if [ -z "$task" ]; then echo '{"result":"swept","session_id":"x"}'; exit 0; fi
if [ "$task" = "T5" ]; then echo "tests fail" >&2; exit 1; fi
perl -0pi -e "s/- \[ \] $task /- [x] $task /" "$CARTO_PROJECT/Goals/` + goal + `.md"
git -C "$CARTO_PROJECT" commit -qam "fake agent: $task done"
echo '{"result":"done '"$task"'","session_id":"x"}'
`

func setup(t *testing.T) (*Runner, string) {
	t.Helper()
	testvault.Identity(t)
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	clones := map[string]string{}
	for _, repo := range []string{"myshop-api", "myshop-web", "myshop-inventory", "myshop-recs", "myshop-schemas"} {
		dir := filepath.Join(t.TempDir(), repo)
		os.MkdirAll(dir, 0o755)
		testvault.Git(t, dir, "init", "-q", "-b", "main")
		os.WriteFile(filepath.Join(dir, "README"), []byte(repo), 0o644)
		testvault.Git(t, dir, "add", "-A")
		testvault.Git(t, dir, "commit", "-qm", "init")
		clones[repo] = dir
	}
	agent := filepath.Join(t.TempDir(), "agent.sh")
	os.WriteFile(agent, []byte(fakeAgent), 0o755)
	sim, err := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sim.ClaimList = nil // the runner's own claim files are the claims
	now := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)
	r := &Runner{
		Folder: folder,
		Load:   func() (*core.Project, error) { return core.Load(folder, sim) },
		Clone:  func(repo string) string { return clones[repo] },
		Agent:  []string{agent, "{prompt}"},
		Now:    func() time.Time { return now },
		Home:   t.TempDir(),
		Max:    1, // the fake agent edits the goal outside the vault lock, as no real agent may
	}
	return r, root
}

func TestTheRunnerDrainsTheQueue(t *testing.T) {
	r, root := setup(t)
	bin := filepath.Join(t.TempDir(), "bin")
	t.Setenv("BIN_OUT", bin)
	t.Setenv("CARTO_BIN", "")
	var out strings.Builder
	r.Out = &out
	if err := r.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	g := testvault.Read(t, filepath.Join(r.Folder, "Goals", goal+".md"))
	for _, want := range []string{"- [x] T4 [[inventory-service]]", "- [x] T7 [[shop-api]]", "[attempts:: 2]", "T5 failed (attempt 2)", "It is stuck and waits for the user."} {
		if !strings.Contains(g, want) {
			t.Fatalf("the goal lacks %q\nrunner said:\n%s\ngoal:\n%s", want, out.String(), g)
		}
	}
	claims, _ := filepath.Glob(filepath.Join(root, ".carto", "claims", "*.json"))
	if len(claims) != 0 {
		t.Fatalf("claims left behind: %v", claims)
	}
	if _, err := os.Stat(filepath.Join(root, ".carto", "runner.json")); !os.IsNotExist(err) {
		t.Fatal("the lease was not released")
	}
	logs, _ := filepath.Glob(filepath.Join(r.Folder, "Sessions", "2026-10", "2026-10-09 1100 *.md"))
	if len(logs) < 4 {
		t.Fatalf("expected a session log for each session, found %d", len(logs))
	}
	if _, err := os.Stat(filepath.Join(r.Home, "worktrees", "myshop-api", "goal-add-currency", ".git")); err != nil {
		t.Fatal("no worktree for the goal branch in myshop-api")
	}
	if out := testvault.Git(t, root, "status", "--porcelain"); out != "" {
		t.Fatalf("the runner left the vault dirty:\n%s", out)
	}
	if log := testvault.Git(t, root, "log", "--format=%s"); !strings.Contains(log, "carto: claim for session") || !strings.Contains(log, "carto: the runner ended") {
		t.Fatalf("claims and the lease are not versioned:\n%s", log)
	}
	if self, _ := os.Executable(); testvault.Read(t, bin) != self {
		t.Fatalf("the agents ran carto %q, not the runner's own binary", testvault.Read(t, bin))
	}
}

// A claim left by a runner that ended counts as a failed attempt when the next starts.
func TestStaleClaimsCountAsAttempts(t *testing.T) {
	r, root := setup(t)
	os.MkdirAll(filepath.Join(root, ".carto", "claims"), 0o755)
	os.WriteFile(filepath.Join(root, ".carto", "claims", "old.json"), []byte(`{"goal":"`+goal+`","task":"T6","session":"old","repo":"myshop-inventory"}`), 0o644)
	if err := r.take(); err != nil {
		t.Fatal(err)
	}
	if err := r.sweepStale(); err != nil {
		t.Fatal(err)
	}
	r.release()
	if !strings.Contains(testvault.Read(t, filepath.Join(r.Folder, "Goals", goal+".md")), "the runner that ran it ended before the task did") {
		t.Fatal("the stale claim was not counted")
	}
}

// A second runner refuses while the lease of a live one holds.
func TestOneRunnerAtATime(t *testing.T) {
	r, root := setup(t)
	os.MkdirAll(filepath.Join(root, ".carto"), 0o755)
	host, _ := os.Hostname()
	os.WriteFile(filepath.Join(root, ".carto", "runner.json"), []byte(`{"host":"`+host+`","pid":1,"until":"2026-10-09T12:00:00"}`), 0o644)
	if err := r.Run(context.Background(), false); err == nil || !strings.Contains(err.Error(), "the runner runs on") {
		t.Fatalf("a second runner started: %v", err)
	}
}

// The default command gives the agent the vault and the clones, but not its own folder twice.
func TestCommandAddsTheFolders(t *testing.T) {
	r := &Runner{Folder: "/v/P3", Dirs: []string{"/v", "/code/a", "/code/b"}}
	got := strings.Join(r.command(&Job{Kind: "sweep", Dir: "/code/a", Session: "s1"}), " ")
	if !strings.Contains(got, "--session-id s1") || !strings.HasSuffix(got, "--add-dir /v --add-dir /code/b") || strings.Contains(got, "{dirs}") {
		t.Fatalf("the command: %s", got)
	}
}

// A branch that another session has checked out makes the runner wait, not fail.
func TestWorktreeOfABusyBranch(t *testing.T) {
	testvault.Identity(t)
	clone := t.TempDir()
	testvault.Git(t, clone, "init", "-q", "-b", "main")
	testvault.Git(t, clone, "commit", "-q", "--allow-empty", "-m", "start")
	other := filepath.Join(t.TempDir(), "other")
	testvault.Git(t, clone, "worktree", "add", "-q", "-b", "goal/x", other)
	r := &Runner{Home: t.TempDir()}
	_, err := r.worktree(clone, "repo", "goal/x", "main", false)
	var busy *busyError
	real, _ := filepath.EvalSymlinks(other)
	if !errors.As(err, &busy) || busy.at != real {
		t.Fatalf("a busy branch: %v", err)
	}
	if dir, err := r.worktree(clone, "repo", "goal/y", "main", false); err != nil || dir == "" {
		t.Fatalf("a free branch: %v", err)
	}
}

// A new branch starts at origin's default branch, or at the clone's own when work lands locally.
func TestWorktreeStartsAtTheBase(t *testing.T) {
	testvault.Identity(t)
	clone := t.TempDir()
	testvault.Git(t, clone, "init", "-q", "-b", "main")
	testvault.Git(t, clone, "commit", "-q", "--allow-empty", "-m", "origin")
	origin := testvault.Git(t, clone, "rev-parse", "HEAD")
	testvault.Git(t, clone, "update-ref", "refs/remotes/origin/main", origin)
	testvault.Git(t, clone, "commit", "-q", "--allow-empty", "-m", "landed locally")
	local := testvault.Git(t, clone, "rev-parse", "HEAD")
	r := &Runner{Home: t.TempDir()}
	for branch, want := range map[string]string{"goal/pr": origin, "goal/local": local} {
		dir, err := r.worktree(clone, "repo", branch, "main", branch == "goal/local")
		if err != nil {
			t.Fatal(err)
		}
		if got := testvault.Git(t, dir, "rev-parse", "HEAD"); got != want {
			t.Errorf("%s starts at %s, want %s", branch, got, want)
		}
	}
}
