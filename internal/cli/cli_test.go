package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/testvault"
)

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestTheCommandsOfTheUser(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	sim := filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")
	p := []string{"--project", folder, "--sim", sim}

	if code, out, _ := run(t, "", "help"); code != 0 || !strings.Contains(out, "carto run") {
		t.Fatalf("help: %d %s", code, out)
	}
	if code, out, errs := run(t, "", append([]string{"status"}, p...)...); code != 0 || !strings.Contains(out, "## Attention") || !strings.Contains(out, "**Land** T2 of [[Upkeep]]") {
		t.Fatalf("status: %d %s %s", code, out, errs)
	}
	if code, out, _ := run(t, "", append([]string{"impact", "Shop Data Schema"}, p...)...); code != 0 || !strings.Contains(out, "shop-web: DS-1 falls on it") || strings.Contains(out, "invoice-worker") {
		t.Fatalf("impact: %d %s", code, out)
	}
	if code, _, errs := run(t, "", append([]string{"check"}, p...)...); code != 0 {
		t.Fatalf("check of a valid project: %d %s", code, errs)
	}

	ops := `[{"op":"set","doc":"shop-web","field":"runtime","value":"Node 22"}]`
	if code, out, errs := run(t, ops, append([]string{"act", "--title", "set runtime", "--ops", "-"}, p...)...); code != 0 || !strings.Contains(out, "applied") {
		t.Fatalf("act: %d %s %s", code, out, errs)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "shop-web.md")), "runtime: Node 22") {
		t.Fatal("act did not write the field")
	}
	if code, _, errs := run(t, "", append([]string{"undo"}, p...)...); code != 0 {
		t.Fatalf("undo: %d %s", code, errs)
	}
	if strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "shop-web.md")), "runtime: Node 22") {
		t.Fatal("undo did not restore the field")
	}
	if out := testvault.Git(t, root, "status", "--porcelain"); out != "" {
		t.Fatalf("the commands left the vault dirty:\n%s", out)
	}

	// An adoption changes computed fields too; undo must still take it back, and each
	// act is one commit with its record and its refresh.
	before := testvault.Git(t, root, "rev-list", "--count", "HEAD")
	ops = `[{"op":"add","doc":"shop-web","field":"adopts","value":"[[API Standards]]"}]`
	if code, _, errs := run(t, ops, append([]string{"act", "--title", "adopt API Standards", "--ops", "-"}, p...)...); code != 0 {
		t.Fatalf("act: %d %s", code, errs)
	}
	if files := testvault.Git(t, root, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(files, "Services/shop-web.md") || !strings.Contains(files, "Your edits.md") || !strings.Contains(files, "Systems/Storefront.md") {
		t.Fatalf("the act's commit holds:\n%s", files)
	}
	if code, _, errs := run(t, "", append([]string{"undo"}, p...)...); code != 0 {
		t.Fatalf("undo of an adoption: %d %s", code, errs)
	}
	if strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "shop-web.md")), "API Standards") {
		t.Fatal("undo left the adoption")
	}
	if after := testvault.Git(t, root, "rev-list", "--count", "HEAD"); atoi(after)-atoi(before) != 2 {
		t.Fatalf("an act and its undo made %d commits", atoi(after)-atoi(before))
	}

	if code, _, errs := run(t, "", append([]string{"approve", "no such change"}, p...)...); code != 1 || errs == "" {
		t.Fatalf("approve of nothing: %d %s", code, errs)
	}
	if code, _, errs := run(t, "", append([]string{"run"}, p...)...); code != 1 || !strings.Contains(errs, "does not run with --sim") {
		t.Fatalf("run with a simulation: %d %s", code, errs)
	}
	if code, _, errs := run(t, "", "nope"); code != 1 || !strings.Contains(errs, `no command "nope"`) {
		t.Fatalf("an unknown command: %d %s", code, errs)
	}
	if code, out, _ := run(t, "{}", "hook", "guard"); code != 0 || out != "" {
		t.Fatalf("a hook with empty input must pass quietly: %d %q", code, out)
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		n = n*10 + int(r-'0')
	}
	return n
}

// A card dropped into a subgroup of the project canvas is one act, and the canvas is laid
// out again; undo takes it back.
func TestACanvasGesture(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	p := []string{"--project", folder, "--sim", filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")}
	if code, _, errs := run(t, "", append([]string{"render"}, p...)...); code != 0 {
		t.Fatal(errs)
	}
	path := filepath.Join(folder, "MyShop.canvas")
	var c struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []any            `json:"edges"`
	}
	json.Unmarshal([]byte(testvault.Read(t, path)), &c)
	num := func(n map[string]any, k string) float64 { return n[k].(float64) }
	inside := func(g, n map[string]any) bool {
		return num(n, "x") >= num(g, "x") && num(n, "y") >= num(g, "y") &&
			num(n, "x")+num(n, "width") <= num(g, "x")+num(g, "width") && num(n, "y")+num(n, "height") <= num(g, "y")+num(g, "height")
	}
	var web, sub map[string]any
	for _, n := range c.Nodes {
		if n["label"] == "shop-web" {
			web = n
		}
	}
	for _, n := range c.Nodes {
		if n["label"] == "Contracts" && inside(web, n) {
			sub = n
		}
	}
	c.Nodes = append(c.Nodes, map[string]any{"id": "dropped", "type": "file", "file": "MyShop/Contracts/API Standards.md",
		"x": num(sub, "x") + 20, "y": num(sub, "y") + 30, "width": 400.0, "height": 200.0})
	b, _ := json.MarshalIndent(c, "", "\t")
	os.WriteFile(path, b, 0o644)

	code, out, errs := run(t, "", append([]string{"gesture", path, "--json"}, p...)...)
	if code != 0 {
		t.Fatal(errs)
	}
	var res struct {
		Change  string   `json:"change"`
		Summary []string `json:"summary"`
		Notes   []string `json:"notes"`
	}
	json.Unmarshal([]byte(out), &res)
	if res.Change == "" || len(res.Notes) != 0 {
		t.Fatalf("gesture: %s", out)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "shop-web.md")), "[[API Standards]]") {
		t.Fatal("the gesture did not adopt")
	}
	if strings.Contains(testvault.Read(t, path), `"dropped"`) {
		t.Fatal("the canvas was not laid out again")
	}
	if out := testvault.Git(t, root, "status", "--porcelain"); out != "" {
		t.Fatalf("dirty after a gesture:\n%s", out)
	}
	if code, _, errs := run(t, "", append([]string{"undo"}, p...)...); code != 0 {
		t.Fatalf("undo of a gesture: %s", errs)
	}
	if strings.Contains(testvault.Read(t, filepath.Join(folder, "Services", "shop-web.md")), "[[API Standards]]") {
		t.Fatal("undo left the adoption")
	}
	code, out, _ = run(t, "", append([]string{"gesture", path, "--json"}, p...)...)
	if code != 0 || !strings.Contains(out, `"change": ""`) {
		t.Fatalf("a gesture on the generated canvas: %s", out)
	}
}

// The side panel's New commands write documents that pass validation.
func TestNewEntities(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	p := []string{"--project", folder, "--sim", filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")}
	for _, args := range [][]string{
		{"new", "system", "Payments", "--description", "Takes money", "--code", "pay"},
		{"new", "service", "refund-worker", "--description", "Pays refunds", "--repository", "github.com/myshop/myshop-api", "--path", "refunds/", "--system", "Payments"},
		{"new", "module", "myshop-money", "--description", "Money types", "--repository", "github.com/myshop/myshop-api", "--path", "money/"},
		{"new", "interface", "Refund Events", "--description", "One event for each refund"},
		{"new", "contract", "Refund Policy", "--description", "Rules for refunds", "--code", "ref"},
	} {
		if code, _, errs := run(t, "", append(args, p...)...); code != 0 {
			t.Fatalf("%v: %s", args, errs)
		}
	}
	if code, out, errs := run(t, "", append([]string{"check"}, p...)...); code != 0 {
		t.Fatalf("check after new entities: %s %s", out, errs)
	}
	if code, _, errs := run(t, "", append([]string{"new", "service", "x", "--description", "y"}, p...)...); code == 0 || !strings.Contains(errs, "repository and its path") {
		t.Fatalf("a service with no repository: %s", errs)
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Systems", "Payments.md")), "code: PAY") {
		t.Fatal("the system's code")
	}
	if code, _, errs := run(t, "", append([]string{"undo"}, p...)...); code != 0 {
		t.Fatalf("undo of a new entity: %s", errs)
	}
	if _, err := os.Stat(filepath.Join(folder, "Contracts", "Refund Policy.md")); !os.IsNotExist(err) {
		t.Fatal("undo did not remove the new contract")
	}
}
