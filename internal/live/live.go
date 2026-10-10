// Package live is the environment of a real machine: git clones, the runner's claim files,
// the user's Mark seen, and the clock.
package live

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Env reads the world for one project.
type Env struct {
	VaultRoot string
	clones    map[string]string // short repository name -> local clone
	defaults  map[string]string // short repository name -> default branch
	Default   string
	SweepAt   string // "HH:MM" of the nightly sweep
	now       func() time.Time
}

// New builds the environment of a project folder. It reads the project document's
// repositories and default branch, and the machine settings for the clones.
func New(projectFolder, vaultRoot string, cfg *config.Config) (*Env, error) {
	e := &Env{VaultRoot: vaultRoot, clones: map[string]string{}, defaults: map[string]string{}, Default: "main", SweepAt: "02:00", now: Wall}
	docs, _ := filepath.Glob(filepath.Join(projectFolder, "*.md"))
	for _, path := range docs {
		d, err := md.Load(path)
		if err != nil || d.Type != "project" {
			continue
		}
		if b := md.Str(d.FM["default_branch"]); b != "" {
			e.Default = b
		}
		if s := md.Str(d.FM["sweep_at"]); s != "" {
			e.SweepAt = s
		}
		repos, _ := d.FM["repositories"].([]any)
		for _, r := range repos {
			remote, branch, _ := strings.Cut(md.Str(r), "@")
			name := filepath.Base(remote)
			if clone := cfg.Clone(remote); clone != "" {
				e.clones[name] = clone
			}
			if branch != "" {
				e.defaults[name] = branch
			}
		}
	}
	return e, nil
}

// Wall returns the local wall clock as a zone-free time, as documents write times.
func Wall() time.Time {
	t := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

func (e *Env) Now() time.Time { return e.now() }

// LastVisit reads the user's Mark seen from .carto/seen.json.
func (e *Env) LastVisit() time.Time {
	var seen map[string]string
	b, err := os.ReadFile(filepath.Join(e.VaultRoot, ".carto", "seen.json"))
	if err != nil || json.Unmarshal(b, &seen) != nil {
		return time.Time{}
	}
	return md.Time(seen[User()])
}

// MarkSeen records the user's visit now.
func (e *Env) MarkSeen() error {
	path := filepath.Join(e.VaultRoot, ".carto", "seen.json")
	seen := map[string]string{}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &seen)
	}
	seen[User()] = e.Now().Format("2006-01-02T15:04:05")
	b, _ := json.MarshalIndent(seen, "", "  ")
	w := vault.NewWriter(e.VaultRoot)
	defer w.Abort()
	w.Put(path, string(b)+"\n")
	_, err := w.Commit("mark seen")
	return err
}

// User is the name the vault keeps the user's own state under.
func User() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "user"
}

// NextSweep returns the next time of the nightly sweep.
func (e *Env) NextSweep() time.Time {
	now := e.Now()
	at, err := time.Parse("15:04", e.SweepAt)
	if err != nil {
		return time.Time{}
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), at.Hour(), at.Minute(), 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// Claims reads the runner's claim files.
func (e *Env) Claims() []core.Claim {
	files, _ := filepath.Glob(filepath.Join(e.VaultRoot, ".carto", "claims", "*.json"))
	sort.Strings(files)
	var out []core.Claim
	for _, f := range files {
		var c core.Claim
		if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &c) == nil {
			out = append(out, c)
		}
	}
	return out
}

func (e *Env) defaultOf(repo string) string {
	if b := e.defaults[repo]; b != "" {
		return b
	}
	return e.Default
}

// head returns the ref to compare against: origin/<branch> when it exists, else <branch>.
func (e *Env) head(dir, branch string) string {
	if exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "origin/"+branch).Run() == nil {
		return "origin/" + branch
	}
	return branch
}

// OnDefault reports whether the result's commit is reachable from the default branch.
func (e *Env) OnDefault(repo, commit, ref string, landed func(repo, ref string) bool) bool {
	dir := e.clones[repo]
	if dir == "" || commit == "" {
		return ref == e.defaultOf(repo) || landed(repo, ref)
	}
	return exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", commit, e.head(dir, e.defaultOf(repo))).Run() == nil
}

// ChangedSince returns the commits in <commit>..<default> that touch path.
func (e *Env) ChangedSince(repo, path string, r *core.Result, landed func(repo, ref string) bool) []string {
	dir := e.clones[repo]
	if dir == "" {
		return nil
	}
	return e.log(dir, r, e.head(dir, e.defaultOf(repo)), path)
}

// BranchChangedSince returns the commits in <commit>..<ref> that touch path.
func (e *Env) BranchChangedSince(repo, ref, path string, r *core.Result) []string {
	dir := e.clones[repo]
	if dir == "" {
		return nil
	}
	return e.log(dir, r, e.head(dir, ref), path)
}

func (e *Env) log(dir string, r *core.Result, head, path string) []string {
	args := []string{"-C", dir, "log", "--format=%H"}
	if r.Commit != "" && exec.Command("git", "-C", dir, "cat-file", "-e", r.Commit+"^{commit}").Run() == nil {
		args = append(args, r.Commit+".."+head)
	} else {
		args = append(args, "--since="+r.Date.Format("2006-01-02T15:04:05"), head)
	}
	spec := strings.TrimRight(path, "/")
	if spec == "" {
		spec = "."
	}
	args = append(args, "--", spec)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// Clone returns the local clone of a repository by its short name.
func (e *Env) Clone(repo string) string { return e.clones[repo] }
