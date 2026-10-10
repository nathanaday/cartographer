package view_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/view"
)

// The MyShop example was rendered by the prototype (testdata/prototype/carto.py). The Go
// core, run on the same documents and the same simulation, must render them unchanged.
func load(t *testing.T) *core.Project {
	t.Helper()
	env, err := core.LoadSim("../../testdata/prototype/simulation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := core.Load("../../testdata/vault/MyShop", env)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDocumentsRenderAsThePrototypeRendersThem(t *testing.T) {
	p := load(t)
	for name, d := range p.Docs {
		b, err := os.ReadFile(d.Path)
		if err != nil {
			t.Fatal(err)
		}
		want := string(b)
		blank := callouts.ReplaceAllString(want, "$1")
		if tf := view.ToolFields(p, name); tf != nil {
			empty := map[string]any{}
			for k := range tf {
				empty[k] = "?"
			}
			blank = md.SetFields(blank, empty)
		}
		if strings.Contains(want, "> [!example]") && strings.Contains(blank, "> [!example]") {
			t.Fatalf("%s: the test failed to strip its callouts", name)
		}
		got := view.RenderExamples(p, name, blank)
		if tf := view.ToolFields(p, name); tf != nil {
			got = md.SetFields(got, tf)
		}
		if got != want {
			t.Errorf("%s differs from the prototype:\n%s", name, diff(want, got))
		}
	}
}

func TestCanvasesMatchThePrototype(t *testing.T) {
	p := load(t)
	root, _ := filepath.Abs("../../testdata/vault")
	cases := map[string]*view.Canvas{"MyShop.canvas": view.ProjectCanvas(p, root), "Tasks.canvas": view.TasksCanvas(p, root)}
	for _, g := range p.Of("goal") {
		if p.S(g, "status") != "done" && !p.Standing(g) {
			cases["Canvases/"+g+".canvas"] = view.GoalCanvas(p, g, root)
		}
	}
	for file, cv := range cases {
		b, err := os.ReadFile(filepath.Join("../../testdata/vault/MyShop", file))
		if err != nil {
			t.Fatal(err)
		}
		want := normalize(t, string(b))
		got := normalize(t, string(cv.JSON()))
		if want != got {
			t.Errorf("%s differs from the prototype:\n%s", file, diff(want, got))
		}
	}
}

var callouts = regexp.MustCompile("(```cartographer-\\w+\n```\n)\n> \\[!example\\][^\n]*\n(?:>[^\n]*\n)*")

func normalize(t *testing.T, s string) string {
	var v any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(s, "cartographer-design/", "")), &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(v, "", " ")
	return string(b)
}

func diff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	var out []string
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			out = append(out, "  want: "+x, "  got:  "+y)
			if len(out) > 24 {
				break
			}
		}
	}
	return strings.Join(out, "\n")
}
