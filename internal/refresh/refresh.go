// Package refresh writes what the core computes back into the vault: the tool fields of
// each document, the canvases, and, for the design example, example callouts.
package refresh

import (
	"os"
	"path/filepath"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
	"github.com/nathanaday/cartographer/internal/view"
)

// Result counts what a refresh wrote.
type Result struct {
	Documents, Canvases int
	Commit              string
}

// Run writes the tool fields and the canvases of a project, and the example callouts when
// examples is true, as one commit.
func Run(p *core.Project, examples bool, message string) (Result, error) {
	w := vault.NewWriter(vault.Root(p.Folder))
	r, err := Stage(p, w, examples)
	if err != nil {
		return r, err
	}
	r.Commit, err = w.Commit(message)
	return r, err
}

// Stage stages the tool fields and canvases of a project in a writer.
func Stage(p *core.Project, w *vault.Writer, examples bool) (Result, error) {
	root := w.Root
	var r Result
	for name, d := range p.Docs {
		text, err := w.Read(d.Path)
		if err != nil {
			return r, err
		}
		next := text
		if examples {
			next = view.RenderExamples(p, name, next)
		}
		if tf := view.ToolFields(p, name); tf != nil {
			next = md.SetFields(next, tf)
		}
		if next != text {
			w.Put(d.Path, next)
			r.Documents++
		}
	}
	canvases := map[string]*view.Canvas{
		filepath.Join(p.Folder, p.Project+".canvas"): view.ProjectCanvas(p, root),
		filepath.Join(p.Folder, "Tasks.canvas"):      view.TasksCanvas(p, root),
	}
	dir := filepath.Join(p.Folder, "Canvases")
	keep := map[string]bool{}
	for _, g := range p.Of("goal") {
		if s := p.S(g, "status"); s != "done" && s != "dropped" && !p.Standing(g) {
			path := filepath.Join(dir, g+".canvas")
			canvases[path] = view.GoalCanvas(p, g, root)
			keep[path] = true
		}
	}
	for path, cv := range canvases {
		text := string(cv.JSON())
		if b, err := os.ReadFile(path); err != nil || string(b) != text {
			w.Put(path, text)
			r.Canvases++
		}
	}
	old, _ := filepath.Glob(filepath.Join(dir, "*.canvas"))
	for _, f := range old {
		if !keep[f] {
			w.Remove(f)
		}
	}
	return r, nil
}
