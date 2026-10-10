// Package gesture reads the user's edits of a structure canvas as declared acts
// (Specification 7.3). It reads the relations a canvas shows from its geometry and its
// labels alone, for the canvas the core generates and for the one the user edited, and
// returns the difference as operations. Everything else the user changed is undone by the
// next layout, and each kind of it gets one note.
package gesture

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/view"
)

// Offer is an adoption the user may want with an attachment: a schema contract of an
// interface that the service does not adopt yet.
type Offer struct {
	Contract string `json:"contract"`
	Entity   string `json:"entity"`
	Because  string `json:"because"`
}

// Result is what a canvas edit means.
type Result struct {
	Ops    []change.Op `json:"ops"`
	Notes  []string    `json:"notes"`
	Offers []Offer     `json:"offers"`
}

type node struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	File   string  `json:"file"`
	Label  string  `json:"label"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type canvas struct {
	Nodes []node            `json:"nodes"`
	Edges []json.RawMessage `json:"edges"`
}

func (n node) contains(m node) bool {
	return m.X >= n.X && m.Y >= n.Y && m.X+m.Width <= n.X+n.Width && m.Y+m.Height <= n.Y+n.Height
}

func (n node) center(m node) bool {
	cx, cy := m.X+m.Width/2, m.Y+m.Height/2
	return cx >= n.X && cy >= n.Y && cx <= n.X+n.Width && cy <= n.Y+n.Height
}

// relation is one link the canvas shows: entity, field, target.
type relation struct{ entity, field, target string }

var fieldOf = map[string]string{"contract": "adopts", "interface": "interfaces", "module": "uses"}
var subgroupOf = map[string]string{"adopts": "Contracts", "interfaces": "Interfaces", "uses": "Modules"}

type reader struct {
	p     *core.Project
	files map[string]string // vault-relative path -> document name
	notes map[string]bool
}

func (r *reader) note(s string) { r.notes[s] = true }

// innermost returns the smallest group that holds m by test, other than m itself.
func innermost(groups []node, m node, holds func(g, m node) bool) (node, bool) {
	var best node
	found := false
	for _, g := range groups {
		if g.ID == m.ID || !holds(g, m) || (g.Width == m.Width && g.Height == m.Height && g.X == m.X && g.Y == m.Y) {
			continue
		}
		if !found || g.Width*g.Height < best.Width*best.Height {
			best, found = g, true
		}
	}
	return best, found
}

// kind classifies a group: project, system, service, module, area (the Modules area of
// the project), or subgroup; with its entity, or the owner of a subgroup.
func (r *reader) kind(groups []node, g node) (string, string) {
	switch r.p.Type(g.Label) {
	case "project", "system", "service", "module":
		return r.p.Type(g.Label), g.Label
	}
	if g.Label != "Contracts" && g.Label != "Interfaces" && g.Label != "Modules" {
		return "", ""
	}
	parent, ok := innermost(groups, g, node.contains)
	if !ok {
		return "", ""
	}
	pk, pe := r.kind(groups, parent)
	if g.Label == "Modules" && pk == "project" {
		return "area", ""
	}
	if pk == "subgroup" || pk == "area" || pk == "" {
		return "", ""
	}
	return "subgroup", pe
}

// relations reads the links a canvas shows.
func (r *reader) relations(c canvas, edited bool) map[relation]bool {
	var groups []node
	for _, n := range c.Nodes {
		if n.Type == "group" {
			groups = append(groups, n)
		}
	}
	out := map[relation]bool{}
	for _, n := range c.Nodes {
		switch n.Type {
		case "group":
			if r.p.Type(n.Label) != "service" {
				continue
			}
			if parent, ok := innermost(groups, n, node.contains); ok {
				if k, sy := r.kind(groups, parent); k == "system" {
					out[relation{n.Label, "systems", sy}] = true
				} else if edited && k != "project" {
					r.note(fmt.Sprintf("A service group belongs in a system group; %s was put back.", n.Label))
				}
			}
		case "file":
			doc, ok := r.files[n.File]
			if !ok {
				if edited {
					r.note("Cards of files outside the map are not kept.")
				}
				continue
			}
			field := fieldOf[r.p.Type(doc)]
			g, ok := innermost(groups, n, node.center)
			if !ok {
				if edited && field != "" {
					r.note(fmt.Sprintf("%s is on no entity; drop it into an entity's group to link it.", doc))
				}
				continue
			}
			k, e := r.kind(groups, g)
			if e == doc || field == "" {
				continue // an entity's own card, or a card that is no link
			}
			if k == "subgroup" && subgroupOf[field] != g.Label {
				if edited {
					r.note(fmt.Sprintf("%s goes in %s, not in %s; it was put back.", doc, subgroupOf[field], g.Label))
				}
				continue
			}
			switch {
			case k == "" || k == "area":
				if edited {
					r.note(fmt.Sprintf("%s is on no entity; drop it into an entity's group to link it.", doc))
				}
				continue
			case field != "adopts" && r.p.Type(e) != "service":
				if edited {
					r.note(fmt.Sprintf("Only a service has %s; %s was put back.", field, doc))
				}
				continue
			}
			out[relation{e, field, doc}] = true
		case "text", "link":
			if edited {
				r.note("Text and link cards are not kept on a structure canvas.")
			}
		}
	}
	if edited && len(c.Edges) > 0 {
		r.note("A structure canvas has no lines; groups show every relation.")
	}
	return out
}

// Read compares the canvas the user edited with the one the core generates for the
// project, and returns the operations the edit means.
func Read(p *core.Project, vaultRoot string, edited []byte) (Result, error) {
	var got, want canvas
	if err := json.Unmarshal(edited, &got); err != nil {
		return Result{}, fmt.Errorf("the canvas is not valid JSON: %w", err)
	}
	if err := json.Unmarshal(view.ProjectCanvas(p, vaultRoot).JSON(), &want); err != nil {
		return Result{}, err
	}
	r := &reader{p: p, files: map[string]string{}, notes: map[string]bool{}}
	for name, d := range p.Docs {
		if rel, err := filepath.Rel(vaultRoot, d.Path); err == nil {
			r.files[filepath.ToSlash(rel)] = name
		}
	}
	before, after := r.relations(want, false), r.relations(got, true)
	res := Result{Ops: []change.Op{}, Notes: []string{}, Offers: []Offer{}}
	var added, removed []relation
	for rel := range after {
		if !before[rel] {
			added = append(added, rel)
		}
	}
	for rel := range before {
		if !after[rel] {
			removed = append(removed, rel)
		}
	}
	order := func(xs []relation) {
		sort.Slice(xs, func(i, j int) bool {
			return xs[i].entity+"\x00"+xs[i].field+"\x00"+xs[i].target < xs[j].entity+"\x00"+xs[j].field+"\x00"+xs[j].target
		})
	}
	order(added)
	order(removed)
	for _, rel := range removed {
		res.Ops = append(res.Ops, change.Op{Op: "remove", Doc: rel.entity, Field: rel.field, Value: "[[" + rel.target + "]]"})
	}
	for _, rel := range added {
		res.Ops = append(res.Ops, change.Op{Op: "add", Doc: rel.entity, Field: rel.field, Value: "[[" + rel.target + "]]"})
		if rel.field == "interfaces" {
			adopted := map[string]bool{}
			for _, c := range p.Adopts(rel.entity) {
				adopted[c] = true
			}
			for _, s := range md.Names(p.FM(rel.target, "schemas")) {
				if !adopted[s] && !after[relation{rel.entity, "adopts", s}] {
					res.Offers = append(res.Offers, Offer{s, rel.entity, rel.target})
				}
			}
		}
	}
	if len(res.Ops) == 0 && moved(got, want) {
		r.note("Cartographer lays out the canvas; a card or group moved inside its group goes back to its place.")
	}
	for n := range r.notes {
		res.Notes = append(res.Notes, n)
	}
	sort.Strings(res.Notes)
	return res, nil
}

// moved reports whether a node of the edited canvas is not where the layout puts it.
func moved(got, want canvas) bool {
	at := map[string]node{}
	for _, n := range want.Nodes {
		at[n.ID] = n
	}
	for _, n := range got.Nodes {
		if w, ok := at[n.ID]; ok && (w.X != n.X || w.Y != n.Y || w.Width != n.Width || w.Height != n.Height) {
			return true
		}
	}
	return false
}

// Title names the act of a set of operations.
func Title(ops []change.Op) string {
	var parts []string
	for _, o := range ops {
		verb := map[string]string{"add": "add", "remove": "remove"}[o.Op]
		parts = append(parts, fmt.Sprintf("%s %s %s %s", verb, strings.Trim(o.Value, "[]"), map[string]string{"add": "to", "remove": "from"}[o.Op], o.Doc+" "+o.Field))
	}
	return "canvas: " + strings.Join(parts, "; ")
}
