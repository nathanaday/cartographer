package mcpserver

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/nathanaday/cartographer/internal/app"
	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/vault"
)

// lintProject loads a project for lint. With a draft goal, it copies the project to a
// scratch folder, approves the goal there, and loads the copy: the map as the goal would
// leave it (Specification 5.1.4).
func lintProject(folder, goal, sim string, now time.Time) (*core.Project, error) {
	if goal == "" {
		p, _, err := app.Open(folder, sim)
		return p, err
	}
	scratch, err := os.MkdirTemp("", "carto-lint-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	if err := os.MkdirAll(filepath.Join(scratch, ".obsidian"), 0o755); err != nil {
		return nil, err
	}
	copyFolder := filepath.Join(scratch, filepath.Base(folder))
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(folder, path)
		dst := filepath.Join(copyFolder, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return nil, err
	}
	ix, err := change.NewIndex(copyFolder)
	if err != nil {
		return nil, err
	}
	w := vault.NewWriter(scratch)
	defer w.Abort()
	if _, err := change.Apply(ix, w, []change.Op{{Op: "goal.approve", Doc: goal}}, now); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	p, _, err := app.Open(copyFolder, sim)
	return p, err
}
