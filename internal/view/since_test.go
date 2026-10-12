package view_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
	"github.com/nathanaday/cartographer/internal/view"
)

// A goal closed since the last visit gets a line that counts its follow-ups.
func TestSinceNamesAClosedGoal(t *testing.T) {
	folder := filepath.Join(testvault.New(t), "MyShop")
	path := filepath.Join(folder, "Goals", "Add currency to order lines.md")
	text := strings.Replace(testvault.Read(t, path), "status: active", "status: done", 1)
	text = strings.Replace(text, "## Task details\n", "## Task details\n\n- After the goal lands: one.\n- After the goal lands: two.\n", 1)
	text += "- 2026-10-08 09:00 every obligation met and every task closed: the goal is done.\n"
	os.WriteFile(path, []byte(text), 0o644)
	env, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(folder, env)
	if err != nil {
		t.Fatal(err)
	}
	want := "- Closed: [[Add currency to order lines]], with 2 follow-ups for after the landing in its Task details."
	if got := strings.Join(view.Since(p, ""), "\n"); !strings.Contains(got, want) {
		t.Fatalf("Since lacks %q:\n%s", want, got)
	}
}
