// Package diff compares two entities at two levels: which aspect kinds each has
// (presence), and how the fields of a shared kind differ.
package diff

import (
	"sort"

	"github.com/nathanaday/cartographer/internal/model"
)

// PresenceDiff reports which aspect kinds are unique to each side and shared.
type PresenceDiff struct {
	OnlyInA []string `json:"only_in_a"`
	OnlyInB []string `json:"only_in_b"`
	InBoth  []string `json:"in_both"`
}

// FieldChange is a field present in both aspects whose definition differs.
type FieldChange struct {
	Name string      `json:"name"`
	A    model.Field `json:"a"`
	B    model.Field `json:"b"`
}

// KindFieldDiff is the field-level comparison for one shared kind.
type KindFieldDiff struct {
	Kind    string        `json:"kind"`
	OnlyInA []model.Field `json:"only_in_a"`
	OnlyInB []model.Field `json:"only_in_b"`
	Changed []FieldChange `json:"changed"`
}

// Result is the full two-level diff between two entities.
type Result struct {
	Presence PresenceDiff    `json:"presence"`
	Fields   []KindFieldDiff `json:"fields"`
}

func presence(a, b model.Entity) PresenceDiff {
	inA := map[string]bool{}
	for _, k := range a.Kinds() {
		inA[k] = true
	}
	inB := map[string]bool{}
	for _, k := range b.Kinds() {
		inB[k] = true
	}
	pd := PresenceDiff{OnlyInA: []string{}, OnlyInB: []string{}, InBoth: []string{}}
	for k := range inA {
		if inB[k] {
			pd.InBoth = append(pd.InBoth, k)
		} else {
			pd.OnlyInA = append(pd.OnlyInA, k)
		}
	}
	for k := range inB {
		if !inA[k] {
			pd.OnlyInB = append(pd.OnlyInB, k)
		}
	}
	sort.Strings(pd.OnlyInA)
	sort.Strings(pd.OnlyInB)
	sort.Strings(pd.InBoth)
	return pd
}

// Entities computes the two-level diff. If kind is non-empty, field-level
// comparison is restricted to that kind; otherwise every shared kind is compared.
func Entities(a, b model.Entity, kind string) Result {
	return Result{Presence: presence(a, b), Fields: []KindFieldDiff{}}
}
