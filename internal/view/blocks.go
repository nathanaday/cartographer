// Package view turns a computed project into what the user sees: the rendered blocks of
// each document, the tool fields, and the canvases.
package view

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
)

// Kinds are the rendered blocks, by the name after "cartographer-".
var Kinds = map[string]func(*core.Project, string) []string{
	"obligations": Obligations, "children": Children, "work": Work, "adopters": Adopters, "attached": Attached,
	"progress": Progress, "goals": Goals, "health": Health, "attention": Attention, "since": Since,
}

// Block renders one block of one document as Markdown lines.
func Block(p *core.Project, kind, doc string) ([]string, error) {
	f, ok := Kinds[kind]
	if !ok {
		return nil, fmt.Errorf("no block named %q", kind)
	}
	if _, ok := p.Docs[doc]; !ok {
		return nil, fmt.Errorf("no document named %q in the project", doc)
	}
	return f(p, doc), nil
}

func L(n string) string { return "[[" + n + "]]" }

func table(head []string, rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}
	out := []string{"| " + strings.Join(head, " | ") + " |", "|" + strings.Repeat("---|", len(head))}
	for _, r := range rows {
		out = append(out, "| "+strings.Join(r, " | ")+" |")
	}
	return out
}

func or(lines []string, none string) []string {
	if len(lines) == 0 {
		return []string{none}
	}
	return lines
}

func figText(f core.Figures) string {
	s := fmt.Sprintf("met %d", f.Met)
	if f.Waived > 0 {
		s += fmt.Sprintf(" · waived %d", f.Waived)
	}
	return s + fmt.Sprintf(" / %d", f.Total)
}

func statusText(o *core.Obligation) string {
	s := o.Status
	switch o.Status {
	case "met-none":
		s = "met (none)"
	case "violated":
		s = "**violated**"
	case "waived":
		s = "waived (" + o.WaivedFrom + ")"
	}
	if o.Stale != "" {
		s += " · stale " + o.Stale
	}
	return s
}

func sourceText(p *core.Project, o *core.Obligation) string {
	switch {
	case o.Source == "local":
		return "local"
	case o.Source == o.Entity:
		return "adopts"
	case o.Clause.Scope == "whole" && p.Type(o.Source) != "goal":
		return L(o.Source) + " (whole)"
	}
	return L(o.Source)
}

func resultCell(p *core.Project, o *core.Obligation) string {
	if o.Result != nil {
		return L(o.Result.Check)
	}
	if rs := p.ResultsOf(o.Entity, o.Clause.Owner, o.Clause.ID); len(rs) > 0 {
		return "inconclusive in " + L(rs[len(rs)-1].Check)
	}
	if p.CheckedEver(o.Entity) {
		return "none"
	}
	return "not yet checked"
}

func openFindings(p *core.Project, e string) []string {
	var out []string
	for _, n := range p.Of("finding") {
		if f := p.Finding[n]; f.Entity == e && f.Status == "open" {
			out = append(out, n)
		}
	}
	return out
}

func joinLinks(names []string, sep string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = L(n)
	}
	return strings.Join(parts, sep)
}

// Obligations renders an entity's obligations, pending obligations, findings, and lint.
func Obligations(p *core.Project, e string) []string {
	obs := p.ActiveOf(e)
	sort.SliceStable(obs, func(i, j int) bool {
		if obs[i].Clause.Owner != obs[j].Clause.Owner {
			return obs[i].Clause.Owner < obs[j].Clause.Owner
		}
		return obs[i].Clause.ID < obs[j].Clause.ID
	})
	var rows [][]string
	for _, o := range obs {
		rows = append(rows, []string{o.Clause.Link(), md.Words(o.Clause.Statement, 8), sourceText(p, o), statusText(o), resultCell(p, o)})
	}
	out := or(table([]string{"Clause", "Rule", "From", "Status", "Result"}, rows), "No active obligations.")
	if len(obs) > 0 {
		s := "Compliance: " + figText(core.Count(obs)) + "."
		if !p.CheckedEver(e) {
			s += " Not yet checked: in the sweep queue."
		}
		out = append(out, "", s)
	}
	pend := p.PendingOf(e)
	sort.SliceStable(pend, func(i, j int) bool { return pend[i].Clause.ID < pend[j].Clause.ID })
	if len(pend) > 0 {
		var prow [][]string
		for _, o := range pend {
			st := "unverified"
			if o.Result != nil {
				st = o.Status
				if o.Result.Ref != p.Default {
					st += " on `" + o.Result.Ref + "`"
				}
			}
			prow = append(prow, []string{o.Clause.Link(), L(o.Goal), st})
		}
		out = append(out, "", "Pending, for goals:", "")
		out = append(out, table([]string{"Clause", "Goal", "Result"}, prow)...)
	}
	if fs := openFindings(p, e); len(fs) > 0 {
		out = append(out, "", "Open findings: "+joinLinks(fs, " · "))
	}
	var ls []string
	for _, i := range p.Lint {
		if i.Entity == e {
			ls = append(ls, i.Message)
		}
	}
	if len(ls) > 0 {
		out = append(out, "", "Lint: "+strings.Join(ls, "; ")+".")
	}
	return out
}

// Children renders the systems of a project, the services of a system, or the users of a module.
func Children(p *core.Project, e string) []string {
	switch p.Type(e) {
	case "project":
		var rows [][]string
		for _, s := range p.Of("system") {
			r := p.Rollup(s)
			rows = append(rows, []string{L(s), joinLinks(p.Members(s), " · "), figText(r), strconv.Itoa(r.Violated)})
		}
		var mobs []*core.Obligation
		for _, m := range p.Of("module") {
			mobs = append(mobs, p.ActiveOf(m)...)
		}
		mf := core.Count(mobs)
		rows = append(rows, []string{"Modules", joinLinks(p.Of("module"), " · "), figText(mf), strconv.Itoa(mf.Violated)})
		return table([]string{"System", "Services", "Rollup", "Violated"}, rows)
	case "system":
		var rows [][]string
		for _, m := range p.Members(e) {
			c := p.Compliance(m)
			rows = append(rows, []string{L(m), figText(c), strconv.Itoa(c.Violated)})
		}
		return table([]string{"Service", "Compliance", "Violated"}, rows)
	case "module":
		var users []string
		for _, s := range p.Of("service") {
			if contains(md.Names(p.FM(s, "uses")), e) {
				users = append(users, s)
			}
		}
		return or(nonEmpty(joinLinks(users, " · ")), "No service uses it.")
	}
	return nil
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func agentsText(p *core.Project, g string) string {
	var parts []string
	for _, c := range p.ClaimsOf(g) {
		parts = append(parts, fmt.Sprintf("%s in %s: [[%s]]", c.Task, c.Repo, c.Log))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " · ")
}

func taskState(p *core.Project, t *core.Task) string {
	if b := p.Blocked(t); len(b) > 0 {
		return "waits on " + joinLinks(b, " · ")
	}
	for _, c := range p.ClaimsOf(t.Goal) {
		if c.Task == t.ID {
			return fmt.Sprintf("an agent works on it in %s: [[%s]]", c.Repo, c.Log)
		}
	}
	if core.Attempts(t) >= 2 {
		return "stuck"
	}
	return "open"
}

// Work renders the open work that names an entity.
func Work(p *core.Project, e string) []string {
	var out []string
	for _, g := range p.Of("goal") {
		for _, t := range p.Tasks[g] {
			if t.Target == e && t.Box != "x" && t.Box != "-" {
				s := taskState(p, t)
				out = append(out, fmt.Sprintf("- %s of %s (%s): %s. %s.", t.ID, L(g), p.S(g, "status"), t.Text, strings.ToUpper(s[:1])+s[1:]))
			}
		}
	}
	for _, q := range p.Of("question") {
		if p.S(q, "status") == "open" && contains(md.Names(p.FM(q, "about")), e) {
			out = append(out, "- Question for you: "+L(q)+".")
		}
	}
	for _, n := range openFindings(p, e) {
		out = append(out, "- Open finding: "+L(n)+".")
	}
	return or(out, "No open work.")
}

var digits = regexp.MustCompile(`\D`)

// Adopters renders a contract's obligations by entity and clause.
func Adopters(p *core.Project, c string) []string {
	var cls []*core.Clause
	for _, cl := range p.Clauses {
		if cl.Owner == c && (cl.State == "active" || cl.State == "pending") {
			cls = append(cls, cl)
		}
	}
	num := func(id string) int { n, _ := strconv.Atoi(digits.ReplaceAllString(id, "")); return n }
	sort.SliceStable(cls, func(i, j int) bool { return num(cls[i].ID) < num(cls[j].ID) })
	set := map[string]bool{}
	for _, o := range p.Obligations {
		if o.Clause.Owner == c {
			set[o.Entity] = true
		}
	}
	ents := keys(set)
	short := map[string]string{"met": "met", "met-none": "met (none)", "violated": "**violated**", "waived": "waived", "unverified": "unverified"}
	head := []string{"Entity"}
	for _, cl := range cls {
		h := cl.ID
		if cl.State == "pending" {
			h += " (pending)"
		}
		head = append(head, h)
	}
	var rows [][]string
	for _, e := range ents {
		row := []string{L(e)}
		for _, cl := range cls {
			var o *core.Obligation
			for _, x := range p.Obligations {
				if x.Entity == e && x.Clause == cl {
					o = x
					break
				}
			}
			cell := ""
			if o != nil {
				cell = short[o.Status]
				if o.Stale != "" {
					cell += " · stale"
				}
				if o.Pending() && o.Result != nil && o.Result.Ref != p.Default {
					cell += " (branch)"
				}
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}
	out := or(table(head, rows), "No adopters.")
	var adopters []string
	all := append([]string{p.Project}, p.Of("system")...)
	all = append(all, p.Of("service")...)
	all = append(all, p.Of("module")...)
	for _, a := range all {
		if contains(p.Adopts(a), c) {
			adopters = append(adopters, a)
		}
	}
	out = append(out, "", "Adopted by "+joinLinks(adopters, " · ")+". **Adopt for…**")
	var fs []string
	for _, n := range p.Of("finding") {
		if f := p.Finding[n]; f.Contract == L(c) && f.Status == "open" {
			fs = append(fs, n)
		}
	}
	if len(fs) > 0 {
		out = append(out, "", "Open findings: "+joinLinks(fs, " · "))
	}
	if ls := lintMentioning(p, L(c)); len(ls) > 0 {
		out = append(out, "", "Lint: "+strings.Join(ls, "; ")+".")
	}
	return out
}

func lintMentioning(p *core.Project, link string) []string {
	var out []string
	for _, i := range p.Lint {
		if strings.Contains(i.Message, link) {
			out = append(out, i.Message)
		}
	}
	return out
}

// Attached renders the services attached to an interface.
func Attached(p *core.Project, i string) []string {
	var att []string
	for _, s := range p.Of("service") {
		if contains(md.Names(p.FM(s, "interfaces")), i) {
			att = append(att, s)
		}
	}
	a := joinLinks(att, " · ")
	if a == "" {
		a = "none"
	}
	out := []string{"Attached: " + a}
	if ext := p.S(i, "external"); ext != "" {
		out = append(out, "External: "+ext)
	}
	if ls := lintMentioning(p, L(i)); len(ls) > 0 {
		out = append(out, "", "Lint: "+strings.Join(ls, "; ")+".")
	}
	return out
}

func branchText(p *core.Project, g string) string {
	bs := p.GoalBranches(g)
	if len(bs) == 0 {
		return ""
	}
	n := 0
	for _, b := range bs {
		if b.State == "landed" {
			n++
		}
	}
	return fmt.Sprintf("landed %d/%d", n, len(bs))
}

// ProgressLine is a goal's one-line progress.
func ProgressLine(p *core.Project, g string, waiting bool) string {
	d, k := p.GoalTasks(g, time.Time{})
	if p.Standing(g) {
		return fmt.Sprintf("tasks %d/%d · open %d", d, k, k-d)
	}
	m, n := p.GoalProgress(g, time.Time{})
	parts := []string{fmt.Sprintf("obligations %d/%d", m, n), fmt.Sprintf("tasks %d/%d", d, k)}
	if bt := branchText(p, g); bt != "" && p.S(g, "status") != "draft" {
		parts = append(parts, bt)
	}
	if waiting {
		if qs := p.QuestionsBlocking(g); len(qs) > 0 {
			s := fmt.Sprintf("waiting on you: %d question", len(qs))
			if len(qs) > 1 {
				s += "s"
			}
			parts = append(parts, s)
		}
		if p.S(g, "status") == "draft" {
			s := "waiting on you: approval"
			if d := p.S(g, "defer"); d != "" {
				s += ", deferred until " + d
			}
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// Progress renders a goal's progress block.
func Progress(p *core.Project, g string) []string {
	out := []string{"**" + ProgressLine(p, g, true) + "**", ""}
	if !p.Standing(g) {
		var rows [][]string
		for _, o := range p.GoalObligations(g) {
			st := "unverified"
			switch {
			case o.Status == "waived":
				st = "waived (does not count)"
			case o.Result != nil:
				st = o.Status
			}
			rs := p.ResultsOf(o.Entity, o.Clause.Owner, o.Clause.ID)
			var last *core.Result
			if len(rs) > 0 {
				last = rs[len(rs)-1]
			}
			if last != nil && last.Result == "inconclusive" && o.Result == nil {
				st = "inconclusive"
			}
			where, check := "", "none"
			if last != nil {
				check = L(last.Check)
				if last.Ref != p.Default {
					where = " on `" + last.Ref + "`"
				}
			}
			rows = append(rows, []string{o.Clause.Link(), L(o.Entity), st + where, check})
		}
		out = append(out, table([]string{"Clause", "On", "Result", "Last check"}, rows)...)
		if bs := p.GoalBranches(g); len(bs) > 0 {
			var parts []string
			for _, b := range bs {
				parts = append(parts, b.Repo+" "+b.State)
			}
			out = append(out, "", "Branch `"+p.S(g, "branch")+"`: "+strings.Join(parts, " · "))
		}
	}
	if qs := p.QuestionsBlocking(g); len(qs) > 0 {
		out = append(out, "", "Waiting on you: "+joinLinks(qs, " · "))
	}
	if len(p.ClaimsOf(g)) > 0 {
		out = append(out, "", "Agents: "+agentsText(p, g))
	}
	if ready := p.Ready(g); len(ready) > 0 {
		var ids []string
		for _, t := range ready {
			ids = append(ids, t.ID)
		}
		out = append(out, "", "Ready for the run queue: "+strings.Join(ids, ", "))
	}
	return out
}

func maxAgents(p *core.Project) int {
	switch v := p.FM(p.Project, "max_agents").(type) {
	case int:
		return v
	}
	return 3
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Goals renders the dashboard's table of goals and the run queue.
func Goals(p *core.Project, _ string) []string {
	var rows [][]string
	for _, g := range p.Of("goal") {
		if s := p.S(g, "status"); s == "done" || s == "dropped" {
			continue
		}
		var wait []string
		for _, q := range p.QuestionsBlocking(g) {
			wait = append(wait, L(q))
		}
		if p.S(g, "status") == "draft" {
			wait = append(wait, "approval")
		}
		status := p.S(g, "status")
		if p.Standing(g) {
			status = "standing"
		}
		if d := md.Time(p.FM(g, "defer")); !d.IsZero() && d.After(p.Env.Now()) {
			status += ", deferred until " + p.S(g, "defer")
		}
		var ids []string
		for _, t := range p.Ready(g) {
			ids = append(ids, t.ID)
		}
		ready := "—"
		if len(ids) > 0 {
			ready = strings.Join(ids, ", ")
			if len(p.ClaimsOf("")) >= maxAgents(p) {
				ready += " (no free slot)"
			}
		}
		w := strings.Join(wait, " · ")
		if w == "" {
			w = "—"
		}
		rows = append(rows, []string{L(g), status, ProgressLine(p, g, false), ready, w, agentsText(p, g), stamp(p.FM(g, "updated"))})
	}
	ready, sweep := 0, p.SweepQueue()
	for _, g := range p.Of("goal") {
		ready += len(p.Ready(g))
	}
	out := table([]string{"Goal", "Status", "Progress", "Ready", "Waiting on you", "Agents", "Last activity"}, rows)
	return append(out, "", fmt.Sprintf("Run queue: %s · %d agents at work of %d · %d obligations to sweep, next sweep %s. **Run now**",
		plural(ready, "ready task"), len(p.ClaimsOf("")), maxAgents(p), sweep, p.Env.NextSweep().Format("2006-01-02 15:04")))
}

func stamp(v any) string {
	t := md.Time(v)
	if t.IsZero() {
		return md.Str(v)
	}
	return t.Format("2006-01-02 15:04")
}

// Option is one option of a finding or question.
type Option struct {
	N           string `json:"n"`
	Title       string `json:"title"`
	Recommended bool   `json:"recommended,omitempty"`
}

var optRe = regexp.MustCompile(`(?m)^(\d+)\. \*\*(.+?)\*\*(.*)$`)
var recRe = regexp.MustCompile(`\s*\(recommended\)`)

// Options reads the options of a finding or question.
func Options(p *core.Project, doc string) []Option {
	var out []Option
	for _, m := range optRe.FindAllStringSubmatch(md.Section(p.Docs[doc].Body, "Options"), -1) {
		out = append(out, Option{m[1], strings.TrimRight(recRe.ReplaceAllString(m[2], ""), "."), strings.Contains(m[2]+m[3], "recommended")})
	}
	return out
}

func optionLine(opts []Option) string {
	var parts []string
	rec := false
	for _, o := range opts {
		s := o.N + ". " + o.Title
		if o.Recommended {
			s += " ★"
			rec = true
		}
		parts = append(parts, s)
	}
	s := "  " + strings.Join(parts, " · ")
	if rec {
		s += " · **Accept recommended**"
	}
	return s + " · **Not now until…**"
}

// Item is one decision in Attention. Kind says which acts it offers: answer, decide,
// and lint choose an option; approve-goal and approve-change approve or reject; land
// lands or asks for changes; unstick retries, takes, or drops; renew and do only inform.
// Key names what the acts apply to: a document, "Goal#T2" for a task, or "L1|entity"
// for a lint result. DeferKey is what Not now defers, or "" when the line has none.
type Item struct {
	Kind     string   `json:"kind"`
	Key      string   `json:"key"`
	Doc      string   `json:"doc,omitempty"`
	Text     string   `json:"text"`
	Options  []Option `json:"options,omitempty"`
	Actions  []string `json:"actions,omitempty"`
	Entities []string `json:"entities,omitempty"`
	DeferKey string   `json:"defer_key,omitempty"`
	Defer    string   `json:"defer,omitempty"`
	score    int
	at       time.Time
	notNow   bool
}

func (it Item) lines() []string {
	out := []string{"- " + it.Text}
	switch {
	case len(it.Options) > 0:
		out = append(out, optionLine(it.Options))
	case len(it.Actions) > 0:
		s := "  " + strings.Join(it.Actions, " · ")
		if it.notNow {
			s += " · **Not now until…**"
		}
		out = append(out, s)
	}
	return out
}

// AttentionItems returns the decisions that wait for the user, in order, and those the
// user deferred.
func AttentionItems(p *core.Project) (items, deferred []Item) {
	now := p.Env.Now()
	put := func(it Item) {
		if d := md.Time(it.Defer); !d.IsZero() && d.After(now) {
			deferred = append(deferred, it)
		} else {
			items = append(items, it)
		}
	}
	for _, q := range p.Of("question") {
		if p.S(q, "status") != "open" {
			continue
		}
		var blocks []string
		for _, g := range p.Of("goal") {
			for _, t := range p.Tasks[g] {
				if contains(md.Names(t.F["blocked"]), q) {
					blocks = append(blocks, t.ID+" of "+L(g))
				}
			}
		}
		tail := " Blocks nothing."
		if len(blocks) > 0 {
			tail = " Unblocks " + strings.Join(blocks, ", ") + "."
		}
		put(Item{Kind: "answer", Key: q, Doc: q, Text: "**Answer** " + L(q) + "." + tail, Options: Options(p, q), DeferKey: q, Defer: p.S(q, "defer"),
			score: len(blocks), at: md.Time(p.FM(q, "created"))})
	}
	for _, c := range p.Of("change") {
		if p.S(c, "status") != "proposed" {
			continue
		}
		summary := strings.TrimSpace(strings.ReplaceAll(md.Section(p.Docs[c].Body, "Summary"), "\n- ", "; "))
		summary = strings.TrimPrefix(summary, "- ")
		put(Item{Kind: "approve-change", Key: md.Str(p.FM(c, "id")), Doc: c, Text: fmt.Sprintf("**Approve** the change %s: %s", L(c), strings.TrimSuffix(summary, ".")+"."),
			Actions: []string{"Approve", "Reject"}, score: 0, at: md.Time(p.FM(c, "created"))})
	}
	landing := p.S(p.Project, "landing")
	for _, g := range p.Of("goal") {
		if p.S(g, "status") == "draft" {
			n := len(p.Tasks[g])
			put(Item{Kind: "approve-goal", Key: g, Doc: g, Text: fmt.Sprintf("**Approve** the draft goal %s. Starts %d tasks.", L(g), n),
				Actions: []string{"Approve", "Drop"}, notNow: true, DeferKey: g, Defer: p.S(g, "defer"), score: n, at: md.Time(p.FM(g, "created"))})
		}
		tasks, whole := p.ReadyToLand(g)
		if p.Standing(g) && landing == "user" {
			for _, t := range tasks {
				key := g + "#" + t.ID
				put(Item{Kind: "land", Key: key, Doc: g, Text: fmt.Sprintf("**Land** %s of %s: %s. The reviewer passed; %s, branch `%s`.", t.ID, L(g), t.Text, p.TaskRepo(t), p.TaskBranch(t)),
					Actions: []string{"Land", "Request changes"}, notNow: true, DeferKey: key, Defer: t.F["defer"], score: 1, at: md.Time(t.F["done"])})
			}
		} else if whole && landing == "user" {
			m, n := p.GoalProgress(g, time.Time{})
			var open []string
			for _, b := range p.GoalBranches(g) {
				if b.State != "landed" {
					open = append(open, b.Repo)
				}
			}
			put(Item{Kind: "land", Key: g, Doc: g, Text: fmt.Sprintf("**Land** %s: reviewer passed; obligations %d/%d met on the branch; %s.", L(g), m, n, strings.Join(open, ", ")),
				Actions: []string{"Land", "Request changes"}, Entities: goalEntities(p, g), notNow: true, DeferKey: g, Defer: p.S(g, "defer"), score: len(p.Tasks[g]), at: now})
		}
		for _, t := range p.Tasks[g] {
			if t.Box == " " && core.Attempts(t) >= 2 && t.F["user"] != "true" {
				key := g + "#" + t.ID
				put(Item{Kind: "unstick", Key: key, Doc: g, Text: fmt.Sprintf("**Unstick** %s of %s: failed twice.", t.ID, L(g)),
					Actions: []string{"Retry with a hint", "I'll do it", "Drop"}, notNow: true, DeferKey: key, Defer: t.F["defer"], score: 1, at: now})
			}
		}
	}
	for _, n := range p.Of("finding") {
		f := p.Finding[n]
		if f.Status != "open" {
			continue
		}
		idle := int(now.Sub(md.Time(p.FM(n, "updated"))).Hours() / 24)
		stuck := f.Chosen != "" && idle >= p.NudgeAfter
		if f.Chosen == "" || stuck {
			why := ""
			if stuck {
				why = fmt.Sprintf(" Its chosen option has not moved in %d days.", idle)
			}
			put(Item{Kind: "decide", Key: n, Doc: n, Text: "**Decide** on " + L(n) + "." + why, Options: Options(p, n), DeferKey: n, Defer: p.S(n, "defer"),
				score: 1, at: md.Time(p.FM(n, "found"))})
		}
	}
	held := map[string]bool{}
	for _, q := range p.Of("question") {
		if p.S(q, "status") == "open" && p.S(q, "holds") != "" {
			held[p.S(q, "holds")] = true
		}
	}
	for _, i := range p.Lint {
		if !held["lint "+i.Rule+" [["+i.Entity+"]]"] {
			put(Item{Kind: "lint", Key: i.Rule + "|" + i.Entity, Doc: i.Entity, Text: "**Decide** on lint " + i.Rule + ": " + i.Message + ".",
				Options: []Option{{"1", "Fix it on the map", true}, {"2", "Waive", false}}, at: now})
		}
	}
	for _, w := range p.Waivers {
		if u := md.Time(w.F["until"]); !u.IsZero() {
			if d := u.Sub(now).Hours() / 24; d >= 0 && d < 8 {
				put(Item{Kind: "renew", Key: w.Entity, Doc: w.Entity, Text: fmt.Sprintf("**Renew or end** the waiver in %s that ends %s.", L(w.Entity), w.F["until"]), at: u})
			}
		}
	}
	for _, g := range p.Of("goal") {
		for _, t := range p.Tasks[g] {
			if due := md.Time(t.F["due"]); t.Box == " " && t.F["user"] == "true" && !due.IsZero() && due.Sub(now).Hours()/24 < 8 {
				put(Item{Kind: "do", Key: g + "#" + t.ID, Doc: g, Text: fmt.Sprintf("**Do** %s of %s: %s, due %s.", t.ID, L(g), t.Text, t.F["due"]), at: due})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].at.Before(items[j].at)
	})
	return items, deferred
}

func goalEntities(p *core.Project, g string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range p.Tasks[g] {
		if !seen[t.Target] {
			seen[t.Target] = true
			out = append(out, t.Target)
		}
	}
	return out
}

// Attention renders the decisions that wait for the user.
func Attention(p *core.Project, _ string) []string {
	items, deferred := AttentionItems(p)
	var out []string
	for _, it := range items {
		out = append(out, it.lines()...)
	}
	out = or(out, "Nothing waits for you.")
	if len(deferred) > 0 {
		var ds []string
		for _, d := range deferred {
			ds = append(ds, "until "+d.Defer)
		}
		out = append(out, "", fmt.Sprintf("Deferred: %d (%s).", len(deferred), strings.Join(ds, ", ")))
	}
	return out
}

// Health renders each problem once, with where it stands.
func Health(p *core.Project, _ string) []string {
	var out []string
	for _, n := range p.Of("finding") {
		f := p.Finding[n]
		if f.Status != "open" && f.Status != "waived" {
			continue
		}
		where := ""
		if len(f.Resolution) > 0 {
			l := md.Links(f.Resolution[0])[0]
			var task *core.Task
			for _, t := range p.Tasks[l.Name] {
				if t.Block == l.Anchor {
					task = t
				}
			}
			if task != nil {
				var state string
				if task.Box == "x" {
					state = "done, waiting for you to land"
					if p.Landed(p.TaskRepo(task), p.TaskBranch(task)) {
						state = "landed"
					}
				} else {
					state = "waiting in the run queue"
					for _, c := range p.ClaimsOf(l.Name) {
						if c.Task == task.ID {
							state = "an agent works on it"
						}
					}
				}
				where = fmt.Sprintf(" → %s %s, %s", l.Name, task.ID, state)
			} else {
				where = " → " + f.Resolution[0] + " (" + p.S(l.Name, "status")
				if d := p.S(l.Name, "defer"); d != "" {
					where += ", deferred until " + d
				}
				where += ")"
			}
		}
		if f.Status == "waived" {
			for _, w := range p.Waivers {
				if w.Entity == f.Entity {
					where = " · waived until " + w.F["until"] + where
					break
				}
			}
		}
		out = append(out, "- "+strings.ToUpper(f.Kind[:1])+strings.ToLower(f.Kind[1:])+": "+L(n)+where+".")
	}
	for _, o := range p.Obligations {
		if o.Pending() || o.Status != "unverified" || len(p.ResultsOf(o.Entity, o.Clause.Owner, o.Clause.ID)) == 0 {
			continue
		}
		s := "- Inconclusive: " + o.Clause.Link() + " on " + L(o.Entity)
		for _, q := range p.Of("question") {
			about := md.Names(p.FM(q, "about"))
			if p.S(q, "status") == "open" && contains(about, o.Entity) && contains(about, o.Clause.Owner) {
				s += " → " + L(q)
				break
			}
		}
		out = append(out, s+".")
	}
	for _, i := range p.Lint {
		out = append(out, "- Lint "+i.Rule+": "+i.Message+".")
	}
	var never []string
	for _, e := range append(append(p.Of("system"), p.Of("service")...), p.Of("module")...) {
		if !p.CheckedEver(e) && len(p.ActiveOf(e)) > 0 {
			never = append(never, e)
		}
	}
	if len(never) > 0 {
		out = append(out, "- Never checked: "+joinLinks(never, " · ")+".")
	}
	stale, oldest := 0, 0
	for _, o := range p.Obligations {
		if !o.Pending() && o.Stale != "" {
			stale++
			if d := int(p.Env.Now().Sub(o.Result.Date).Hours() / 24); d > oldest {
				oldest = d
			}
		}
	}
	s := fmt.Sprintf("- Sweep queue: %d obligations (%d stale, oldest %d days), next sweep %s.", p.SweepQueue(), stale, oldest, p.Env.NextSweep().Format("2006-01-02 15:04"))
	if oldest > p.StaleAfter {
		s += " **Not draining.**"
	}
	out = append(out, s)
	if len(out) == 1 {
		return []string{"All obligations met."}
	}
	return out
}

// Since renders what changed since the user's last Mark seen.
func Since(p *core.Project, _ string) []string {
	lv := p.Env.LastVisit()
	since := "Since " + lv.Format("2006-01-02 15:04") + "."
	if lv.IsZero() {
		since = "Since the project started."
	}
	out := []string{since + " **Mark seen**", ""}
	for _, g := range p.Of("goal") {
		if p.Standing(g) {
			continue
		}
		m0, n0 := p.GoalProgress(g, lv)
		m1, n1 := p.GoalProgress(g, time.Time{})
		d0, k0 := p.GoalTasks(g, lv)
		d1, k1 := p.GoalTasks(g, time.Time{})
		if m0 != m1 || d0 != d1 {
			out = append(out, fmt.Sprintf("- %s: obligations %d/%d → %d/%d, tasks %d/%d → %d/%d.", L(g), m0, n0, m1, n1, d0, k0, d1, k1))
		}
	}
	for _, g := range p.Of("goal") {
		for _, l := range p.Landings(g) {
			if l.At.After(lv) {
				s := "- Landed: " + l.Repo
				if l.Task != "" {
					s += " (" + l.Task + ")"
				}
				out = append(out, s+" of "+L(g)+", at "+l.SHA+".")
			}
		}
	}
	type group struct {
		order []string
		ids   map[string][]string
	}
	newly := map[string]*group{"met": {ids: map[string][]string{}}, "violated": {ids: map[string][]string{}}}
	for _, o := range p.Obligations {
		if o.Pending() {
			continue
		}
		before, _ := p.StatusAt(o, lv)
		key := o.Status
		if key == "met-none" {
			key = "met"
		}
		if gr, ok := newly[key]; ok && before != o.Status {
			if _, seen := gr.ids[o.Entity]; !seen {
				gr.order = append(gr.order, o.Entity)
			}
			gr.ids[o.Entity] = append(gr.ids[o.Entity], o.Clause.ID)
		}
	}
	grouped := func(gr *group) string {
		var parts []string
		for _, e := range gr.order {
			ids := gr.ids[e]
			first := len(ids) > 3
			for _, o := range p.ActiveOf(e) {
				if st, _ := p.StatusAt(o, lv); st != "unverified" {
					first = false
				}
			}
			if first {
				parts = append(parts, fmt.Sprintf("%s (%d, its first check)", L(e), len(ids)))
			} else {
				parts = append(parts, fmt.Sprintf("%s (%d: %s)", L(e), len(ids), strings.Join(ids, ", ")))
			}
		}
		if len(parts) == 0 {
			return "none"
		}
		return strings.Join(parts, ", ")
	}
	out = append(out, "- Newly met on the default branch: "+grouped(newly["met"])+". Newly violated: "+grouped(newly["violated"])+".")
	var opened, resolved []string
	for _, n := range p.Of("finding") {
		if md.Time(p.FM(n, "found")).After(lv) {
			opened = append(opened, n)
		}
		if f := p.Finding[n]; f.Status == "resolved" && md.Time(f.Resolved).After(lv) {
			resolved = append(resolved, n)
		}
	}
	out = append(out, "- Findings opened: "+orNone(joinLinks(opened, " · "))+". Resolved: "+orNone(joinLinks(resolved, " · "))+".")
	var answered []string
	for _, q := range p.Of("question") {
		if a := md.Time(p.FM(q, "answered")); !a.IsZero() && a.After(lv) {
			answered = append(answered, q)
		}
	}
	return append(out, "- Questions answered: "+orNone(joinLinks(answered, " · "))+".")
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ToolFields returns the tool-written frontmatter fields of a document.
func ToolFields(p *core.Project, n string) map[string]any {
	checked := func(e string) string {
		var last time.Time
		for _, c := range p.Checks {
			if c.Target == e && c.Date.After(last) {
				last = c.Date
			}
		}
		if last.IsZero() {
			return ""
		}
		return last.Format("2006-01-02")
	}
	switch p.Type(n) {
	case "service", "module":
		f := p.Compliance(n)
		return map[string]any{"compliance": fmt.Sprintf("%d/%d", f.Met, f.Total), "violated": f.Violated, "checked": checked(n)}
	case "system":
		f, r := p.Compliance(n), p.Rollup(n)
		return map[string]any{"compliance": fmt.Sprintf("%d/%d", f.Met, f.Total), "rollup": fmt.Sprintf("%d/%d", r.Met, r.Total), "violated": r.Violated}
	case "project":
		r := p.Rollup(n)
		return map[string]any{"rollup": fmt.Sprintf("%d/%d", r.Met, r.Total), "violated": r.Violated}
	case "goal":
		m, k := p.GoalProgress(n, time.Time{})
		d, j := p.GoalTasks(n, time.Time{})
		aff := []string{}
		for _, e := range p.Affects(n) {
			aff = append(aff, L(e))
		}
		landed := ""
		if bs := p.GoalBranches(n); len(bs) > 0 {
			c := 0
			for _, b := range bs {
				if b.State == "landed" {
					c++
				}
			}
			landed = fmt.Sprintf("%d/%d", c, len(bs))
		}
		return map[string]any{"affects": aff, "progress": fmt.Sprintf("%d/%d", m, k), "tasks": fmt.Sprintf("%d/%d", d, j), "landed": landed}
	case "finding":
		f := p.Finding[n]
		res := f.Resolution
		if res == nil {
			res = []string{}
		}
		return map[string]any{"status": f.Status, "contract": f.Contract, "resolution": res, "resolved": f.Resolved}
	}
	return nil
}

var blockRe = regexp.MustCompile("```cartographer-(\\w+)\\n```\\n(?:\\n> \\[!example\\][^\\n]*\\n(?:>[^\\n]*\\n)*)?")

// RenderExamples rewrites each rendered block of a document's text with an example
// callout below it, showing what the plugin renders. It is for the design example.
func RenderExamples(p *core.Project, name, text string) string {
	return blockRe.ReplaceAllStringFunc(text, func(m string) string {
		kind := blockRe.FindStringSubmatch(m)[1]
		lines, err := Block(p, kind, name)
		if err != nil {
			return m
		}
		var b strings.Builder
		b.WriteString("```cartographer-" + kind + "\n```\n\n> [!example] What the plugin shows\n")
		for _, l := range lines {
			if l == "" {
				b.WriteString(">\n")
			} else {
				b.WriteString("> " + l + "\n")
			}
		}
		return b.String()
	})
}
