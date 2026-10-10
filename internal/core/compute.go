package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/md"
)

var conclusive = map[string]bool{"met": true, "met-none": true, "violated": true}

// Obligation is one clause that falls on one entity.
type Obligation struct {
	Entity     string
	Clause     *Clause
	Source     string // the adopter, "local", or a goal
	Goal       string // set when the obligation is pending
	Status     string
	Result     *Result
	Stale      string
	WaivedFrom string
}

// Pending reports whether the obligation counts only toward a goal.
func (o *Obligation) Pending() bool { return o.Goal != "" }

// FindingState is what the core computes about a finding document.
type FindingState struct {
	Status, Contract, Resolved, Chosen, Entity, Kind string
	Resolution                                       []string
}

// ---------- membership ----------

// Members returns the services of a system.
func (p *Project) Members(system string) []string {
	var out []string
	for _, s := range p.Of("service") {
		if contains(md.Names(p.FM(s, "systems")), system) {
			out = append(out, s)
		}
	}
	return out
}

// Inside returns an entity and everything inside its group.
func (p *Project) Inside(entity string) []string {
	switch p.Type(entity) {
	case "project":
		out := []string{entity}
		out = append(out, p.Of("system")...)
		out = append(out, p.Of("service")...)
		return append(out, p.Of("module")...)
	case "system":
		return append([]string{entity}, p.Members(entity)...)
	}
	return []string{entity}
}

// Adopts returns the contracts an entity adopts.
func (p *Project) Adopts(entity string) []string { return md.Names(p.FM(entity, "adopts")) }

func (p *Project) effectiveAdopts(service string) map[string]bool {
	out := map[string]bool{}
	for _, c := range p.Adopts(service) {
		out[c] = true
	}
	for _, c := range p.Adopts(p.Project) {
		out[c] = true
	}
	for _, s := range md.Names(p.FM(service, "systems")) {
		for _, c := range p.Adopts(s) {
			out[c] = true
		}
	}
	return out
}

// CheckedEver reports whether any check targets the entity.
func (p *Project) CheckedEver(entity string) bool {
	for _, c := range p.Checks {
		if c.Target == entity {
			return true
		}
	}
	return false
}

// ---------- goals: status and branches ----------

// Live reports whether a goal is active (or shown as parked).
func (p *Project) Live(g string) bool {
	s := p.S(g, "status")
	return s == "active" || s == "parked"
}

// Standing reports whether a goal is Upkeep.
func (p *Project) Standing(g string) bool {
	b, _ := p.FM(g, "standing").(bool)
	return b
}

// TaskBranch returns the branch a task works on.
func (p *Project) TaskBranch(t *Task) string {
	if p.Standing(t.Goal) {
		return "upkeep/" + strings.ToLower(t.ID)
	}
	return p.S(t.Goal, "branch")
}

// TaskRepo returns the repository a task runs in.
func (p *Project) TaskRepo(t *Task) string {
	loc := p.S(t.Target, "repository")
	if loc == "" {
		loc = p.S(t.Target, "definition")
	}
	if strings.Count(loc, "/") >= 2 {
		r, _ := RepoPath(loc)
		return r
	}
	return t.F["repo"]
}

// Landed reports whether a goal's Log records that repo landed from ref.
func (p *Project) Landed(repo, ref string) bool {
	for _, g := range p.Of("goal") {
		for _, l := range p.Landings(g) {
			if l.Repo == repo && (p.S(g, "branch") == ref || (l.Task != "" && "upkeep/"+strings.ToLower(l.Task) == ref)) {
				return true
			}
		}
	}
	return false
}

// Branch is a goal's branch in one repository.
type Branch struct {
	Repo, State string
	At          time.Time
}

// GoalBranches returns a goal's branches, by repository.
func (p *Project) GoalBranches(g string) []Branch {
	if p.Standing(g) {
		return nil
	}
	repos := map[string]bool{}
	for _, t := range p.Tasks[g] {
		if r := p.TaskRepo(t); t.Box != "-" && r != "" {
			repos[r] = true
		}
	}
	landed := map[string]time.Time{}
	for _, l := range p.Landings(g) {
		landed[l.Repo] = l.At
	}
	var out []Branch
	for _, r := range sortedKeys(repos) {
		if at, ok := landed[r]; ok {
			out = append(out, Branch{r, "landed", at})
		} else {
			out = append(out, Branch{Repo: r, State: "open"})
		}
	}
	return out
}

func (p *Project) counts(r *Result) bool {
	return p.Env.OnDefault(r.Repo, r.Commit, r.Ref, p.Landed)
}

// ---------- obligations ----------

func (p *Project) fallsOn(c *Clause, adopter string) []string {
	switch p.Type(adopter) {
	case "project":
		if c.Scope == "each" {
			return append(append([]string{}, p.Of("service")...), p.Of("module")...)
		}
	case "system":
		if c.Scope == "each" {
			return p.Members(adopter)
		}
	}
	return []string{adopter}
}

func (p *Project) goalAddedAdoption(contract, adopter string) string {
	for _, g := range p.Of("goal") {
		if !p.Live(g) {
			continue
		}
		for _, a := range p.Changes[g] {
			if a.Verb == "adopt" && len(a.Links) == 2 && a.Links[0].Name == contract && a.Links[1].Name == adopter {
				return g
			}
		}
	}
	return ""
}

func (p *Project) computeObligations() {
	p.Drafts = map[string][]*Obligation{}
	seen := map[string]bool{}
	add := func(o *Obligation) {
		k := fmt.Sprintf("%s|%s|%s|%d", o.Entity, o.Clause.Owner, o.Clause.ID, o.Clause.Rev)
		if !seen[k] {
			seen[k] = true
			p.Obligations = append(p.Obligations, o)
		}
	}
	adopters := append([]string{p.Project}, p.Of("system")...)
	adopters = append(adopters, p.Of("service")...)
	adopters = append(adopters, p.Of("module")...)
	for _, a := range adopters {
		for _, c := range p.Adopts(a) {
			via := p.goalAddedAdoption(c, a)
			for _, cl := range p.Clauses {
				if cl.Owner != c || (cl.State != "active" && cl.State != "pending") {
					continue
				}
				if cl.State == "pending" && !p.Live(cl.Goal) {
					continue
				}
				for _, e := range p.fallsOn(cl, a) {
					g := via
					if g == "" && cl.State == "pending" {
						g = cl.Goal
					}
					add(&Obligation{Entity: e, Clause: cl, Source: a, Goal: g, Status: "unverified"})
				}
			}
		}
		for _, cl := range p.Clauses {
			if cl.Owner == a && cl.State == "active" {
				add(&Obligation{Entity: a, Clause: cl, Source: "local", Status: "unverified"})
			}
		}
	}
	for _, cl := range p.Clauses {
		if cl.State != "requirement" {
			continue
		}
		on := md.First(cl.F["on"])
		if on == "" {
			continue
		}
		o := &Obligation{Entity: on, Clause: cl, Source: cl.Owner, Goal: cl.Owner, Status: "unverified"}
		if p.Live(cl.Owner) {
			add(o)
		} else if p.S(cl.Owner, "status") == "draft" {
			p.Drafts[cl.Owner] = append(p.Drafts[cl.Owner], o)
		}
	}
}

// ActiveOf returns an entity's active obligations.
func (p *Project) ActiveOf(entity string) []*Obligation {
	var out []*Obligation
	for _, o := range p.Obligations {
		if o.Entity == entity && !o.Pending() {
			out = append(out, o)
		}
	}
	return out
}

// PendingOf returns an entity's pending obligations.
func (p *Project) PendingOf(entity string) []*Obligation {
	var out []*Obligation
	for _, o := range p.Obligations {
		if o.Entity == entity && o.Pending() {
			out = append(out, o)
		}
	}
	return out
}

// ---------- status and staleness ----------

func (p *Project) implicitDeps(o *Obligation) []string {
	var out []string
	if d := p.Docs[o.Clause.Owner]; d != nil && d.Type == "contract" {
		if def := md.Str(d.FM["definition"]); def != "" {
			r, path := RepoPath(def)
			out = append(out, r+":"+path)
		}
	}
	if p.Type(o.Entity) == "system" {
		for _, s := range p.Members(o.Entity) {
			r, _ := RepoPath(p.S(s, "repository"))
			out = append(out, r+":"+strings.TrimLeft(p.S(s, "path"), "./"))
		}
	}
	return out
}

// staleCommits returns how many commits make a result stale, and whether an external
// dependency has aged past stale_after.
func (p *Project) staleCommits(r *Result, deps []string, depth int) (int, bool) {
	shas, external := map[string]bool{}, false
	for _, dep := range deps {
		switch {
		case strings.HasPrefix(dep, "external:"):
			external = external || days(p.Env.Now().Sub(r.Date)) > p.StaleAfter
		case strings.HasPrefix(dep, "[["):
			if depth >= 3 {
				continue
			}
			ls := md.Links(dep)
			if len(ls) == 0 {
				continue
			}
			for _, k := range sortedResultKeys(p.Results) {
				for _, x := range p.Results[k] {
					if x.Check == ls[0].Name+"#"+ls[0].Anchor || x.Check == ls[0].Name {
						if n, _ := p.staleCommits(x, x.Deps, depth+1); n > 0 {
							shas["cited:"+x.Check] = true
						}
					}
				}
			}
		case strings.Contains(dep, ":"):
			repo, path, _ := strings.Cut(dep, ":")
			for _, s := range p.Env.ChangedSince(repo, path, r, p.Landed) {
				shas[s] = true
			}
		}
	}
	return len(shas), external
}

func days(d time.Duration) int {
	if d < 0 {
		return -int((-d).Hours()/24) - 1
	}
	return int(d.Hours() / 24)
}

func (p *Project) latest(o *Obligation, at time.Time, anyRef bool) *Result {
	var last *Result
	for _, r := range p.ResultsOf(o.Entity, o.Clause.Owner, o.Clause.ID) {
		if r.Rev != o.Clause.Rev || !conclusive[r.Result] || (!anyRef && !p.counts(r)) || (!at.IsZero() && r.Date.After(at)) {
			continue
		}
		last = r
	}
	return last
}

// StatusAt returns an obligation's status and latest counted result at a time (zero: now).
func (p *Project) StatusAt(o *Obligation, at time.Time) (string, *Result) {
	r := p.latest(o, at, o.Pending())
	if r == nil {
		return "unverified", nil
	}
	return r.Result, r
}

func (p *Project) computeStatus() {
	all := append([]*Obligation{}, p.Obligations...)
	for _, g := range p.Of("goal") {
		all = append(all, p.Drafts[g]...)
	}
	for _, o := range all {
		o.Status, o.Result = p.StatusAt(o, time.Time{})
		if o.Result != nil && !o.Pending() {
			n, external := p.staleCommits(o.Result, append(append([]string{}, o.Result.Deps...), p.implicitDeps(o)...), 0)
			age := days(p.Env.Now().Sub(o.Result.Date))
			timed := o.Clause.Scope == "whole" || o.Clause.Verify == "metric" || o.Clause.Verify == "review"
			if n > 0 || external || (timed && age > p.StaleAfter) {
				if n > 0 {
					o.Stale = fmt.Sprintf("%d commits, %d days", n, age)
				} else {
					o.Stale = fmt.Sprintf("%d days", age)
				}
			}
		}
		if o.Result != nil && o.Pending() && o.Result.Ref != p.Default {
			for _, d := range o.Result.Deps {
				if strings.HasPrefix(d, "[[") || strings.HasPrefix(d, "external:") || !strings.Contains(d, ":") {
					continue
				}
				_, path, _ := strings.Cut(d, ":")
				if len(p.Env.BranchChangedSince(o.Result.Repo, o.Result.Ref, path, o.Result)) > 0 {
					o.Stale = "branch moved"
				}
			}
		}
		if !o.Pending() && o.Status == "violated" && p.WaiverFor(o.Entity, o.Clause.Key(), o.Clause.Rev, o.Result.Fails) != nil {
			o.WaivedFrom, o.Status = o.Status, "waived"
		}
	}
}

// Figures counts the statuses of a set of obligations.
type Figures struct{ Met, Waived, Violated, Unverified, Stale, Total int }

// Count returns the figures of a set of obligations.
func Count(obs []*Obligation) Figures {
	var f Figures
	for _, o := range obs {
		switch o.Status {
		case "met", "met-none":
			f.Met++
		case "waived":
			f.Waived++
		case "violated":
			f.Violated++
		case "unverified":
			f.Unverified++
		}
		if o.Stale != "" {
			f.Stale++
		}
		f.Total++
	}
	return f
}

// Compliance returns an entity's own figures.
func (p *Project) Compliance(e string) Figures { return Count(p.ActiveOf(e)) }

// Rollup returns the figures of an entity and everything inside it.
func (p *Project) Rollup(e string) Figures {
	var obs []*Obligation
	for _, x := range p.Inside(e) {
		obs = append(obs, p.ActiveOf(x)...)
	}
	return Count(obs)
}

// ---------- findings ----------

func (p *Project) computeFindings() {
	p.Finding = map[string]*FindingState{}
	for _, n := range p.Of("finding") {
		kind := p.S(n, "kind")
		ent := md.First(p.FM(n, "entity"))
		st := &FindingState{Status: "open", Kind: kind, Entity: ent}
		opts := md.Fields(md.Section(p.Docs[n].Body, "Options"))
		if kind == "violation" {
			ls := md.Links(p.FM(n, "clause"))
			if len(ls) > 0 {
				doc, cid := ls[0].Name, ls[0].Anchor
				if p.Type(doc) == "contract" {
					st.Contract = "[[" + doc + "]]"
				}
				var o *Obligation
				for _, x := range p.Obligations {
					if x.Entity == ent && x.Clause.Owner == doc && x.Clause.ID == cid && !x.Pending() {
						o = x
						break
					}
				}
				switch {
				case o == nil:
					st.Status, st.Resolved = "resolved", "obligation ended"
				case o.Status == "waived":
					st.Status = "waived"
				case o.Status == "met" || o.Status == "met-none":
					st.Status, st.Resolved = "resolved", o.Result.Date.Format("2006-01-02")
				}
			}
		} else {
			key := p.S(n, "key")
			target := ent
			if target == "" {
				target = p.Project
			}
			about := md.Names(p.FM(n, "about"))
			tail := key
			if i := strings.LastIndex(key, ":"); i >= 0 {
				tail = key[i+1:]
			}
			var rows []Dependency
			for _, d := range p.Dependencies[target] {
				if strings.Contains(strings.ReplaceAll(d.Key, "`", ""), tail) {
					rows = append(rows, d)
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if !rows[i].Date.Equal(rows[j].Date) {
					return rows[i].Date.Before(rows[j].Date)
				}
				return rows[i].Check < rows[j].Check
			})
			switch {
			case kind == "divergence" && len(about) > 0 && contains(md.Names(p.FM(ent, "interfaces")), about[0]):
				st.Status, st.Resolved = "resolved", opts["on"]
			case len(rows) > 0 && (rows[len(rows)-1].Result == "convergence" || rows[len(rows)-1].Result == "mapped"):
				st.Status, st.Resolved = "resolved", rows[len(rows)-1].Date.Format("2006-01-02")
			}
			if p.WaiverFor(target, "dependency `"+key+"`", 0, nil) != nil {
				st.Status = "waived"
			}
		}
		st.Chosen = opts["chosen"]
		for _, g := range p.Of("goal") {
			for _, t := range p.Tasks[g] {
				if contains(md.Names(t.F["finding"]), n) {
					st.Resolution = append(st.Resolution, "[["+t.Goal+"#^"+t.Block+"]]")
				}
			}
		}
		for _, g := range p.Of("goal") {
			if contains(md.Names(p.FM(g, "resolves")), n) {
				st.Resolution = append(st.Resolution, "[["+g+"]]")
			}
		}
		p.Finding[n] = st
	}
}

// Findings returns the findings with a status ("" for all), in path order.
func (p *Project) Findings(status string) []string {
	var out []string
	for _, n := range p.Of("finding") {
		if status == "" || p.Finding[n].Status == status {
			out = append(out, n)
		}
	}
	return out
}

// ---------- goals: progress and the queue ----------

func (p *Project) goalClauseKeys(g string) map[string]bool {
	keys := map[string]bool{}
	for _, a := range p.Changes[g] {
		if (a.Verb == "add" || a.Verb == "revise" || a.Verb == "retire") && len(a.Links) > 0 {
			keys[a.Links[0].Name+"#"+a.Links[0].Anchor] = true
		}
	}
	for _, f := range md.Names(p.FM(g, "resolves")) {
		if p.S(f, "kind") == "violation" {
			for _, l := range md.Links(p.FM(f, "clause")) {
				keys[l.Name+"#"+l.Anchor] = true
			}
		}
	}
	return keys
}

// GoalObligations returns a goal's obligations: pending, its requirements, and the
// clauses of the findings it resolves.
func (p *Project) GoalObligations(g string) []*Obligation {
	keys := p.goalClauseKeys(g)
	var obs []*Obligation
	for _, o := range p.Obligations {
		if o.Goal == g || keys[o.Clause.Key()] {
			obs = append(obs, o)
		}
	}
	obs = append(obs, p.Drafts[g]...)
	sort.SliceStable(obs, func(i, j int) bool {
		a, b := obs[i], obs[j]
		ai, bi := a.Clause.Owner != g, b.Clause.Owner != g
		if ai != bi {
			return !ai
		}
		if a.Clause.ID != b.Clause.ID {
			return a.Clause.ID < b.Clause.ID
		}
		return a.Entity < b.Entity
	})
	return obs
}

// GoalMet reports whether a goal obligation is met (at a time; zero: now).
func (p *Project) GoalMet(o *Obligation, at time.Time) bool {
	st := o.Status
	if o.Pending() || !at.IsZero() {
		st, _ = p.StatusAt(o, at)
	}
	return st == "met" || st == "met-none"
}

// GoalProgress returns met and total obligations of a goal.
func (p *Project) GoalProgress(g string, at time.Time) (int, int) {
	obs := p.GoalObligations(g)
	m := 0
	for _, o := range obs {
		if p.GoalMet(o, at) {
			m++
		}
	}
	return m, len(obs)
}

// GoalTasks returns closed and total tasks of a goal.
func (p *Project) GoalTasks(g string, at time.Time) (int, int) {
	d := 0
	for _, t := range p.Tasks[g] {
		if t.Box != "x" && t.Box != "-" {
			continue
		}
		done := md.Time(t.F["done"])
		if done.IsZero() {
			done = p.Env.Now()
		}
		if at.IsZero() || !done.After(at) {
			d++
		}
	}
	return d, len(p.Tasks[g])
}

// Blocked returns the open questions and findings a task waits for.
func (p *Project) Blocked(t *Task) []string {
	var out []string
	for _, x := range md.Names(t.F["blocked"]) {
		if _, ok := p.Docs[x]; !ok {
			continue
		}
		if p.S(x, "status") == "open" || (p.Finding[x] != nil && p.Finding[x].Status == "open") {
			out = append(out, x)
		}
	}
	return out
}

// ClaimsOf returns the claims of a goal ("" for all).
func (p *Project) ClaimsOf(g string) []Claim {
	var out []Claim
	for _, c := range p.Env.Claims() {
		if g == "" || c.Goal == g {
			out = append(out, c)
		}
	}
	return out
}

// Attempts returns how many times a task failed.
func Attempts(t *Task) int {
	n, _ := strconv.Atoi(t.F["attempts"])
	return n
}

// Ready returns the tasks of a goal that the runner can start.
func (p *Project) Ready(g string) []*Task {
	if !p.Live(g) {
		return nil
	}
	if d := md.Time(p.FM(g, "defer")); !d.IsZero() && d.After(p.Env.Now()) {
		return nil
	}
	claimed := map[string]bool{}
	for _, c := range p.ClaimsOf(g) {
		claimed[c.Task] = true
	}
	done := map[string]bool{}
	for _, t := range p.Tasks[g] {
		if t.Box == "x" || t.Box == "-" {
			done[t.ID] = true
		}
	}
	busy := map[string]bool{}
	for _, c := range p.ClaimsOf("") {
		busy[c.Repo+"|"+p.S(c.Goal, "branch")] = true
	}
	var out []*Task
	for _, t := range p.Tasks[g] {
		if t.Box != " " || claimed[t.ID] || len(p.Blocked(t)) > 0 || t.F["user"] == "true" || Attempts(t) >= 2 {
			continue
		}
		ok := true
		for _, a := range splitTrim(t.F["after"]) {
			if !done[a] {
				ok = false
			}
		}
		if ok && !busy[p.TaskRepo(t)+"|"+p.TaskBranch(t)] {
			out = append(out, t)
		}
	}
	return out
}

// QuestionsBlocking returns the open questions that block a goal's tasks.
func (p *Project) QuestionsBlocking(g string) []string {
	set := map[string]bool{}
	for _, t := range p.Tasks[g] {
		for _, x := range md.Names(t.F["blocked"]) {
			if p.Type(x) == "question" && p.S(x, "status") == "open" {
				set[x] = true
			}
		}
	}
	return sortedKeys(set)
}

// Affects returns the entities a goal's obligations fall on.
func (p *Project) Affects(g string) []string {
	set := map[string]bool{}
	for _, o := range p.GoalObligations(g) {
		set[o.Entity] = true
	}
	return sortedKeys(set)
}

// ReadyToLand returns, for Upkeep, the done tasks whose branch has not landed; for a
// goal, whether every task is done and a repository has not landed.
func (p *Project) ReadyToLand(g string) ([]*Task, bool) {
	if p.Standing(g) {
		var out []*Task
		for _, t := range p.Tasks[g] {
			if t.Box == "x" && t.F["commit"] != "" && !p.Landed(p.TaskRepo(t), p.TaskBranch(t)) {
				out = append(out, t)
			}
		}
		return out, false
	}
	d, k := p.GoalTasks(g, time.Time{})
	if !p.Live(g) || k == 0 || d != k {
		return nil, false
	}
	for _, b := range p.GoalBranches(g) {
		if b.State != "landed" {
			return nil, true
		}
	}
	return nil, false
}

// SweepQueue returns how many active obligations wait for a sweep.
func (p *Project) SweepQueue() int {
	n := 0
	for _, o := range p.Obligations {
		if o.Pending() {
			continue
		}
		if (o.Status == "unverified" && !p.waitsForQuestion(o)) || o.Stale != "" {
			n++
		}
	}
	return n
}

func (p *Project) waitsForQuestion(o *Obligation) bool {
	rs := p.ResultsOf(o.Entity, o.Clause.Owner, o.Clause.ID)
	if len(rs) == 0 || rs[len(rs)-1].Result != "inconclusive" {
		return false
	}
	for _, q := range p.Of("question") {
		about := md.Names(p.FM(q, "about"))
		if p.S(q, "status") == "open" && contains(about, o.Entity) && contains(about, o.Clause.Owner) {
			return true
		}
	}
	return false
}

// ---------- lint and validation ----------

func (p *Project) computeLint() {
	for _, s := range p.Of("service") {
		eff := p.effectiveAdopts(s)
		for _, i := range md.Names(p.FM(s, "interfaces")) {
			for _, sc := range md.Names(p.FM(i, "schemas")) {
				if eff[sc] {
					continue
				}
				waived := false
				for _, w := range p.Waivers {
					if w.Entity == s && w.Kind == "lint" && strings.HasPrefix(w.Target, "lint L1") {
						waived = true
					}
				}
				if !waived {
					p.Lint = append(p.Lint, Issue{"L1", s, fmt.Sprintf("[[%s]] is attached to [[%s]], whose schema is [[%s]], but does not adopt it", s, i, sc)})
				}
			}
		}
		if len(md.Names(p.FM(s, "systems"))) == 0 {
			p.Lint = append(p.Lint, Issue{"L3", s, fmt.Sprintf("[[%s]] belongs to no system", s)})
		}
	}
	for _, i := range p.Of("interface") {
		n := 0
		for _, s := range p.Of("service") {
			if contains(md.Names(p.FM(s, "interfaces")), i) {
				n++
			}
		}
		if n < 2 && p.S(i, "external") == "" {
			p.Lint = append(p.Lint, Issue{"L2", i, fmt.Sprintf("[[%s]] has %d attached service and no external party", i, n)})
		}
	}
}

func (p *Project) validate() {
	codes := map[string][]string{}
	var order []string
	for _, n := range p.order {
		if c := p.S(n, "code"); c != "" {
			if _, ok := codes[c]; !ok {
				order = append(order, c)
			}
			codes[c] = append(codes[c], n)
		}
	}
	for _, c := range order {
		if len(codes[c]) > 1 {
			p.Validation = append(p.Validation, Issue{"V2", codes[c][0], fmt.Sprintf("code %s is used by %s", c, strings.Join(codes[c], ", "))})
		}
	}
	for _, c := range p.Clauses {
		if c.State == "requirement" && md.First(c.F["on"]) == "" {
			p.Validation = append(p.Validation, Issue{"V4", c.Owner, c.Link() + " has no on"})
		}
	}
	for _, k := range sortedResultKeys(p.Results) {
		e := strings.SplitN(k, "|", 2)[0]
		for _, r := range p.Results[k] {
			cid := strings.SplitN(k, "|", 3)[2]
			if r.Result == "met-none" && !strings.HasPrefix(r.Applies, "none:") {
				p.Validation = append(p.Validation, Issue{"V5", e, fmt.Sprintf("met-none for %s in %s has no reason", cid, r.Check)})
			}
			if r.Result == "inconclusive" && !strings.Contains(r.Evidence, "reason:") {
				p.Validation = append(p.Validation, Issue{"V5", e, fmt.Sprintf("inconclusive for %s in %s has no reason", cid, r.Check)})
			}
			if len(r.Deps) == 0 && conclusive[r.Result] {
				p.Validation = append(p.Validation, Issue{"V5", e, fmt.Sprintf("%s in %s has no dependency", cid, r.Check)})
			}
		}
	}
	owner := map[string]string{}
	for _, g := range p.Of("goal") {
		if s := p.S(g, "status"); s == "done" || s == "dropped" {
			continue
		}
		for _, a := range p.Changes[g] {
			if (a.Verb == "add" || a.Verb == "revise" || a.Verb == "retire") && len(a.Links) > 0 {
				k := a.Links[0].Name + "#" + a.Links[0].Anchor
				if o, ok := owner[k]; ok {
					p.Validation = append(p.Validation, Issue{"V6", g, fmt.Sprintf("%s and %s both change %s", g, o, k)})
				}
				owner[k] = g
			}
		}
	}
	for _, g := range p.Of("goal") {
		if !p.Standing(g) {
			continue
		}
		for _, t := range p.Tasks[g] {
			if t.F["finding"] == "" && t.F["question"] == "" && t.F["by"] == "" {
				p.Validation = append(p.Validation, Issue{"V7", g, "upkeep task " + t.ID + " names no source"})
			}
		}
	}
	for _, o := range p.Obligations {
		if o.Pending() || o.Status != "violated" {
			continue
		}
		found := false
		for _, f := range p.Finding {
			if f.Kind == "violation" && f.Entity == o.Entity && f.Status == "open" {
				found = true
			}
		}
		if !found {
			p.Validation = append(p.Validation, Issue{"core", o.Entity, o.Clause.Link() + " is violated and has no open finding"})
		}
	}
}

// ---------- colors ----------

// Color returns the canvas color of an entity card ("" for none).
func (p *Project) Color(e string) string {
	inside := p.Inside(e)
	for _, n := range p.Of("finding") {
		f := p.Finding[n]
		if f.Kind == "violation" && f.Status == "open" && contains(inside, f.Entity) {
			return "1"
		}
	}
	t := p.Type(e)
	if t != "project" && len(p.ActiveOf(e)) > 0 && !p.CheckedEver(e) {
		return ""
	}
	scope := []string{e}
	if t == "system" || t == "project" {
		scope = inside
	}
	var obs []*Obligation
	for _, x := range scope {
		if p.CheckedEver(x) {
			obs = append(obs, p.ActiveOf(x)...)
		}
	}
	f := Count(obs)
	switch {
	case f.Total == 0:
		return ""
	case f.Unverified > 0:
		return "3"
	case f.Waived > 0:
		return "5"
	}
	return "4"
}

// ContractRed reports whether an open violation of a contract exists in a group.
func (p *Project) ContractRed(contract, owner string) bool {
	inside := p.Inside(owner)
	for _, n := range p.Of("finding") {
		f := p.Finding[n]
		if f.Kind == "violation" && f.Status == "open" && f.Contract == "[["+contract+"]]" && contains(inside, f.Entity) {
			return true
		}
	}
	return false
}

// ---------- helpers ----------

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedResultKeys(m map[string][]*Result) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ToApply returns the questions and findings whose chosen option waits for an agent to
// carry out its change.
func (p *Project) ToApply() []string {
	var out []string
	for _, n := range p.order {
		section := "Options"
		switch p.Docs[n].Type {
		case "question":
			section = "Answer"
		case "finding":
		default:
			continue
		}
		if md.Fields(md.Section(p.Docs[n].Body, section))["apply"] == "pending" {
			out = append(out, n)
		}
	}
	return out
}
