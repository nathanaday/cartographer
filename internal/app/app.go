// Package app joins the machine settings, the live environment, and the core: it opens a
// project, chooses the project a request means, and finds the entity that owns a file.
package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/live"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/refresh"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Open loads a project folder with the live environment, or with a simulation file when
// sim is not empty.
func Open(folder, sim string) (*core.Project, core.Env, error) {
	var env core.Env
	if sim != "" {
		s, err := core.LoadSim(sim)
		if err != nil {
			return nil, nil, err
		}
		env = s
	} else {
		cfg, err := config.Load()
		if err != nil {
			return nil, nil, err
		}
		e, err := live.New(folder, vault.Root(folder), cfg)
		if err != nil {
			return nil, nil, err
		}
		env = e
	}
	p, err := core.Load(folder, env)
	return p, env, err
}

// Resolve returns the project folder a request means: a folder, a configured project's
// name, a folder still to make in an existing one, or, when key is empty, the only configured project or the project whose
// repositories hold the working folder.
func Resolve(key, cwd string) (string, error) {
	if key != "" {
		if st, err := os.Stat(key); err == nil && st.IsDir() {
			return filepath.Abs(key)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if key != "" {
		for _, p := range cfg.Projects {
			if filepath.Base(p) == key {
				return p, nil
			}
		}
		if abs, err := filepath.Abs(key); err == nil {
			if st, err := os.Stat(filepath.Dir(abs)); err == nil && st.IsDir() {
				return abs, nil // a new project's folder
			}
		}
		return "", fmt.Errorf("no project %q; the configured projects are %s", key, names(cfg.Projects))
	}
	if len(cfg.Projects) == 1 {
		return cfg.Projects[0], nil
	}
	if cwd != "" {
		if p, _, _ := ForPath(cfg, cwd); p != "" {
			return p, nil
		}
	}
	if len(cfg.Projects) == 0 {
		return "", errors.New("no project is configured; run carto setup --project <folder>")
	}
	return "", fmt.Errorf("name a project: %s", names(cfg.Projects))
}

func names(ps []string) string {
	var out []string
	for _, p := range ps {
		out = append(out, filepath.Base(p))
	}
	return strings.Join(out, ", ")
}

// ForPath finds the configured project whose repositories hold a file or folder. It
// returns the project folder, the repository's remote, and the path relative to the clone.
func ForPath(cfg *config.Config, path string) (string, string, string) {
	abs, _ := filepath.Abs(path)
	dir := abs
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		dir = filepath.Dir(abs)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", ""
	}
	top := strings.TrimSpace(string(out))
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	if real, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(real, filepath.Base(abs))
	}
	remote := config.RemoteOf(top)
	if remote == "" {
		return "", "", ""
	}
	rel, _ := filepath.Rel(top, abs)
	for _, p := range cfg.Projects {
		for _, r := range Repositories(p) {
			if strings.SplitN(r, "@", 2)[0] == remote {
				return p, remote, filepath.ToSlash(rel)
			}
		}
	}
	return "", remote, filepath.ToSlash(rel)
}

// Repositories returns the repositories a project document lists.
func Repositories(folder string) []string {
	docs, _ := filepath.Glob(filepath.Join(folder, "*.md"))
	for _, f := range docs {
		if d, err := md.Load(f); err == nil && d.Type == "project" {
			return md.List(d.FM, "repositories")
		}
	}
	return nil
}

// Owner returns the service or module whose path owns a file of a repository: the longest
// matching path (Specification 4.1). It returns "" when no entity owns it.
func Owner(p *core.Project, remote, rel string) string {
	type cand struct {
		name string
		n    int
	}
	var cands []cand
	for _, kind := range []string{"service", "module"} {
		for _, e := range p.Of(kind) {
			if strings.SplitN(p.S(e, "repository"), "@", 2)[0] != remote {
				continue
			}
			path := strings.TrimPrefix(strings.TrimPrefix(p.S(e, "path"), "./"), "/")
			if path == "" || path == "." || rel == strings.TrimSuffix(path, "/") || strings.HasPrefix(rel, strings.TrimSuffix(path, "/")+"/") {
				cands = append(cands, cand{e, len(path)})
			}
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].n > cands[j].n })
	return cands[0].name
}

// Writer starts a set of writes to the vault of a project folder.
func Writer(folder string) *vault.Writer { return vault.NewWriter(vault.Root(folder)) }

// CanClose reports why a goal cannot close yet: an obligation not met, a task not closed,
// or a repository not landed (Specification 5.1.6). It returns nil when it can close.
func CanClose(p *core.Project, goal string) error {
	if !p.Live(goal) {
		return fmt.Errorf("[[%s]] is not active", goal)
	}
	if m, n := p.GoalProgress(goal, time.Time{}); m != n {
		return fmt.Errorf("[[%s]] has %d of %d obligations met", goal, m, n)
	}
	if d, k := p.GoalTasks(goal, time.Time{}); d != k {
		return fmt.Errorf("[[%s]] has %d of %d tasks closed", goal, d, k)
	}
	for _, b := range p.GoalBranches(goal) {
		if b.State != "landed" {
			return fmt.Errorf("[[%s]] has not landed in %s", goal, b.Repo)
		}
	}
	return nil
}

// Commit finishes a set of writes as one commit: it writes them, stages the map's
// computed state (tool fields and canvases) that follows from them, and seals the act of
// c when c is not nil, so that Undo can take it back.
func Commit(w *vault.Writer, folder string, env core.Env, c *change.Doc, now time.Time, msg string) error {
	if err := w.Flush(); err != nil {
		return err
	}
	p, err := core.Load(folder, env)
	if err != nil {
		return err
	}
	if _, err := refresh.Stage(p, w, false); err != nil {
		return err
	}
	if c != nil {
		if err := change.Seal(w, c, now); err != nil {
			return err
		}
	}
	_, err = w.Commit(msg)
	return err
}

// Close closes a goal whose obligations are met, tasks closed, and branches landed.
func Close(p *core.Project, env core.Env, folder, goal string, now time.Time) error {
	if err := CanClose(p, goal); err != nil {
		return err
	}
	ix, err := change.NewIndex(folder)
	if err != nil {
		return err
	}
	w := Writer(folder)
	defer w.Abort()
	if _, err := change.CloseGoal(ix, w, goal, now); err != nil {
		return err
	}
	return Commit(w, folder, env, nil, now, "close "+goal)
}
