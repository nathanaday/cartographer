package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

// A new project starts with a proposal that creates its document, and the user approves it
// before the project exists.
func TestTheFirstChangeCreatesTheProject(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := t.TempDir()
	testvault.Git(t, root, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "README.md"), []byte("A vault.\n"), 0o644)
	testvault.Git(t, root, "add", "-A")
	testvault.Git(t, root, "commit", "-q", "-m", "start")
	folder := filepath.Join(root, "Acme")
	p := []string{"--project", folder}

	service := `[{"op":"create","path":"Services/api.md","text":"---\nid: svc-aaaaaa\ntype: carto-service\ndescription: The API.\nrepository: github.com/acme/api\npath: ./\n---\n"}]`
	if code, _, errs := run(t, service, append([]string{"propose", "--title", "Map api", "--ops", "-"}, p...)...); code == 0 || !strings.Contains(errs, "the first change creates it") {
		t.Fatalf("a first change without the project document: %d %s", code, errs)
	}
	project := `[{"op":"create","path":"Acme.md","text":"---\nid: prj-acme01\ntype: carto-project\ndescription: Acme's services.\nrepositories: [\"github.com/acme/api\"]\ndefault_branch: main\nlanding: user\n---\n\n## Purpose\n"}]`
	nested := strings.Replace(project, `"path":"Acme.md"`, `"path":"Sub/Acme.md"`, 1)
	if code, _, errs := run(t, nested, append([]string{"propose", "--title", "Start Acme", "--ops", "-"}, p...)...); code == 0 || !strings.Contains(errs, "at its top") {
		t.Fatalf("a project document below the project folder: %d %s", code, errs)
	}
	code, out, errs := run(t, project, append([]string{"propose", "--title", "Start Acme", "--ops", "-", "--json"}, p...)...)
	if code != 0 {
		t.Fatalf("propose the project: %s", errs)
	}
	var got struct{ ID string }
	json.Unmarshal([]byte(out), &got)
	if code, _, errs := run(t, "", append([]string{"status"}, p...)...); code == 0 || !strings.Contains(errs, "holds no carto-project document") {
		t.Fatalf("status before the approval: %d %s", code, errs)
	}
	if code, _, errs := run(t, "", append([]string{"approve", got.ID}, p...)...); code != 0 {
		t.Fatalf("approve the project: %s", errs)
	}
	if code, out, errs := run(t, "", append([]string{"status"}, p...)...); code != 0 || !strings.Contains(out, "## Attention") {
		t.Fatalf("status after the approval: %d %s %s", code, out, errs)
	}
	if code, _, errs := run(t, strings.Replace(project, "prj-acme01", "prj-acme02", 1), append([]string{"propose", "--title", "Again", "--ops", "-"}, p...)...); code == 0 || !strings.Contains(errs, "exists") {
		t.Fatalf("a second project document: %d %s", code, errs)
	}
	if status := testvault.Git(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("the vault is not clean: %s", status)
	}
}

// A proposed change that edits a document a lint result names holds the result: Attention
// asks only for the approval, and Health says where the result stands.
func TestAProposalHoldsTheLintItFixes(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	p := []string{"--project", folder, "--sim", filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")}
	if code, _, errs := run(t, "", append([]string{"new", "service", "refund-worker", "--description", "Pays refunds", "--repository", "github.com/myshop/myshop-api", "--path", "refunds/"}, p...)...); code != 0 {
		t.Fatal(errs)
	}
	lint := "lint L3: [[refund-worker]] belongs to no system"
	if _, out, _ := run(t, "", append([]string{"status"}, p...)...); !strings.Contains(out, "**Decide** on "+lint) {
		t.Fatalf("no lint line before the proposal:\n%s", out)
	}
	other := `[{"op":"add","doc":"invoice-worker","field":"adopts","value":"[[API Standards]]"}]`
	if code, _, errs := run(t, other, append([]string{"propose", "--title", "invoice-worker adopts the API Standards", "--ops", "-"}, p...)...); code != 0 {
		t.Fatal(errs)
	}
	if _, out, _ := run(t, "", append([]string{"status"}, p...)...); strings.Contains(out, "API Standards]], proposed.") {
		t.Fatalf("a proposal held a lint result it does not fix:\n%s", out)
	}
	ops := `[{"op":"add","doc":"refund-worker","field":"systems","value":"[[Back Office]]"}]`
	if code, _, errs := run(t, ops, append([]string{"propose", "--title", "refund-worker joins Back Office", "--ops", "-"}, p...)...); code != 0 {
		t.Fatal(errs)
	}
	_, out, _ := run(t, "", append([]string{"status"}, p...)...)
	if strings.Contains(out, "**Decide** on "+lint) || !strings.Contains(out, "Lint L3: [[refund-worker]] belongs to no system → [[") || !strings.Contains(out, "refund-worker joins Back Office]], proposed.") {
		t.Fatalf("after the proposal:\n%s", out)
	}
}

// Findings of one kind that share a repository form one Attention line, and one choice
// decides them all in one act.
func TestFindingsThatShareARepositoryShareALine(t *testing.T) {
	testvault.Identity(t)
	t.Setenv("CARTO_HOME", t.TempDir())
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	p := []string{"--project", folder, "--sim", filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml")}
	reindex := filepath.Join(folder, "Findings", "reindex job is in no entity.md")
	src := testvault.Read(t, reindex)
	if _, out, _ := run(t, "", append([]string{"choose", "reindex job is in no entity", "export job is in no entity", "2"}, p...)...); out != "" {
		t.Fatal("a choice for a finding that does not exist")
	}
	options := regexp.MustCompile(`(?s)## Options\n.*?\n## History`)
	src = options.ReplaceAllString(src, "## Options\n\n1. **Add it to the map** (recommended).\n2. **Waive.**\n\n## History")
	os.WriteFile(reindex, []byte(src), 0o644)
	text := strings.Replace(strings.Replace(src, "fnd-r31ndx", "fnd-grp001", 1), "server/cmd/reindex/main.go", "server/cmd/export/main.go", 1)
	os.WriteFile(filepath.Join(folder, "Findings", "export job is in no entity.md"), []byte(text), 0o644)
	testvault.Git(t, root, "add", "-A")
	testvault.Git(t, root, "commit", "-q", "-m", "a second unmapped finding")
	_, out, _ := run(t, "", append([]string{"status", "--json"}, p...)...)
	want := "**Decide** on 2 unmapped entry points in myshop-api: [[export job is in no entity|server/cmd/export/main.go]] · [[reindex job is in no entity|server/cmd/reindex/main.go]]."
	if !strings.Contains(out, want) {
		t.Fatalf("no shared line:\n%s", out)
	}
	_, out, _ = run(t, "", append([]string{"block", "attention", "MyShop", "--json"}, p...)...)
	if !strings.Contains(out, `"members": [`) {
		t.Fatalf("the line has no members:\n%s", out)
	}
	before := testvault.Git(t, root, "rev-list", "--count", "HEAD")
	if code, _, errs := run(t, "", append([]string{"choose", "reindex job is in no entity", "export job is in no entity", "2"}, p...)...); code != 0 {
		t.Fatalf("choose for both: %s", errs)
	}
	if after := testvault.Git(t, root, "rev-list", "--count", "HEAD"); atoi(after) != atoi(before)+1 {
		t.Fatalf("one choice made %d commits", atoi(after)-atoi(before))
	}
	for _, f := range []string{"reindex job is in no entity", "export job is in no entity"} {
		if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Findings", f+".md")), "[chosen:: 2]") {
			t.Fatalf("%s has no choice", f)
		}
	}
}
