package record

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
)

func setup(t *testing.T) (string, core.Env) {
	t.Helper()
	testvault.Identity(t)
	root := testvault.New(t)
	env, err := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "MyShop"), env
}

var later = time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)

// A check that finds a violated clause met resolves its finding and says so in History.
func TestACheckResolvesAFinding(t *testing.T) {
	folder, env := setup(t)
	out, err := Record(folder, env, Report{Target: "inventory-service", Repository: "github.com/myshop/myshop-inventory", Commit: "4d5e6f7a", Ref: "main",
		Rows: []Row{{Clause: "[[Shop Data Schema#DS-2]]", Rev: 1, Applies: "`unit_cost`, `price`", Result: "met", Evidence: "integer cents at import", DependsOn: []string{"myshop-inventory:inventory/stock/"}}}}, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Resolved) != 1 || out.Resolved[0] != "DS-2 violated in inventory-service" {
		t.Fatalf("resolved %v", out.Resolved)
	}
	f := testvault.Read(t, filepath.Join(folder, "Findings", "DS-2 violated in inventory-service.md"))
	if !strings.Contains(f, "status: resolved") || !strings.Contains(f, "resolved by [["+out.Check+"]]") {
		t.Fatalf("the finding does not show the resolution:\n%s", f)
	}
	p, _ := core.Load(folder, env)
	if c := p.Compliance("inventory-service"); c.Violated != 0 || c.Met != 10 {
		t.Fatalf("inventory-service compliance is %+v", c)
	}
}

// A new violation opens a finding; a failure that comes back reopens the same one.
func TestAViolationOpensAndReopensOneFinding(t *testing.T) {
	folder, env := setup(t)
	row := func(result, fails string) Row {
		return Row{Clause: "[[Security Policy#SEC-4]]", Rev: 1, Applies: "worker log", Result: result, Fails: fails,
			Evidence: "`invoices/worker.py:40` logs the email", DependsOn: []string{"myshop-inventory:invoices/worker.py"}}
	}
	out, err := Record(folder, env, Report{Target: "invoice-worker", Repository: "github.com/myshop/myshop-inventory", Commit: "aaaa111", Rows: []Row{row("violated", "the email in the worker log")}}, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Opened) != 1 || out.Opened[0] != "SEC-4 violated in invoice-worker" {
		t.Fatalf("opened %v", out.Opened)
	}
	if _, err := Record(folder, env, Report{Target: "invoice-worker", Repository: "github.com/myshop/myshop-inventory", Commit: "bbbb222", Rows: []Row{row("met", "")}}, later.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	out, err = Record(folder, env, Report{Target: "invoice-worker", Repository: "github.com/myshop/myshop-inventory", Commit: "cccc333", Rows: []Row{row("violated", "the email again")}}, later.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Opened) != 0 || len(out.Reopened) != 1 {
		t.Fatalf("a recurring failure should reopen, not open: opened %v, reopened %v", out.Opened, out.Reopened)
	}
	f := testvault.Read(t, filepath.Join(folder, "Findings", "SEC-4 violated in invoice-worker.md"))
	if strings.Count(f, "\n- ") < 3 {
		t.Fatalf("the History does not hold opened, resolved, and reopened:\n%s", f)
	}
}

// A report that breaks a format rule is refused, and nothing is written.
func TestBadReportsAreRefused(t *testing.T) {
	folder, env := setup(t)
	cases := map[string]Row{
		"met-none without a reason": {Clause: "[[Security Policy#SEC-3]]", Rev: 1, Applies: "nothing", Result: "met-none", DependsOn: []string{"x:y"}},
		"no dependency":             {Clause: "[[Security Policy#SEC-1]]", Rev: 1, Applies: "x", Result: "met"},
		"an unknown result":         {Clause: "[[Security Policy#SEC-1]]", Rev: 1, Applies: "x", Result: "fine", DependsOn: []string{"x:y"}},
		"not an obligation":         {Clause: "[[API Standards#API-1]]", Rev: 1, Applies: "x", Result: "met", DependsOn: []string{"x:y"}},
		"a violation with no parts": {Clause: "[[Security Policy#SEC-1]]", Rev: 1, Applies: "x", Result: "violated", DependsOn: []string{"x:y"}},
	}
	for name, row := range cases {
		if _, err := Record(folder, env, Report{Target: "invoice-worker", Commit: "x", Rows: []Row{row}}, later); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A divergence opens a finding keyed by the dependency.
func TestADivergenceOpensAFinding(t *testing.T) {
	folder, env := setup(t)
	out, err := Record(folder, env, Report{Target: "recs-server", Repository: "github.com/myshop/myshop-recs", Commit: "dddd444",
		Dependencies: []Dep{{Dependency: "`https://api.segment.io`", Result: "divergence", Evidence: "`server/track.py:3`"}}}, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Opened) != 1 || !strings.Contains(out.Opened[0], "recs-server uses api.segment.io without declaring it") {
		t.Fatalf("opened %v", out.Opened)
	}
}

// A sweep gathers sections, then its coverage opens unmapped findings.
func TestASweep(t *testing.T) {
	folder, env := setup(t)
	name, err := StartSweep(folder, later)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Record(folder, env, Report{Target: "shop-auth", Repository: "github.com/myshop/myshop-api", Commit: "eeee555", Sweep: name,
		Rows: []Row{{Clause: "[[Security Policy#SEC-4]]", Rev: 1, Applies: "auth failure log", Result: "met", Evidence: "unchanged", DependsOn: []string{"myshop-api:auth/"}}}}, later); err != nil {
		t.Fatal(err)
	}
	out, err := FinishSweep(folder, env, name, []Coverage{{Repository: "myshop-web", Entry: "`scripts/seed.ts`", Result: "unmapped", Evidence: "a script in package.json"}}, []string{"Budget ended after shop-auth."}, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Opened) != 1 {
		t.Fatalf("opened %v", out.Opened)
	}
	doc := testvault.Read(t, filepath.Join(folder, "Checks", "2026-10", name+".md"))
	for _, want := range []string{"## shop-auth", "### Obligations", "met: 1", "## Coverage", "scripts/seed.ts"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("the sweep lacks %q:\n%s", want, doc)
		}
	}
	p, _ := core.Load(folder, env)
	found := false
	for _, r := range p.ResultsOf("shop-auth", "Security Policy", "SEC-4") {
		if r.Check == name+"#shop-auth" {
			found = true
		}
	}
	if !found {
		t.Fatal("the core does not read the sweep's section as a check")
	}
}

func TestTitle(t *testing.T) {
	for in, want := range map[string]string{
		"algo-sandbox uses [[seq-run]] without declaring it":               "algo-sandbox uses seq-run without declaring it",
		"model-garden uses leaderboard.json` (repository root) without it": "model-garden uses leaderboard.json (repository root) without it",
		"tools/detect_image/main.cpp in p3-edge is in no entity":           "tools › detect_image › main.cpp in p3-edge is in no entity",
		"core-agent uses /tmp/p3-edge/<slug>/raster-<name>.png":            "core-agent uses tmp › p3-edge › slug › raster-name.png",
		"recs-server uses `https://api.segment.io`":                        "recs-server uses api.segment.io",
		"a " + strings.Repeat("word ", 30):                                 "a " + strings.TrimSpace(strings.Repeat("word ", 23)),
	} {
		if got := title(in); got != want {
			t.Errorf("%q:\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestDepKey(t *testing.T) {
	for in, want := range map[string]string{
		"`iot` (AWS IoT Core control-plane API: CreateThing)": "iot",
		"leaderboard.json (repository root)":                  "leaderboard.json",
		" hooks.slack.com ":                                   "hooks.slack.com",
		"[[Gateway Image]]":                                   "[[Gateway Image]]",
	} {
		if got := depKey(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
