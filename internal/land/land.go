// Package land merges a goal's (or an upkeep task's) branches through the forge, so that
// branch protection and required checks apply, and records each merge as a "landed" line
// of the goal's Log (Specification 5.1.5).
package land

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/work"
)

// Exec runs a command in a folder and returns its standard output.
type Exec func(dir string, name string, args ...string) (string, error)

// System runs real commands.
func System(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// Target is one branch to land in one repository.
type Target struct {
	Repo, Branch, Task string
}

// Targets returns what landing a goal (or "Upkeep#T2") merges, definition repositories
// first (Specification 5.1.5).
func Targets(p *core.Project, key string) (string, []Target, error) {
	goal, task, _ := strings.Cut(key, "#")
	if _, ok := p.Docs[goal]; !ok || p.Type(goal) != "goal" {
		return "", nil, fmt.Errorf("no goal named %q", goal)
	}
	if p.Standing(goal) {
		ready, _ := p.ReadyToLand(goal)
		for _, t := range ready {
			if t.ID == task {
				return goal, []Target{{p.TaskRepo(t), p.TaskBranch(t), t.ID}}, nil
			}
		}
		return "", nil, fmt.Errorf("%s of %s is not done, or has landed", task, goal)
	}
	if _, whole := p.ReadyToLand(goal); !whole {
		return "", nil, fmt.Errorf("[[%s]] is not ready to land: every task must be done, and a repository must be left to land", goal)
	}
	if !p.Reviewed(goal) {
		return "", nil, fmt.Errorf("[[%s]] is not ready to land: the review has not passed; the runner reviews it when every task is done", goal)
	}
	definitions := map[string]bool{}
	for _, t := range p.Tasks[goal] {
		if p.Type(t.Target) == "contract" {
			definitions[p.TaskRepo(t)] = true
		}
	}
	var out []Target
	for _, b := range p.GoalBranches(goal) {
		if b.State != "landed" {
			out = append(out, Target{Repo: b.Repo, Branch: p.S(goal, "branch")})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return definitions[out[i].Repo] && !definitions[out[j].Repo] })
	return goal, out, nil
}

// Land merges each target's pull request in its clone and records it. It stops at the
// first failure; what landed before it stays recorded.
func Land(folder string, p *core.Project, key string, clone func(repo string) string, run Exec, now time.Time) ([]string, error) {
	goal, targets, err := Targets(p, key)
	if err != nil {
		return nil, err
	}
	var landed []string
	for _, t := range targets {
		dir := clone(t.Repo)
		if dir == "" {
			return landed, fmt.Errorf("no local clone of %s on this machine", t.Repo)
		}
		var sha string
		if PullRequests(p) {
			if _, err := run(dir, "gh", "pr", "view", t.Branch, "--json", "state"); err != nil {
				return landed, fmt.Errorf("%s has no pull request for %s yet; the runner opens it after the review (carto run)", t.Repo, t.Branch)
			}
			if _, err := run(dir, "gh", "pr", "merge", t.Branch, "--merge"); err != nil {
				return landed, err
			}
			sha, err = run(dir, "gh", "pr", "view", t.Branch, "--json", "mergeCommit", "-q", ".mergeCommit.oid")
			if err != nil || sha == "" {
				return landed, fmt.Errorf("%s merged, but its merge commit is unknown: %v", t.Repo, err)
			}
		} else if sha, err = Merge(dir, p.Default, t.Branch, "Merge "+t.Branch+" ("+key+")", run); err != nil {
			return landed, fmt.Errorf("%s: %w", t.Repo, err)
		}
		if len(sha) > 7 {
			sha = sha[:7]
		}
		k, err := work.New(folder, now)
		if err != nil {
			return landed, err
		}
		line := "landed " + t.Repo + " " + sha
		if t.Task != "" {
			line = "landed " + t.Task + " " + t.Repo + " " + sha
		}
		if err := k.Log(goal, line); err != nil {
			k.Writer().Abort()
			return landed, err
		}
		if _, err := k.Writer().Commit(line); err != nil {
			return landed, err
		}
		landed = append(landed, t.Repo)
	}
	return landed, nil
}

// PullRequests reports whether the project lands through pull requests (the default), or
// by a merge in the local clone, for repositories that have no forge.
func PullRequests(p *core.Project) bool {
	v := strings.ToLower(p.S(p.Project, "pull_requests"))
	return v != "false" && v != "no"
}

// Merge merges branch into the default branch base of a clone, with a merge commit, and
// returns the commit. When base is checked out, it merges there and needs a clean tree;
// else it merges in a temporary worktree and moves base only if base did not move.
func Merge(dir, base, branch, message string, run Exec) (string, error) {
	if _, err := run(dir, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		return "", fmt.Errorf("no branch %s in %s", branch, dir)
	}
	list, err := run(dir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	at, cur := "", ""
	for _, l := range strings.Split(list, "\n") {
		if strings.HasPrefix(l, "worktree ") {
			cur = strings.TrimPrefix(l, "worktree ")
		}
		if l == "branch refs/heads/"+base {
			at = cur
		}
	}
	if at != "" {
		if st, _ := run(at, "git", "status", "--porcelain", "--untracked-files=no"); st != "" {
			return "", fmt.Errorf("%s has uncommitted changes on %s; commit or stash them, then land again", at, base)
		}
		if _, err := run(at, "git", "merge", "--no-ff", "-m", message, branch); err != nil {
			run(at, "git", "merge", "--abort")
			return "", fmt.Errorf("%s does not merge cleanly into %s: %v", branch, base, err)
		}
		return run(at, "git", "rev-parse", "HEAD")
	}
	old, err := run(dir, "git", "rev-parse", "refs/heads/"+base)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "carto-land-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if _, err := run(dir, "git", "worktree", "add", "--detach", tmp, base); err != nil {
		return "", err
	}
	defer run(dir, "git", "worktree", "remove", "--force", tmp)
	if _, err := run(tmp, "git", "merge", "--no-ff", "-m", message, branch); err != nil {
		return "", fmt.Errorf("%s does not merge cleanly into %s: %v", branch, base, err)
	}
	sha, err := run(tmp, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if _, err := run(dir, "git", "update-ref", "refs/heads/"+base, sha, old); err != nil {
		return "", fmt.Errorf("%s moved during the merge; land again: %v", base, err)
	}
	return sha, nil
}
