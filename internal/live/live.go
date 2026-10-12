// Package live is the environment of a real machine: git clones, the runner's claim files,
// the user's Mark seen, and the clock.
package live

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Env reads the world for one project, at one moment: it keeps what git said, so a new
// load of the project makes a new Env.
type Env struct {
	VaultRoot string
	clones    map[string]string // short repository name -> local clone
	defaults  map[string]string // short repository name -> default branch
	Default   string
	SweepAt   string // "HH:MM" of the nightly sweep
	local     bool   // pull_requests: false: work lands in the clones' own branches, not on origin
	now       func() time.Time

	// What git said, for the life of this Env: one project load asks the same questions
	// for many results, and each answer costs a process.
	mu      sync.Mutex
	answers map[string]string
	ranges  map[string][]commitFiles
}

type commitFiles struct {
	sha   string
	files []string
}

// ask runs git once for each distinct question and keeps its output; "" with false is a failure.
func (e *Env) ask(args ...string) (string, bool) {
	key := strings.Join(args, "\x00")
	e.mu.Lock()
	if out, ok := e.answers[key]; ok {
		e.mu.Unlock()
		return strings.TrimPrefix(out, "\x00"), out != "\x00fail"
	}
	e.mu.Unlock()
	b, err := exec.Command("git", args...).Output()
	out := string(b)
	if err != nil {
		out = "\x00fail"
	}
	e.mu.Lock()
	if e.answers == nil {
		e.answers = map[string]string{}
	}
	e.answers[key] = out
	e.mu.Unlock()
	return strings.TrimPrefix(out, "\x00"), err == nil
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
		v := strings.ToLower(md.Str(d.FM["pull_requests"]))
		e.local = v == "false" || v == "no"
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
	if e.local {
		return branch
	}
	if _, ok := e.ask("-C", dir, "rev-parse", "--verify", "--quiet", "origin/"+branch); ok {
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
	_, ok := e.ask("-C", dir, "merge-base", "--is-ancestor", commit, e.head(dir, e.defaultOf(repo)))
	return ok
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
	var span []string
	if _, ok := e.ask("-C", dir, "cat-file", "-e", r.Commit+"^{commit}"); r.Commit != "" && ok {
		span = []string{r.Commit + ".." + head}
	} else {
		span = []string{"--since=" + r.Date.Format("2006-01-02T15:04:05"), head}
	}
	var out []string
	for _, c := range e.commits(dir, span) {
		for _, f := range c.files {
			if pathspec(strings.TrimRight(path, "/"), f) {
				out = append(out, c.sha)
				break
			}
		}
	}
	return out
}

// commits returns the commits of a span with the files each one changed, newest first.
func (e *Env) commits(dir string, span []string) []commitFiles {
	key := dir + "\x00" + strings.Join(span, "\x00")
	e.mu.Lock()
	cached, ok := e.ranges[key]
	e.mu.Unlock()
	if ok {
		return cached
	}
	b, err := exec.Command("git", append([]string{"-C", dir, "log", "--format=%x01%H", "--name-only"}, span...)...).Output()
	var out []commitFiles
	if err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			switch {
			case strings.HasPrefix(l, "\x01"):
				out = append(out, commitFiles{sha: l[1:]})
			case l != "" && len(out) > 0:
				out[len(out)-1].files = append(out[len(out)-1].files, l)
			}
		}
	}
	e.mu.Lock()
	if e.ranges == nil {
		e.ranges = map[string][]commitFiles{}
	}
	e.ranges[key] = out
	e.mu.Unlock()
	return out
}

// pathspec reports whether a changed file falls under a path as git reads a pathspec: the
// whole tree ("" or "."), the file itself, a folder that holds it, or a glob whose * also
// matches a slash.
func pathspec(spec, file string) bool {
	switch {
	case spec == "" || spec == ".":
		return true
	case strings.ContainsAny(spec, "*?["):
		var b strings.Builder
		b.WriteString("^")
		for _, r := range spec {
			switch r {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteString(".")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
		re, err := regexp.Compile(b.String() + "(/.*)?$")
		return err == nil && re.MatchString(file)
	}
	return file == spec || strings.HasPrefix(file, spec+"/")
}

// Clone returns the local clone of a repository by its short name.
func (e *Env) Clone(repo string) string { return e.clones[repo] }
