package live

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, dir, file, text string) string {
	t.Helper()
	os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755)
	os.WriteFile(filepath.Join(dir, file), []byte(text), 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", file)
	return git(t, dir, "rev-parse", "HEAD")
}

// The live environment answers reachability and staleness from real git.
func TestReachabilityAndStalenessFollowGit(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	base := commit(t, dir, "server/orders/line.go", "a")
	git(t, dir, "checkout", "-q", "-b", "goal/x")
	onBranch := commit(t, dir, "server/orders/line.go", "b")
	git(t, dir, "checkout", "-q", "main")
	other := commit(t, dir, "server/router.go", "c")
	_ = other

	e := &Env{clones: map[string]string{"api": dir}, defaults: map[string]string{}, Default: "main", now: Wall}
	never := func(string, string) bool { return false }
	if !e.OnDefault("api", base, "main", never) {
		t.Error("the first commit is on main")
	}
	if e.OnDefault("api", onBranch, "goal/x", never) {
		t.Error("an unmerged branch commit is not on main")
	}
	r := &core.Result{Commit: base, Date: time.Now()}
	if got := e.ChangedSince("api", "server/orders/", r, never); len(got) != 0 {
		t.Errorf("main has not changed server/orders since the base: %v", got)
	}
	if got := e.ChangedSince("api", "server/", r, never); len(got) != 1 {
		t.Errorf("main changed server/ once since the base: %v", got)
	}
	if got := e.BranchChangedSince("api", "goal/x", "server/orders/", r); len(got) != 1 {
		t.Errorf("the goal branch changed server/orders once: %v", got)
	}
	git(t, dir, "merge", "-q", "--no-ff", "-m", "land", "goal/x")
	if !e.OnDefault("api", onBranch, "goal/x", never) {
		t.Error("after a merge, the branch commit is on main")
	}
	if got := e.ChangedSince("api", "", r, never); len(got) == 0 {
		t.Error("an empty path means the whole repository")
	}
}
