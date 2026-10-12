package view

import "testing"

func TestBrief(t *testing.T) {
	short := []string{"**create** contract [[Refund Policy]]", "**add** `adopts` [[Refund Policy]] on [[api]]"}
	if got := Brief(short); got != "**create** contract [[Refund Policy]]; **add** `adopts` [[Refund Policy]] on [[api]]" {
		t.Errorf("a short change: %s", got)
	}
	long := []string{
		"**create** system [[P3 Gateway]]",
		"**create** system [[P3 Cloud]]",
		"**create** contract [[Live Link Contract]]",
		"**add** `systems` [[P3 Gateway]] on [[core-agent]]",
		"**add** `systems` [[P3 Gateway]] on [[itl]]",
		"**add** `systems` [[P3 Cloud]] on [[cloud-api]]",
		"**clause** add LL-1 in [[Live Link Contract]]",
	}
	want := "**create** system ×2: [[P3 Gateway]], [[P3 Cloud]]; **create** contract [[Live Link Contract]]; " +
		"**add** `systems` [[P3 Gateway]] on 2 entities; **add** `systems` [[P3 Cloud]] on [[cloud-api]]; 1 more"
	if got := Brief(long); got != want {
		t.Errorf("a long change:\n got %s\nwant %s", got, want)
	}
}
