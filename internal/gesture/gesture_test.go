package gesture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
	"github.com/nathanaday/cartographer/internal/view"
)

type edit struct {
	t     *testing.T
	p     *core.Project
	root  string
	c     canvas
	extra []json.RawMessage
}

func start(t *testing.T) *edit {
	t.Helper()
	root := filepath.Join(testvault.Repo(), "testdata", "vault")
	env, err := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := core.Load(filepath.Join(root, "MyShop"), env)
	if err != nil {
		t.Fatal(err)
	}
	e := &edit{t: t, p: p, root: root}
	if err := json.Unmarshal(view.ProjectCanvas(p, root).JSON(), &e.c); err != nil {
		t.Fatal(err)
	}
	return e
}

// group returns the index of the group with a label, inside the group named in, if any.
func (e *edit) group(label, in string) int {
	e.t.Helper()
	for i, n := range e.c.Nodes {
		if n.Type == "group" && n.Label == label && (in == "" || e.c.Nodes[e.group(in, "")].contains(n)) {
			return i
		}
	}
	e.t.Fatalf("no group %q in %q", label, in)
	return -1
}

func (e *edit) card(doc string, in int) int {
	e.t.Helper()
	for i, n := range e.c.Nodes {
		if n.Type == "file" && strings.HasSuffix(n.File, "/"+doc+".md") && (in < 0 || e.c.Nodes[in].center(n)) {
			return i
		}
	}
	e.t.Fatalf("no card of %q", doc)
	return -1
}

// put adds a copy of doc's card into a group, as a drag from the file list does.
func (e *edit) put(doc string, into int) {
	src := e.c.Nodes[e.card(doc, -1)]
	g := e.c.Nodes[into]
	src.ID, src.X, src.Y = "new-"+doc, g.X+20, g.Y+30
	e.c.Nodes = append(e.c.Nodes, src)
}

func (e *edit) remove(i int) { e.c.Nodes = append(e.c.Nodes[:i], e.c.Nodes[i+1:]...) }

func (e *edit) read() Result {
	e.t.Helper()
	b, _ := json.Marshal(map[string]any{"nodes": e.c.Nodes, "edges": e.extra})
	res, err := Read(e.p, e.root, b)
	if err != nil {
		e.t.Fatal(err)
	}
	return res
}

func ops(r Result) string {
	var out []string
	for _, o := range r.Ops {
		out = append(out, o.Op+" "+o.Doc+" "+o.Field+" "+o.Value)
	}
	return strings.Join(out, "\n")
}

func TestTheGeneratedCanvasMeansNothing(t *testing.T) {
	e := start(t)
	if r := e.read(); len(r.Ops)+len(r.Notes)+len(r.Offers) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestAddAndRemoveAContract(t *testing.T) {
	e := start(t)
	contracts := e.group("Contracts", "shop-web")
	e.put("API Standards", contracts)
	e.remove(e.card("Shop Data Schema", contracts))
	if got, want := ops(e.read()), "remove shop-web adopts [[Shop Data Schema]]\nadd shop-web adopts [[API Standards]]"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestAContractInTheWrongSubgroupGoesBack(t *testing.T) {
	e := start(t)
	e.put("API Standards", e.group("Interfaces", "shop-web"))
	r := e.read()
	if len(r.Ops) != 0 || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "API Standards goes in Contracts, not in Interfaces") {
		t.Fatalf("%+v", r)
	}
}

// invoice-worker adopts nothing, so it has no Contracts subgroup: its group takes the card.
func TestAnInterfaceOffersItsSchemas(t *testing.T) {
	e := start(t)
	e.put("Stock Levels", e.group("Interfaces", "invoice-worker"))
	r := e.read()
	if got := ops(r); got != "add invoice-worker interfaces [[Stock Levels]]" {
		t.Fatal(got)
	}
	if len(r.Offers) != 1 || r.Offers[0] != (Offer{"Shop Data Schema", "invoice-worker", "Stock Levels"}) {
		t.Fatalf("%+v", r.Offers)
	}
	e = start(t)
	e.put("Stock Levels", e.group("Interfaces", "invoice-worker"))
	e.put("Shop Data Schema", e.group("invoice-worker", ""))
	if r := e.read(); len(r.Offers) != 0 || len(r.Ops) != 2 {
		t.Fatalf("an adoption in the same act still offered: %+v", r)
	}
}

func TestMoveAServiceToAnotherSystem(t *testing.T) {
	e := start(t)
	// Storefront is the rightmost system, with three services in its row: widen it for a fourth.
	sf := &e.c.Nodes[e.group("Storefront", "")]
	sf.Width += 900
	row := e.c.Nodes[e.group("shop-api", "Storefront")].Y
	iw := e.c.Nodes[e.group("invoice-worker", "Back Office")]
	dx, dy := sf.X+40+3*900-iw.X, row-iw.Y
	for i, n := range e.c.Nodes {
		if iw.contains(n) {
			e.c.Nodes[i].X += dx
			e.c.Nodes[i].Y += dy
		}
	}
	if got, want := ops(e.read()), "remove invoice-worker systems [[Back Office]]\nadd invoice-worker systems [[Storefront]]"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestOtherEditsGoBackWithANote(t *testing.T) {
	e := start(t)
	e.extra = []json.RawMessage{json.RawMessage(`{"id":"e1","fromNode":"a","toNode":"b"}`)}
	e.c.Nodes[e.card("Security Policy", -1)].X += 30
	e.c.Nodes = append(e.c.Nodes, node{ID: "t1", Type: "text", X: 0, Y: 0, Width: 10, Height: 10})
	r := e.read()
	if len(r.Ops) != 0 || len(r.Notes) != 3 {
		t.Fatalf("%+v", r)
	}
}
