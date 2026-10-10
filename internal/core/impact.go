package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/cartographer/internal/md"
)

// Hit is one entity a change reaches, and why.
type Hit struct {
	Entity string `json:"entity"`
	Why    string `json:"why"`
}

// Impact returns every entity that a contract, a clause (Contract#ID), an interface, or a
// module reaches (Specification 1.3).
func (p *Project) Impact(target string) ([]Hit, error) {
	doc, anchor, _ := strings.Cut(target, "#")
	doc = strings.TrimSuffix(strings.TrimPrefix(doc, "[["), "]]")
	anchor = strings.TrimSuffix(anchor, "]]")
	set := map[string][]string{}
	add := func(e, why string) {
		for _, w := range set[e] {
			if w == why {
				return
			}
		}
		set[e] = append(set[e], why)
	}
	switch p.Type(doc) {
	case "contract", "system", "service", "module", "goal":
		if p.Type(doc) == "module" && anchor == "" {
			for _, s := range p.Of("service") {
				if contains(md.Names(p.FM(s, "uses")), doc) {
					add(s, "uses "+doc)
				}
			}
		}
		for _, o := range p.Obligations {
			if o.Clause.Owner != doc || (anchor != "" && o.Clause.ID != anchor) {
				continue
			}
			why := o.Clause.ID + " falls on it"
			if o.Pending() {
				why += " (pending, for " + o.Goal + ")"
			}
			add(o.Entity, why)
		}
	case "interface":
		for _, s := range p.Of("service") {
			if contains(md.Names(p.FM(s, "interfaces")), doc) {
				add(s, "attached to "+doc)
			}
		}
		for _, sc := range md.Names(p.FM(doc, "schemas")) {
			for _, o := range p.Obligations {
				if o.Clause.Owner == sc {
					add(o.Entity, "adopts its schema "+sc)
				}
			}
		}
	default:
		return nil, fmt.Errorf("%q is not a contract, clause, interface, or module of the project", target)
	}
	var out []Hit
	for _, e := range sortedKeys(func() map[string]bool {
		m := map[string]bool{}
		for k := range set {
			m[k] = true
		}
		return m
	}()) {
		sort.Strings(set[e])
		out = append(out, Hit{e, strings.Join(set[e], "; ")})
	}
	return out, nil
}
