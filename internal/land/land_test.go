package land

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/testvault"
)

func TestLandAnUpkeepTask(t *testing.T) {
	testvault.Identity(t)
	root := testvault.New(t)
	folder := filepath.Join(root, "MyShop")
	sim, _ := core.LoadSim(filepath.Join(testvault.Repo(), "testdata", "prototype", "simulation.yaml"))
	p, err := core.Load(folder, sim)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	fake := func(dir, name string, args ...string) (string, error) {
		calls = append(calls, dir+": "+name+" "+strings.Join(args, " "))
		if strings.Contains(strings.Join(args, " "), "mergeCommit") {
			return "3c4d5e6f00112233", nil
		}
		return "", nil
	}
	if _, _, err := Targets(p, "Add currency to order lines"); err == nil {
		t.Fatal("a goal with open tasks is ready to land")
	}
	landed, err := Land(folder, p, "Upkeep#T2", func(repo string) string { return "/clones/" + repo }, fake, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if err != nil || len(landed) != 1 {
		t.Fatalf("landed %v: %v", landed, err)
	}
	if !strings.Contains(strings.Join(calls, "\n"), "/clones/myshop-inventory: gh pr merge upkeep/t2 --merge") {
		t.Fatalf("calls: %v", calls)
	}
	p2, _ := core.Load(folder, sim)
	if !p2.Landed("myshop-inventory", "upkeep/t2") {
		t.Fatal("the landing was not recorded")
	}
	if ready, _ := p2.ReadyToLand("Upkeep"); len(ready) != 0 {
		t.Fatalf("Upkeep still has %d tasks to land", len(ready))
	}
	if !strings.Contains(testvault.Read(t, filepath.Join(folder, "Goals", "Upkeep.md")), "landed T2 myshop-inventory 3c4d5e6") {
		t.Fatal("no landed line in the Log")
	}
}
