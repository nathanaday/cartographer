// Package core computes everything Cartographer derives from a project's documents:
// obligations, results and status, staleness, findings, goal progress, lint, validation,
// and the run queue. It reads; it never writes.
package core

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/md"
)

// Clause is one rule: of a contract, an entity's requirements, or a goal's requirements.
type Clause struct {
	Owner, ID, Statement string
	F                    map[string]string
	State                string // pending, active, retired, or requirement (a goal's)
	Rev                  int
	Scope, Verify        string
	Goal                 string
}

// Link is the clause as a wikilink.
func (c *Clause) Link() string { return "[[" + c.Owner + "#" + c.ID + "]]" }

// Key names the clause as owner#id.
func (c *Clause) Key() string { return c.Owner + "#" + c.ID }

// Result is one row of a check: one obligation at one commit.
type Result struct {
	Check, Ref, Repo, Commit  string
	Date                      time.Time
	Rev                       int
	Applies, Result, Evidence string
	Fails, Deps               []string
}

// Task is one task line of a goal.
type Task struct {
	Box, ID, Target, Text, Block, Goal string
	F                                  map[string]string
}

// Waiver is one line of a Waivers section.
type Waiver struct {
	Entity, Kind, Target, Reason string // Target: owner#id, "lint L1 [[x]]", or "dependency `k`"
	F                            map[string]string
}

// Dependency is one row of a check's Dependencies or a sweep's Coverage.
type Dependency struct {
	Date                     time.Time
	Check, Key, Result, Note string
}

// Act is one line of a goal's Map changes.
type Act struct {
	Verb  string
	Links []md.Link
}

// Landing is one "landed" line of a goal's Log.
type Landing struct {
	At        time.Time
	Repo, SHA string
	Task      string
}

// Issue is one lint or validation result.
type Issue struct {
	Rule, Entity, Message string
}

// CheckInfo is what a check (or a section of a sweep) is about.
type CheckInfo struct {
	Target string
	Date   time.Time
	Ref    string
}

// Project is one indexed project folder.
type Project struct {
	Folder  string
	Env     Env
	Docs    map[string]*md.Doc
	order   []string
	Project string
	Default string

	StaleAfter, NudgeAfter int

	Clauses      []*Clause
	Tasks        map[string][]*Task
	Changes      map[string][]Act
	Results      map[string][]*Result // key: entity|owner|id
	Checks       map[string]CheckInfo
	checkOrder   []string
	Dependencies map[string][]Dependency
	Waivers      []*Waiver

	Obligations []*Obligation
	Drafts      map[string][]*Obligation
	Finding     map[string]*FindingState
	Lint        []Issue
	Validation  []Issue
}

// Load indexes a project folder and computes everything.
func Load(folder string, env Env) (*Project, error) {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return nil, err
	}
	p := &Project{Folder: abs, Env: env, Docs: map[string]*md.Doc{}}
	var paths []string
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		d, err := md.Load(path)
		if err != nil {
			p.Validation = append(p.Validation, Issue{"V1", filepath.Base(path), err.Error()})
			continue
		}
		if _, dup := p.Docs[d.Name]; !dup {
			p.order = append(p.order, d.Name)
		}
		p.Docs[d.Name] = d
	}
	projects := p.Of("project")
	if len(projects) == 0 {
		return nil, fmt.Errorf("%s holds no carto-project document", folder)
	}
	p.Project = projects[0]
	p.Default = p.strOr(p.Project, "default_branch", "main")
	p.StaleAfter = p.intOr(p.Project, "stale_after", 30)
	p.NudgeAfter = p.intOr(p.Project, "nudge_after", 14)
	p.parseClauses()
	p.parseGoals()
	p.parseChecks()
	p.parseWaivers()
	p.computeObligations()
	p.computeStatus()
	p.computeFindings()
	p.computeLint()
	p.validate()
	return p, nil
}

// Of returns the documents of one type, in path order.
func (p *Project) Of(t string) []string {
	var out []string
	for _, n := range p.order {
		if p.Docs[n].Type == t {
			out = append(out, n)
		}
	}
	return out
}

// FM returns a frontmatter value of a document, or nil.
func (p *Project) FM(name, key string) any {
	if d, ok := p.Docs[name]; ok {
		return d.FM[key]
	}
	return nil
}

// S returns a frontmatter value as text.
func (p *Project) S(name, key string) string { return md.Str(p.FM(name, key)) }

func (p *Project) strOr(name, key, def string) string {
	if s := p.S(name, key); s != "" {
		return s
	}
	return def
}

func (p *Project) intOr(name, key string, def int) int {
	switch v := p.FM(name, key).(type) {
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Type returns the type of a document, or "".
func (p *Project) Type(name string) string {
	if d, ok := p.Docs[name]; ok {
		return d.Type
	}
	return ""
}

var (
	clauseRe = regexp.MustCompile(`(?m)^### (\S+)\n`)
	taskRe   = regexp.MustCompile(`^- \[(.)\] (T\d+) \[\[([^\]]+)\]\] (.*)$`)
	actRe    = regexp.MustCompile(`^- \*\*(add|revise|retire|adopt|attach|use|join|create)\*\* (.*)$`)
	waiverRe = regexp.MustCompile("^- ((?:lint L\\d+ )?\\[\\[[^\\]]+\\]\\]|dependency `[^`]+`) · (.*)$")
	landRe   = regexp.MustCompile(`(?m)^- (\d{4}-\d\d-\d\d \d\d:\d\d) .*?landed (?:(T\d+) )?([\w-]+) (\w+)`)
)

func (p *Project) parseClauses() {
	for _, n := range p.order {
		d := p.Docs[n]
		switch d.Type {
		case "contract", "system", "service", "module", "goal":
		default:
			continue
		}
		body := md.Section(d.Body, "Clauses") + md.Section(d.Body, "Requirements")
		for _, blk := range clauseBlocks(body) {
			var lines []string
			for _, l := range strings.Split(strings.TrimSpace(blk[1]), "\n") {
				if strings.TrimSpace(l) != "" {
					lines = append(lines, l)
				}
			}
			var stmt, flds, pend []string
			for _, l := range lines {
				switch {
				case strings.HasPrefix(l, "["):
					flds = append(flds, l)
				case strings.HasPrefix(l, ">"):
					pend = append(pend, l)
				default:
					stmt = append(stmt, l)
				}
			}
			f := md.Fields(strings.Join(flds, " "))
			c := newClause(n, blk[0], strings.Join(stmt, " "), f, 0, "")
			if d.Type == "goal" {
				c.State, c.Goal = "requirement", n
			}
			p.Clauses = append(p.Clauses, c)
			if pr, ok := f["pending_rev"]; ok {
				var ps []string
				for _, l := range pend {
					l = strings.TrimPrefix(l, ">")
					l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "[!pending]"))
					ps = append(ps, l)
				}
				rev, _ := strconv.Atoi(pr)
				p.Clauses = append(p.Clauses, newClause(n, blk[0], strings.Join(ps, " "), f, rev, md.First(f["pending_goal"])))
			}
		}
	}
}

// clauseBlocks splits a text into (id, body) pairs at "### id" headings. A block ends at
// the next "### " or "## " heading.
func clauseBlocks(body string) [][2]string {
	var out [][2]string
	locs := clauseRe.FindAllStringSubmatchIndex(body, -1)
	for i, loc := range locs {
		start := loc[1]
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		text := body[start:end]
		if j := strings.Index(text, "\n## "); j >= 0 {
			text = text[:j+1]
		} else if strings.HasPrefix(text, "## ") {
			text = ""
		}
		out = append(out, [2]string{body[loc[2]:loc[3]], text})
	}
	return out
}

func newClause(owner, id, stmt string, f map[string]string, rev int, pendingGoal string) *Clause {
	c := &Clause{Owner: owner, ID: id, Statement: stmt, F: f, State: "active", Scope: "each", Verify: "agent", Rev: 1}
	if s, ok := f["state"]; ok {
		c.State = s
	}
	if s, ok := f["scope"]; ok {
		c.Scope = s
	}
	if s, ok := f["verify"]; ok {
		c.Verify = s
	}
	if s, ok := f["rev"]; ok {
		if n, err := strconv.Atoi(s); err == nil {
			c.Rev = n
		}
	}
	c.Goal = md.First(f["goal"])
	if pendingGoal != "" {
		c.State, c.Rev, c.Goal = "pending", rev, pendingGoal
	}
	return c
}

// Clause returns the clause owner#id (the active one, or the pending revision).
func (p *Project) Clause(owner, id string, pending bool) *Clause {
	for _, c := range p.Clauses {
		if c.Owner == owner && c.ID == id && (c.State == "pending") == pending {
			return c
		}
	}
	return nil
}

func (p *Project) parseGoals() {
	p.Tasks, p.Changes = map[string][]*Task{}, map[string][]Act{}
	for _, g := range p.Of("goal") {
		for _, line := range strings.Split(md.Section(p.Docs[g].Body, "Tasks"), "\n") {
			m := taskRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			p.Tasks[g] = append(p.Tasks[g], &Task{Box: m[1], ID: m[2], Target: m[3], Text: md.StripFields(m[4]),
				F: md.Fields(m[4]), Block: md.BlockID(m[4]), Goal: g})
		}
		for _, line := range strings.Split(md.Section(p.Docs[g].Body, "Map changes"), "\n") {
			if m := actRe.FindStringSubmatch(line); m != nil {
				p.Changes[g] = append(p.Changes[g], Act{m[1], md.Links(m[2])})
			}
		}
	}
}

// Landings returns the "landed" lines of a goal's Log.
func (p *Project) Landings(g string) []Landing {
	var out []Landing
	for _, m := range landRe.FindAllStringSubmatch(md.Section(p.Docs[g].Body, "Log"), -1) {
		at, _ := time.Parse("2006-01-02 15:04", m[1])
		out = append(out, Landing{At: at, Task: m[2], Repo: m[3], SHA: m[4]})
	}
	return out
}

func resultKey(entity, owner, id string) string { return entity + "|" + owner + "|" + id }

// ResultsOf returns the results of one obligation, oldest first.
func (p *Project) ResultsOf(entity, owner, id string) []*Result {
	return p.Results[resultKey(entity, owner, id)]
}

func (p *Project) parseChecks() {
	p.Results, p.Checks, p.Dependencies = map[string][]*Result{}, map[string]CheckInfo{}, map[string][]Dependency{}
	for _, n := range p.Of("check") {
		d := p.Docs[n]
		if b, _ := d.FM["sweep"].(bool); b {
			for _, s := range md.Sections(d.Body) {
				title, body := s[0], s[1]
				if title == "Coverage" {
					for _, cells := range md.TableRows(body) {
						if len(cells) == 4 && cells[0] != "Repository" {
							p.Dependencies[p.Project] = append(p.Dependencies[p.Project],
								Dependency{md.Time(d.FM["created"]), n, cells[0] + ":" + cells[1], cells[2], cells[3]})
						}
					}
				} else if _, ok := p.Docs[title]; ok {
					f := md.Fields(strings.SplitN(body, "###", 2)[0])
					ref := f["ref"]
					if ref == "" {
						ref = p.Default
					}
					repo := ""
					if r, ok := f["repository"]; ok {
						repo, _ = RepoPath(r)
					}
					p.record(n+"#"+title, title, md.Time(f["at"]), ref, repo, f["commit"], body)
				}
			}
			continue
		}
		repo := ""
		if r := md.Str(d.FM["repository"]); r != "" {
			repo, _ = RepoPath(r)
		}
		ref := md.Str(d.FM["ref"])
		if ref == "" {
			ref = p.Default
		}
		p.record(n, md.First(d.FM["target"]), md.Time(d.FM["created"]), ref, repo, md.Str(d.FM["commit"]), d.Body)
	}
	for _, rs := range p.Results {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].Date.Before(rs[j].Date) })
	}
}

func (p *Project) record(name, target string, date time.Time, ref, repo, commit, body string) {
	p.Checks[name] = CheckInfo{target, date, ref}
	p.checkOrder = append(p.checkOrder, name)
	for _, cells := range md.TableRows(md.Subsection(body, "Obligations")) {
		if len(cells) != 7 || !strings.HasPrefix(cells[0], "[[") {
			continue
		}
		ls := md.Links(cells[0])
		if len(ls) == 0 {
			continue
		}
		rev, _ := strconv.Atoi(cells[1])
		var fails []string
		if cells[4] != "—" && cells[4] != "" {
			for _, x := range strings.Split(cells[4], ",") {
				fails = append(fails, strings.TrimSpace(x))
			}
		}
		k := resultKey(target, ls[0].Name, ls[0].Anchor)
		p.Results[k] = append(p.Results[k], &Result{Check: name, Date: date, Ref: ref, Repo: repo, Commit: commit, Rev: rev,
			Applies: cells[2], Result: cells[3], Fails: fails, Evidence: cells[5], Deps: md.SplitList(cells[6])})
	}
	for _, cells := range md.TableRows(md.Subsection(body, "Dependencies")) {
		if len(cells) == 3 && cells[0] != "Dependency" {
			p.Dependencies[target] = append(p.Dependencies[target], Dependency{date, name, cells[0], cells[1], cells[2]})
		}
	}
}

// CheckNames returns the checks and sweep sections in the order they were read.
func (p *Project) CheckNames() []string { return p.checkOrder }

func (p *Project) parseWaivers() {
	for _, n := range p.order {
		for _, line := range strings.Split(md.Section(p.Docs[n].Body, "Waivers"), "\n") {
			m := waiverRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			w := &Waiver{Entity: n, F: md.Fields(m[2]), Reason: md.StripFields(m[2]), Target: m[1]}
			switch {
			case strings.HasPrefix(m[1], "lint"):
				w.Kind = "lint"
			case strings.HasPrefix(m[1], "dependency"):
				w.Kind = "dependency"
			default:
				w.Kind = "clause"
				l := md.Links(m[1])[0]
				w.Target = l.Name + "#" + l.Anchor
			}
			p.Waivers = append(p.Waivers, w)
		}
	}
}

// WaiverFor returns the active waiver of a target on an entity, or nil.
func (p *Project) WaiverFor(entity, target string, rev int, fails []string) *Waiver {
	for _, w := range p.Waivers {
		if w.Entity != entity || w.Target != target {
			continue
		}
		if r, ok := w.F["rev"]; ok && rev != 0 {
			if n, _ := strconv.Atoi(r); n != rev {
				continue
			}
		}
		if g := md.First(w.F["goal"]); g != "" && p.S(g, "status") == "done" {
			continue
		}
		if u := w.F["until"]; u != "" && md.Time(u).Before(p.Env.Now()) {
			continue
		}
		if parts := splitTrim(w.F["parts"]); len(parts) > 0 && len(fails) > 0 && !subset(fails, parts) {
			continue
		}
		return w
	}
	return nil
}

func splitTrim(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func subset(a, b []string) bool {
	set := map[string]bool{}
	for _, x := range b {
		set[x] = true
	}
	for _, x := range a {
		if !set[x] {
			return false
		}
	}
	return true
}

// RepoPath splits github.com/org/repo/some/path into ("repo", "some/path").
func RepoPath(location string) (string, string) {
	parts := strings.Split(location, "/")
	if len(parts) < 3 {
		return "", ""
	}
	return parts[2], strings.Join(parts[3:], "/")
}
