// Package land merges a goal's (or an upkeep task's) branches through the forge, so that
// branch protection and required checks apply, and records each merge as a "landed" line
// of the goal's Log (Specification 5.1.5).
package land

import (
	"bytes"
	"fmt"
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
		if _, err := run(dir, "gh", "pr", "view", t.Branch, "--json", "state"); err != nil {
			return landed, fmt.Errorf("%s has no pull request for %s yet; the runner opens it after the review (carto run)", t.Repo, t.Branch)
		}
		if _, err := run(dir, "gh", "pr", "merge", t.Branch, "--merge"); err != nil {
			return landed, err
		}
		sha, err := run(dir, "gh", "pr", "view", t.Branch, "--json", "mergeCommit", "-q", ".mergeCommit.oid")
		if err != nil || sha == "" {
			return landed, fmt.Errorf("%s merged, but its merge commit is unknown: %v", t.Repo, err)
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
