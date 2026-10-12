package consent

import (
	"testing"
	"time"
)

func TestDecides(t *testing.T) {
	t.Setenv("CARTO_HOME", t.TempDir())
	now := time.Date(2026, 10, 9, 19, 0, 0, 0, time.UTC)
	approve := []string{"approv", "accept"}
	reject := []string{"reject", "decline"}
	for _, c := range []struct {
		prompt string
		verbs  []string
		want   bool
	}{
		{"Approve chg-l869ex", approve, true},
		{"approve 2026-10-09 1859 Map p3-cloud please", approve, true},
		{"Approve the p3-cloud map as proposed.", approve, true},
		{"Looks good, accept the map of p3-cloud", approve, true},
		{"What does the p3-cloud map change?", approve, false},
		{"Reject the p3-cloud map", approve, false},
		{"Reject the p3-cloud map", reject, true},
		{"Approve the p3-edge map", approve, false},
		{"Approve the map", approve, false},
	} {
		t.Run(c.prompt, func(t *testing.T) {
			t.Setenv("CARTO_HOME", t.TempDir())
			Save("s1", c.prompt, now.Add(-time.Minute))
			if _, got := Decides(now, c.verbs, Change{ID: "chg-l869ex", Name: "2026-10-09 1859 Map p3-cloud", Title: "Map p3-cloud"}); got != c.want {
				t.Fatalf("got %v", got)
			}
		})
	}
	t.Setenv("CARTO_HOME", t.TempDir())
	Save("s1", "Approve the p3-cloud map", now.Add(-time.Hour))
	if _, got := Decides(now, approve, Change{ID: "chg-l869ex", Title: "Map p3-cloud"}); got {
		t.Fatal("a prompt older than the window counted")
	}

	contract := Change{ID: "chg-k2a9t3", Title: "Suggested: Camera Allotment Contract and its adopters", Creates: []string{"Camera Allotment Contract"}, Proposed: now.Add(-10 * time.Minute)}
	for prompt, want := range map[string]bool{
		"Approve the Camera Allotment Contract.":             true,
		"approve camera allotment contract and its adopters": true,
		"Approve the Camera Allotment":                       false,
		"Approve all of them":                                true,
		"Approve all but the Edge Event Bus Rules":           true,
		"Show me all of them":                                false,
	} {
		t.Setenv("CARTO_HOME", t.TempDir())
		Save("s1", prompt, now.Add(-time.Minute))
		if _, got := Decides(now, approve, contract); got != want {
			t.Errorf("%q: got %v", prompt, got)
		}
	}
	t.Setenv("CARTO_HOME", t.TempDir())
	Save("s1", "Approve all", now.Add(-20*time.Minute))
	if _, got := Decides(now, approve, contract); got {
		t.Error("approve all counted for a change proposed after it")
	}
}
