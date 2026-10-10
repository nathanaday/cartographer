package view

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
)

// Sizes of the canvas layout (Specification 7.6).
const (
	groupW = 820
	docW   = 700
	docH   = 260
	subW   = 697
	cardW  = 637
	gap    = 80
)

var cardH = map[string]int{"Contracts": 200, "Interfaces": 160, "Modules": 160, "Clauses": 160}

// Node is one node of an Obsidian canvas.
type Node struct {
	ID, Type, File, Label, Subpath, Color string
	X, Y, W, H                            int
}

// Canvas is an Obsidian canvas under construction.
type Canvas struct {
	p     *core.Project
	root  string
	nodes []Node
	ids   map[string]bool
}

// NewCanvas starts a canvas whose file paths are relative to the vault root.
func NewCanvas(p *core.Project, vaultRoot string) *Canvas {
	return &Canvas{p: p, root: vaultRoot, ids: map[string]bool{}}
}

func (c *Canvas) nid(parts ...string) string {
	h := md5.Sum([]byte(strings.Join(parts, "/")))
	id := hex.EncodeToString(h[:])[:16]
	for c.ids[id] {
		h = md5.Sum([]byte(id))
		id = hex.EncodeToString(h[:])[:16]
	}
	c.ids[id] = true
	return id
}

func (c *Canvas) card(ctx, name string, x, y, w, h int, color, subpath string) {
	rel, err := filepath.Rel(c.root, c.p.Docs[name].Path)
	if err != nil {
		rel = c.p.Docs[name].Path
	}
	c.nodes = append(c.nodes, Node{ID: c.nid(ctx, name, subpath), Type: "file", File: filepath.ToSlash(rel), X: x, Y: y, W: w, H: h, Color: color, Subpath: subpath})
}

func (c *Canvas) group(ctx, label string, x, y, w, h int) {
	c.nodes = append(c.nodes, Node{ID: c.nid(ctx, "group", label), Type: "group", Label: label, X: x, Y: y, W: w, H: h})
}

func subH(label string, k int) int { return 80 + k*(cardH[label]+60) }

type cardItem struct{ name, subpath, color string }

func (c *Canvas) subgroup(ctx, label string, items []cardItem, x, y int, owner string) int {
	h := subH(label, len(items))
	c.group(ctx, label, x, y, subW, h)
	for i, it := range items {
		color := it.color
		if label == "Contracts" && owner != "" && c.p.ContractRed(it.name, owner) {
			color = "1"
		}
		c.card(ctx+"/"+label, it.name, x+20, y+80+i*(cardH[label]+60), cardW, cardH[label], color, it.subpath)
	}
	return h
}

type part struct {
	label string
	items []string
}

type entityItem struct {
	name  string
	parts []part
	color string
}

func (c *Canvas) entity(ctx string, e entityItem, x, y int, dry bool) int {
	cy := y + 400
	type laid struct {
		p part
		y int
	}
	var layout []laid
	for _, pt := range e.parts {
		if len(pt.items) > 0 {
			layout = append(layout, laid{pt, cy})
			cy += subH(pt.label, len(pt.items)) + gap
		}
	}
	h := 50 + docH + 50
	if len(layout) > 0 {
		h = cy - gap + 60 - y
	}
	if dry {
		return h
	}
	c.group(ctx, e.name, x, y, groupW, h)
	c.card(ctx, e.name, x+40, y+50, docW, docH, e.color, "")
	for _, l := range layout {
		items := make([]cardItem, len(l.p.items))
		for i, n := range l.p.items {
			items[i] = cardItem{name: n}
		}
		c.subgroup(ctx+"/"+e.name, l.p.label, items, x+43, l.y, e.name)
	}
	return h
}

func (c *Canvas) rows(ctx string, items []entityItem, x, y int, dry bool) (int, int) {
	w, h := 0, 0
	for r := 0; r < len(items); r += 4 {
		end := r + 4
		if end > len(items) {
			end = len(items)
		}
		row := items[r:end]
		rh := 0
		for _, it := range row {
			if v := c.entity("", it, 0, 0, true); v > rh {
				rh = v
			}
		}
		if !dry {
			for i, it := range row {
				c.entity(ctx, it, x+i*(groupW+gap), y+h, false)
			}
		}
		if rw := len(row)*groupW + (len(row)-1)*gap; rw > w {
			w = rw
		}
		h += rh + gap
	}
	return w, h - gap
}

// JSON writes the canvas as Obsidian writes it: groups first, larger before smaller.
func (c *Canvas) JSON() []byte {
	var groups, files []Node
	for _, n := range c.nodes {
		if n.Type == "group" {
			groups = append(groups, n)
		} else {
			files = append(files, n)
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].W*groups[i].H > groups[j].W*groups[j].H })
	var b bytes.Buffer
	b.WriteString("{\n\t\"nodes\": [")
	for i, n := range append(groups, files...) {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n\t\t{")
		fields := [][2]any{{"id", n.ID}, {"type", n.Type}}
		if n.Type == "group" {
			fields = append(fields, [2]any{"label", n.Label})
		} else {
			fields = append(fields, [2]any{"file", n.File})
		}
		fields = append(fields, [2]any{"x", n.X}, [2]any{"y", n.Y}, [2]any{"width", n.W}, [2]any{"height", n.H})
		if n.Subpath != "" {
			fields = append(fields, [2]any{"subpath", n.Subpath})
		}
		if n.Color != "" {
			fields = append(fields, [2]any{"color", n.Color})
		}
		for k, f := range fields {
			if k > 0 {
				b.WriteString(",")
			}
			v, _ := json.Marshal(f[1])
			b.WriteString("\n\t\t\t\"" + f[0].(string) + "\": " + string(v))
		}
		b.WriteString("\n\t\t}")
	}
	b.WriteString("\n\t],\n\t\"edges\": []\n}")
	return b.Bytes()
}

// Count returns the number of groups and cards.
func (c *Canvas) Count() (int, int) {
	g := 0
	for _, n := range c.nodes {
		if n.Type == "group" {
			g++
		}
	}
	return g, len(c.nodes) - g
}

func serviceParts(p *core.Project, s string) []part {
	return []part{{"Contracts", p.Adopts(s)}, {"Interfaces", md.Names(p.FM(s, "interfaces"))}, {"Modules", md.Names(p.FM(s, "uses"))}}
}

// ProjectCanvas is the structure of the whole project (Specification 7.2).
func ProjectCanvas(p *core.Project, vaultRoot string) *Canvas {
	c := NewCanvas(p, vaultRoot)
	type block struct {
		sy           string
		items        []entityItem
		own          []string
		header, w, h int
	}
	var blocks []block
	for _, sy := range p.Of("system") {
		var items []entityItem
		for _, s := range p.Members(sy) {
			items = append(items, entityItem{s, serviceParts(p, s), p.Color(s)})
		}
		w, h := c.rows("", items, 0, 0, true)
		own := p.Adopts(sy)
		header := 50 + docH
		if len(own) > 0 && 60+subH("Contracts", len(own)) > header {
			header = 60 + subH("Contracts", len(own))
		}
		extra := 0
		if len(own) > 0 {
			extra = subW + 80
		}
		bw := w
		if docW+extra > bw {
			bw = docW + extra
		}
		blocks = append(blocks, block{sy, items, own, header, bw + 80, header + 100 + h + 60})
	}
	pad := p.Adopts(p.Project)
	header := 50 + docH
	if v := 60 + subH("Contracts", len(pad)); v > header {
		header = v
	}
	x, y := 40, header+120
	maxH := 0
	for _, b := range blocks {
		c.group("sys", b.sy, x, y, b.w, b.h)
		c.card("sys/"+b.sy, b.sy, x+40, y+50, docW, docH, p.Color(b.sy), "")
		if len(b.own) > 0 {
			items := make([]cardItem, len(b.own))
			for i, n := range b.own {
				items[i] = cardItem{name: n}
			}
			c.subgroup("sys/"+b.sy, "Contracts", items, x+40+docW+80, y+60, b.sy)
		}
		c.rows("sys/"+b.sy, b.items, x+40, y+b.header+100, false)
		x += b.w + 120
		if b.h > maxH {
			maxH = b.h
		}
	}
	my := y + maxH + 160
	var mods []entityItem
	for _, m := range p.Of("module") {
		mods = append(mods, entityItem{m, []part{{"Contracts", p.Adopts(m)}}, p.Color(m)})
	}
	mw, mh := c.rows("mod", mods, 0, 0, true)
	c.group("prj", "Modules", 40, my-20, mw+80, mh+100)
	c.rows("mod", mods, 80, my+40, false)
	pw := x - 120 + 40
	if mw+160 > pw {
		pw = mw + 160
	}
	c.group("prj", p.Project, 0, 0, pw, my+mh+140)
	c.card("prj", p.Project, 40, 50, docW, docH, p.Color(p.Project), "")
	items := make([]cardItem, len(pad))
	for i, n := range pad {
		items[i] = cardItem{name: n}
	}
	c.subgroup("prj/"+p.Project, "Contracts", items, 40+docW+80, 60, p.Project)
	return c
}

// GoalCanvas is a goal's footprint (Specification 7.4).
func GoalCanvas(p *core.Project, g, vaultRoot string) *Canvas {
	c := NewCanvas(p, vaultRoot)
	obs := p.GoalObligations(g)
	var keys [][2]string
	var targets []string
	seenK, seenT := map[string]bool{}, map[string]bool{}
	for _, o := range obs {
		if k := o.Clause.Key(); !seenK[k] {
			seenK[k] = true
			keys = append(keys, [2]string{o.Clause.Owner, o.Clause.ID})
		}
		if !seenT[o.Entity] {
			seenT[o.Entity] = true
			targets = append(targets, o.Entity)
		}
	}
	for _, t := range p.Tasks[g] {
		if _, ok := p.Docs[t.Target]; ok && !seenT[t.Target] {
			seenT[t.Target] = true
			targets = append(targets, t.Target)
		}
	}
	color := func(e string) string {
		var st []string
		for _, o := range obs {
			if o.Entity != e {
				continue
			}
			switch {
			case p.GoalMet(o, zero):
				st = append(st, "met")
			case o.Status == "violated":
				st = append(st, "violated")
			default:
				st = append(st, "unverified")
			}
		}
		switch {
		case len(st) == 0:
			return ""
		case contains(st, "violated"):
			return "1"
		case contains(st, "unverified"):
			return "3"
		}
		return "4"
	}
	c.card("goal", g, 40, 50, docW, docH, "6", "")
	clauses := make([]cardItem, len(keys))
	for i, k := range keys {
		clauses[i] = cardItem{name: k[0], subpath: "#" + k[1]}
	}
	h1 := c.subgroup("goal", "Clauses", clauses, 40+docW+80, 60, "")
	top := 50 + docH
	if 60+h1 > top {
		top = 60 + h1
	}
	top += 100
	rowsN := (len(targets) + 3) / 4
	fh := 80 + rowsN*(docH+60)
	c.group("goal", "Footprint", 40, top, 4*(docW+60)+20, fh)
	for i, e := range targets {
		c.card("foot", e, 60+(i%4)*(docW+60), top+80+(i/4)*(docH+60), docW, docH, color(e), "")
	}
	w := 4*(docW+60) + 100
	if docW+subW+200 > w {
		w = docW + subW + 200
	}
	c.group("goal", g, 0, 0, w, top+fh+60)
	return c
}

// TasksCanvas is the open work of every goal, in lanes (Specification 7.5).
func TasksCanvas(p *core.Project, vaultRoot string) *Canvas {
	c := NewCanvas(p, vaultRoot)
	type entry struct {
		kind    string // q, run, t, goal
		name    string
		task    *core.Task
		blocked []*core.Task
		claim   core.Claim
	}
	order := []string{"Waiting on you", "Stuck", "Running", "Ready", "Landing"}
	lanes := map[string][]entry{}
	claims := map[string]core.Claim{}
	for _, cl := range p.ClaimsOf("") {
		claims[cl.Goal+"|"+cl.Task] = cl
	}
	for _, q := range p.Of("question") {
		var blocked []*core.Task
		for _, g := range p.Of("goal") {
			if !p.Live(g) {
				continue
			}
			for _, t := range p.Tasks[g] {
				if t.Box == " " && contains(md.Names(t.F["blocked"]), q) {
					blocked = append(blocked, t)
				}
			}
		}
		if p.S(q, "status") == "open" && len(blocked) > 0 {
			lanes["Waiting on you"] = append(lanes["Waiting on you"], entry{kind: "q", name: q, blocked: blocked})
		}
	}
	for _, g := range p.Of("goal") {
		if !p.Live(g) {
			continue
		}
		for _, t := range p.Tasks[g] {
			if t.Box != " " {
				continue
			}
			if core.Attempts(t) >= 2 {
				lanes["Stuck"] = append(lanes["Stuck"], entry{kind: "t", task: t})
			} else if cl, ok := claims[g+"|"+t.ID]; ok {
				lanes["Running"] = append(lanes["Running"], entry{kind: "run", task: t, claim: cl})
			}
		}
		for _, t := range p.Ready(g) {
			lanes["Ready"] = append(lanes["Ready"], entry{kind: "t", task: t})
		}
		tasks, whole := p.ReadyToLand(g)
		for _, t := range tasks {
			lanes["Landing"] = append(lanes["Landing"], entry{kind: "t", task: t})
		}
		if whole {
			lanes["Landing"] = append(lanes["Landing"], entry{kind: "goal", name: g})
		}
	}
	const LW, TH, SH = 760, 140, 200
	x := 0
	for _, lane := range order {
		items := lanes[lane]
		if len(items) == 0 {
			continue
		}
		y := 80
		type placed struct {
			e    entry
			y, h int
		}
		var parts []placed
		for _, it := range items {
			var h int
			switch it.kind {
			case "q":
				h = 60 + SH + len(it.blocked)*(TH+20) + 40
			case "run":
				h = 60 + TH + 20 + SH + 40
			case "t":
				h = TH
			default:
				h = SH
			}
			parts = append(parts, placed{it, y, h})
			y += h + 40
		}
		c.group("lanes", lane, x, 0, LW, y+20)
		for _, pl := range parts {
			it := pl.e
			switch it.kind {
			case "q":
				var ids []string
				for _, t := range it.blocked {
					ids = append(ids, t.ID)
				}
				label := "Blocks " + strings.Join(ids, ", ")
				c.nodes = append(c.nodes, Node{ID: c.nid("q", "group", label+it.name), Type: "group", Label: label, X: x + 20, Y: pl.y, W: LW - 40, H: pl.h})
				c.card("q/"+it.name, it.name, x+40, pl.y+40, LW-80, SH, "2", "")
				for k, t := range it.blocked {
					c.card("q/"+it.name, t.Goal, x+40, pl.y+60+SH+k*(TH+20), LW-80, TH, "", "#^"+t.Block)
				}
			case "run":
				label := it.task.ID + " · " + it.claim.Repo
				c.group("run", label, x+20, pl.y, LW-40, pl.h)
				c.card("run/"+label, it.task.Goal, x+40, pl.y+40, LW-80, TH, "6", "#^"+it.task.Block)
				if _, ok := p.Docs[it.claim.Log]; ok {
					c.card("run/"+label, it.claim.Log, x+40, pl.y+60+TH, LW-80, SH, "", "")
				}
			case "t":
				color := ""
				switch lane {
				case "Landing":
					color = "4"
				case "Stuck":
					color = "1"
				}
				c.card("lane/"+lane, it.task.Goal, x+20, pl.y, LW-40, TH, color, "#^"+it.task.Block)
			default:
				c.card("lane/"+lane, it.name, x+20, pl.y, LW-40, SH, "4", "")
			}
		}
		x += LW + 80
	}
	return c
}

var zero time.Time
