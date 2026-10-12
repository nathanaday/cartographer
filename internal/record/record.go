// Package record turns a checker's report into documents: a check (or a section of a
// sweep), and the findings it opens, reopens, or resolves (Specification 5.5, 4.8).
package record

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/refresh"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Row is one result of a report.
type Row struct {
	Clause    string   `json:"clause"` // [[Contract#ID]] or [[Goal#G-1]]
	Rev       int      `json:"rev"`
	Applies   string   `json:"applies"`
	Result    string   `json:"result"` // met, met-none, violated, inconclusive
	Fails     string   `json:"fails,omitempty"`
	Evidence  string   `json:"evidence"`
	DependsOn []string `json:"depends_on"`
}

// Dep is one compared dependency of a report.
type Dep struct {
	Dependency string `json:"dependency"`
	Result     string `json:"result"` // convergence, divergence, absence
	Evidence   string `json:"evidence"`
	Title      string `json:"title,omitempty"` // the name of the finding a divergence or absence opens
}

// Coverage is one row of a sweep's coverage check.
type Coverage struct {
	Repository string `json:"repository"`
	Entry      string `json:"entry"`
	Result     string `json:"result"` // mapped, unmapped
	Evidence   string `json:"evidence"`
	Title      string `json:"title,omitempty"`
}

// Report is what a checker returns.
type Report struct {
	Target       string   `json:"target"`
	Repository   string   `json:"repository,omitempty"`
	Commit       string   `json:"commit,omitempty"`
	Ref          string   `json:"ref,omitempty"`
	Repositories []string `json:"repositories,omitempty"` // a system check
	Commits      []string `json:"commits,omitempty"`
	By           string   `json:"by,omitempty"`
	Session      string   `json:"session,omitempty"`
	Goal         string   `json:"goal,omitempty"`
	Sweep        string   `json:"sweep,omitempty"` // a sweep document to add this check to
	Rows         []Row    `json:"rows"`
	Dependencies []Dep    `json:"dependencies,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

// Outcome is what a record wrote.
type Outcome struct {
	Check                      string
	Opened, Reopened, Resolved []string
	Commit                     string
}

var results = map[string]bool{"met": true, "met-none": true, "violated": true, "inconclusive": true}

// Validate refuses a report that breaks a format rule (Specification 4.9, V3 and V5), or
// that names an obligation the target does not have.
func Validate(p *core.Project, r Report) error {
	if _, ok := p.Docs[r.Target]; !ok {
		return fmt.Errorf("no entity named %q", r.Target)
	}
	if len(r.Rows) == 0 && len(r.Dependencies) == 0 {
		return errors.New("the report has no rows")
	}
	var errs []string
	for _, row := range r.Rows {
		ls := md.Links(row.Clause)
		if len(ls) == 0 {
			errs = append(errs, fmt.Sprintf("%q is not a clause link", row.Clause))
			continue
		}
		if !results[row.Result] {
			errs = append(errs, fmt.Sprintf("%s: result %q is not met, met-none, violated, or inconclusive", row.Clause, row.Result))
		}
		if row.Result == "met-none" && !strings.HasPrefix(row.Applies, "none:") {
			errs = append(errs, fmt.Sprintf("%s: met-none needs \"none: <reason>\" in applies", row.Clause))
		}
		if row.Result == "inconclusive" && !strings.Contains(row.Evidence, "reason:") {
			errs = append(errs, fmt.Sprintf("%s: inconclusive needs \"reason: …\" in evidence", row.Clause))
		}
		if row.Result != "inconclusive" && len(row.DependsOn) == 0 {
			errs = append(errs, fmt.Sprintf("%s: a conclusive result needs at least one dependency", row.Clause))
		}
		if row.Result == "violated" && strings.TrimSpace(row.Fails) == "" {
			errs = append(errs, fmt.Sprintf("%s: a violation needs the parts that fail", row.Clause))
		}
		found := false
		for _, o := range p.Obligations {
			if o.Entity == r.Target && o.Clause.Owner == ls[0].Name && o.Clause.ID == ls[0].Anchor && o.Clause.Rev == row.Rev {
				found = true
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("%s rev %d is not an obligation of %s", row.Clause, row.Rev, r.Target))
		}
	}
	for _, d := range r.Dependencies {
		if d.Result != "convergence" && d.Result != "divergence" && d.Result != "absence" {
			errs = append(errs, fmt.Sprintf("dependency %s: result %q is not convergence, divergence, or absence", d.Dependency, d.Result))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func newID(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return prefix + string(b)
}

func cell(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "|", "/")
	if s == "" {
		return "—"
	}
	return s
}

func tables(r Report) string {
	var b strings.Builder
	b.WriteString("| Clause | Rev | Applies to | Result | Fails | Evidence | Depends on |\n|---|---|---|---|---|---|---|\n")
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s | %s |\n", row.Clause, row.Rev, cell(row.Applies), row.Result,
			cell(row.Fails), cell(row.Evidence), strings.Join(row.DependsOn, ", "))
	}
	deps := "No dependencies were compared in this check.\n"
	if len(r.Dependencies) > 0 {
		var d strings.Builder
		d.WriteString("| Dependency | Result | Evidence |\n|---|---|---|\n")
		for _, x := range r.Dependencies {
			fmt.Fprintf(&d, "| %s | %s | %s |\n", cell(x.Dependency), x.Result, cell(x.Evidence))
		}
		deps = d.String()
	}
	return "## Obligations\n\n" + b.String() + "\n## Dependencies\n\n" + deps
}

func counts(rows []Row) (met, violated, inconclusive int) {
	for _, r := range rows {
		switch r.Result {
		case "met", "met-none":
			met++
		case "violated":
			violated++
		case "inconclusive":
			inconclusive++
		}
	}
	return
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func notes(lines []string) string {
	var out []string
	for _, l := range lines {
		out = append(out, "- "+strings.TrimPrefix(l, "- "))
	}
	return strings.Join(out, "\n")
}

// Record writes a report and its findings, and refreshes the project, as one commit.
func Record(folder string, env core.Env, r Report, now time.Time) (*Outcome, error) {
	p0, err := core.Load(folder, env)
	if err != nil {
		return nil, err
	}
	if err := Validate(p0, r); err != nil {
		return nil, err
	}
	if r.By == "" {
		r.By = "agent"
	}
	if r.Ref == "" {
		r.Ref = p0.Default
	}
	root := vault.Root(p0.Folder)
	w := vault.NewWriter(root)
	defer w.Abort()
	out := &Outcome{}
	met, violated, inconclusive := counts(r.Rows)
	if r.Sweep != "" {
		path, ok := p0.Docs[r.Sweep]
		if !ok || md.Str(path.FM["sweep"]) != "true" {
			return nil, fmt.Errorf("no running sweep named %q", r.Sweep)
		}
		text, _ := w.Read(path.Path)
		loc := fmt.Sprintf("[repository:: %s] [commit:: %s] [ref:: %s] [at:: %s]", r.Repository, short(r.Commit), r.Ref, now.Format("2006-01-02T15:04:05"))
		if len(r.Repositories) > 0 {
			loc = fmt.Sprintf("[repositories:: %s] [commits:: %s] [at:: %s]", strings.Join(r.Repositories, ", "), strings.Join(r.Commits, ", "), now.Format("2006-01-02T15:04:05"))
		}
		section := "## " + r.Target + "\n\n" + loc + "\n\n" + strings.ReplaceAll(tables(r), "## ", "### ")
		if len(r.Notes) > 0 {
			section += "\n### Notes\n\n" + notes(r.Notes) + "\n"
		}
		text = insertBefore(text, "## Coverage\n", section+"\n")
		w.Put(path.Path, text)
		out.Check = r.Sweep + "#" + r.Target
	} else {
		name := fmt.Sprintf("%s %s · %s", now.Format("2006-01-02 1504"), r.Target, short(r.Commit))
		if len(r.Commits) > 0 {
			var cs []string
			for _, c := range r.Commits {
				cs = append(cs, short(c))
			}
			name = fmt.Sprintf("%s %s · %s", now.Format("2006-01-02 1504"), r.Target, strings.Join(cs, ", "))
		}
		path := filepath.Join(p0.Folder, "Checks", now.Format("2006-01"), name+".md")
		fm := []string{"---", "id: " + newID("chk-"), "type: carto-check",
			"description: " + md.Quote(fmt.Sprintf("%s at %s: %d met, %d violated, %d inconclusive.", r.Target, strings.SplitN(name, " · ", 2)[1], met, violated, inconclusive)),
			"tags: []", "created: " + now.Format("2006-01-02T15:04:05"), "updated: " + now.Format("2006-01-02T15:04:05"),
			"target: " + md.Quote("[["+r.Target+"]]")}
		if len(r.Repositories) > 0 {
			fm = append(fm, "repositories: "+md.Value(r.Repositories), "commits: "+md.Value(r.Commits))
		} else {
			fm = append(fm, "repository: "+r.Repository, "commit: "+short(r.Commit), "ref: "+r.Ref)
		}
		goal := ""
		if r.Goal != "" {
			goal = "[[" + r.Goal + "]]"
		}
		fm = append(fm, "by: "+r.By, "session: "+md.Value(r.Session), "goal: "+md.Value(goal),
			fmt.Sprintf("met: %d", met), fmt.Sprintf("violated: %d", violated), fmt.Sprintf("inconclusive: %d", inconclusive), "---")
		text := strings.Join(fm, "\n") + "\n\n" + tables(r) + "\n## Notes\n\n" + notes(r.Notes) + "\n"
		w.Put(path, text)
		out.Check = name
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	p1, err := core.Load(folder, env)
	if err != nil {
		return nil, err
	}
	if err := findings(p0, p1, r, out, w, now); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	p2, err := core.Load(folder, env)
	if err != nil {
		return nil, err
	}
	if _, err := refresh.Stage(p2, w, false); err != nil {
		return nil, err
	}
	out.Commit, err = w.Commit("record " + out.Check)
	return out, err
}

func insertBefore(text, marker, block string) string {
	if i := strings.Index(text, "\n"+marker); i >= 0 {
		return text[:i+1] + block + "\n" + text[i+1:]
	}
	return strings.TrimRight(text, "\n") + "\n\n" + block
}

// findings opens a finding for each new failure, adds a History line to each finding that
// this check reopens or resolves, and names nothing else.
func findings(p0, p1 *core.Project, r Report, out *Outcome, w *vault.Writer, now time.Time) error {
	check := out.Check
	at := now.Format("2006-01-02 15:04")
	history := func(name, line string) error {
		d := p1.Docs[name]
		text, err := w.Read(d.Path)
		if err != nil {
			return err
		}
		text = md.AppendToSection(text, "History", "- "+at+" "+line)
		text, err = md.SetField(text, "updated", now.Format("2006-01-02T15:04:05"))
		if err != nil {
			return err
		}
		w.Put(d.Path, text)
		return nil
	}
	for _, n := range p1.Of("finding") {
		before, after := p0.Finding[n], p1.Finding[n]
		if before == nil || after == nil || after.Entity != r.Target {
			continue
		}
		switch {
		case before.Status != "open" && after.Status == "open":
			out.Reopened = append(out.Reopened, n)
			if err := history(n, "reopened by [["+check+"]]."); err != nil {
				return err
			}
		case before.Status == "open" && after.Status == "resolved":
			out.Resolved = append(out.Resolved, n)
			if err := history(n, "resolved by [["+check+"]]."); err != nil {
				return err
			}
		}
	}
	// New violations: an active obligation of the target that is violated and has no finding.
	for _, row := range r.Rows {
		if row.Result != "violated" {
			continue
		}
		l := md.Links(row.Clause)[0]
		var o *core.Obligation
		for _, x := range p1.Obligations {
			if x.Entity == r.Target && x.Clause.Owner == l.Name && x.Clause.ID == l.Anchor && !x.Pending() {
				o = x
			}
		}
		if o == nil || o.Status != "violated" || hasFinding(p1, "violation", r.Target, l.Name+"#"+l.Anchor, "") {
			continue
		}
		title := fmt.Sprintf("%s violated in %s", l.Anchor, r.Target)
		body := fmt.Sprintf("\n## Finding\n\n%s requires: %s %s fails it: %s.\n\n## Evidence\n\n- %s\n\n## Options\n\n%s\n## History\n\n- %s opened by [[%s]].\n",
			row.Clause, o.Clause.Statement, "[["+r.Target+"]]", row.Fails, row.Evidence, defaultOptions("violation", row.Clause, r.Target), at, check)
		if err := create(p1, w, title, "violation", r.Target, row.Clause, "", "", check, now, body); err != nil {
			return err
		}
		out.Opened = append(out.Opened, title)
	}
	for _, d := range r.Dependencies {
		if d.Result == "convergence" {
			continue
		}
		key := depKey(d.Dependency)
		if hasFinding(p1, d.Result, r.Target, "", key) {
			continue
		}
		name := title(d.Title)
		if name == "" {
			if d.Result == "divergence" {
				name = title(fmt.Sprintf("%s uses %s without declaring it", r.Target, key))
			} else {
				name = title(fmt.Sprintf("%s does not use %s", r.Target, key))
			}
		}
		what := "The code has a dependency that the map does not declare."
		if d.Result == "absence" {
			what = "The map declares a relation that the code does not have."
		}
		body := fmt.Sprintf("\n## Finding\n\n%s %s: %s.\n\n## Evidence\n\n- %s\n\n## Options\n\n%s\n## History\n\n- %s opened by [[%s]].\n",
			what, "[["+r.Target+"]]", d.Dependency, d.Evidence, defaultOptions(d.Result, d.Dependency, r.Target), at, check)
		if err := create(p1, w, name, d.Result, r.Target, "", "", key, check, now, body); err != nil {
			return err
		}
		out.Opened = append(out.Opened, name)
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[\\/:*?"<>|#^\[\]]`)

func safe(s string) string { return strings.TrimSpace(unsafeName.ReplaceAllString(s, "-")) }

var scheme = regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://`)

// title makes a finding's name from a text that may hold links, code, and paths: it keeps
// the words, shows a path's folders with ›, and keeps it short enough for a file name.
func title(s string) string {
	s = scheme.ReplaceAllString(s, "")
	s = strings.NewReplacer("[[", "", "]]", "", "`", "", "<", "", ">", "", " /", " ", "/", " › ").Replace(s)
	s = strings.Join(strings.Fields(safe(s)), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120])
		if i := strings.LastIndex(s, " "); i > 60 {
			s = s[:i]
		}
		s = strings.TrimRight(s, " -,(›")
	}
	return s
}

// depKey is the name of a dependency without the description a checker may add after it.
func depKey(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " ("); i > 0 {
		s = s[:i]
	}
	return strings.Trim(strings.TrimSpace(s), "`")
}

func hasFinding(p *core.Project, kind, entity, clause, key string) bool {
	for _, n := range p.Of("finding") {
		f := p.Finding[n]
		if f.Kind != kind || f.Entity != entity {
			continue
		}
		if clause != "" {
			for _, l := range md.Links(p.FM(n, "clause")) {
				if l.Name+"#"+l.Anchor == clause {
					return true
				}
			}
		}
		if key != "" && p.S(n, "key") == key {
			return true
		}
	}
	return false
}

func defaultOptions(kind, subject, entity string) string {
	switch kind {
	case "violation":
		return fmt.Sprintf("1. **Add an upkeep task** (recommended). The runner starts it at once.\n   Change: add a task for [[%s]] to [[Upkeep]], with this finding as its source.\n2. **Start a goal.**\n3. **Waive until a date.**\n   Change: add a waiver of %s to [[%s]].\n", entity, subject, entity)
	case "divergence":
		return fmt.Sprintf("1. **Declare it on the map now** (recommended). Attach the interface, or add the store or platform service.\n   Change: declare %s on [[%s]].\n2. **Remove the use**, as an upkeep task.\n3. **Waive until a date.**\n", subject, entity)
	case "absence":
		return fmt.Sprintf("1. **Remove it from the map** (recommended).\n   Change: remove %s from [[%s]].\n2. **Add the use to the code**, as an upkeep task.\n3. **Waive until a date.**\n", subject, entity)
	}
	return "1. **Add it to the map** (recommended).\n2. **Waive.**\n"
}

func create(p *core.Project, w *vault.Writer, title, kind, entity, clause, about, key, check string, now time.Time, body string) error {
	path := filepath.Join(p.Folder, "Findings", title+".md")
	for i := 2; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		path = filepath.Join(p.Folder, "Findings", fmt.Sprintf("%s (%d).md", title, i))
	}
	e := ""
	if entity != "" {
		e = "[[" + entity + "]]"
	}
	ts := now.Format("2006-01-02T15:04:05")
	fm := []string{"---", "id: " + newID("fnd-"), "type: carto-finding",
		"description: " + md.Value(strings.TrimSuffix(title, ".")+"."), "tags: []", "created: " + ts, "updated: " + ts,
		"kind: " + kind, "entity: " + md.Value(e), "clause: " + md.Value(clause), "about: " + md.Value(about), "key: " + md.Value(key),
		"check: " + md.Quote("[["+check+"]]"), "found: " + now.Format("2006-01-02"), `defer: ""`,
		`status: ""`, `contract: ""`, "resolution: []", `resolved: ""`, "---"}
	w.Put(path, strings.Join(fm, "\n")+"\n"+body)
	return nil
}

// StartSweep creates the document of a new sweep and returns its name.
func StartSweep(folder string, now time.Time) (string, error) {
	abs, _ := filepath.Abs(folder)
	project := ""
	ix, _ := filepath.Glob(filepath.Join(abs, "*.md"))
	for _, f := range ix {
		if d, err := md.Load(f); err == nil && d.Type == "project" {
			project = d.Name
		}
	}
	if project == "" {
		return "", fmt.Errorf("%s holds no project document", folder)
	}
	name := now.Format("2006-01-02 1504") + " Sweep"
	path := filepath.Join(abs, "Checks", now.Format("2006-01"), name+".md")
	ts := now.Format("2006-01-02T15:04:05")
	text := strings.Join([]string{"---", "id: " + newID("chk-"), "type: carto-check", `description: "A sweep, running."`, "tags: []",
		"created: " + ts, "updated: " + ts, "target: " + md.Quote("[["+project+"]]"), "sweep: true", "repositories: []", "commits: []",
		"by: agent", `session: ""`, `goal: ""`, "met: 0", "violated: 0", "inconclusive: 0", "---", "",
		"## Coverage", "", "Not run yet.", "", "## Notes", ""}, "\n") + "\n"
	w := vault.NewWriter(vault.Root(abs))
	w.Put(path, text)
	_, err := w.Commit("start " + name)
	return name, err
}

// FinishSweep writes a sweep's coverage and notes and its counts, and opens the findings
// of unmapped entry points.
func FinishSweep(folder string, env core.Env, sweep string, coverage []Coverage, sweepNotes []string, now time.Time) (*Outcome, error) {
	p, err := core.Load(folder, env)
	if err != nil {
		return nil, err
	}
	d, ok := p.Docs[sweep]
	if !ok {
		return nil, fmt.Errorf("no sweep named %q", sweep)
	}
	w := vault.NewWriter(vault.Root(p.Folder))
	defer w.Abort()
	text, _ := w.Read(d.Path)
	var cov strings.Builder
	cov.WriteString("| Repository | Entry point | Result | Evidence |\n|---|---|---|---|\n")
	for _, c := range coverage {
		fmt.Fprintf(&cov, "| %s | %s | %s | %s |\n", c.Repository, cell(c.Entry), c.Result, cell(c.Evidence))
	}
	i := strings.Index(text, "\n## Coverage\n")
	if i < 0 {
		return nil, fmt.Errorf("%s has no Coverage section", sweep)
	}
	text = text[:i+1] + "## Coverage\n\n" + cov.String() + "\n## Notes\n\n" + notes(sweepNotes) + "\n"
	met, violated, inconclusive := 0, 0, 0
	var repos, commits []string
	for _, n := range p.CheckNames() {
		if !strings.HasPrefix(n, sweep+"#") {
			continue
		}
		for _, rs := range p.Results {
			for _, r := range rs {
				if r.Check != n {
					continue
				}
				switch r.Result {
				case "met", "met-none":
					met++
				case "violated":
					violated++
				case "inconclusive":
					inconclusive++
				}
			}
		}
	}
	for _, line := range regexp.MustCompile(`\[repository:: ([^\]]+)\] \[commit:: ([^\]]+)\]`).FindAllStringSubmatch(text, -1) {
		repos, commits = append(repos, line[1]), append(commits, line[2])
	}
	for k, v := range map[string]any{"met": met, "violated": violated, "inconclusive": inconclusive, "repositories": repos, "commits": commits,
		"description": fmt.Sprintf("Sweep: %d met, %d violated, %d inconclusive.", met, violated, inconclusive), "updated": now.Format("2006-01-02T15:04:05")} {
		if text, err = md.SetField(text, k, v); err != nil {
			return nil, err
		}
	}
	w.Put(d.Path, text)
	out := &Outcome{Check: sweep}
	for _, c := range coverage {
		if c.Result != "unmapped" {
			continue
		}
		key := c.Repository + ":" + strings.Trim(c.Entry, "`")
		if hasFinding(p, "unmapped", "", "", key) {
			continue
		}
		name := title(c.Title)
		if name == "" {
			name = title(strings.Trim(c.Entry, "`") + " in " + filepath.Base(c.Repository) + " is in no entity")
		}
		body := fmt.Sprintf("\n## Finding\n\n%s in %s is an entry point that no entity lists in `entry`.\n\n## Evidence\n\n- %s\n\n## Options\n\n%s\n## History\n\n- %s opened by the coverage check of [[%s]].\n",
			c.Entry, c.Repository, c.Evidence, defaultOptions("unmapped", "", ""), now.Format("2006-01-02 15:04"), sweep)
		if err := create(p, w, name, "unmapped", "", "", "", key, sweep, now, body); err != nil {
			return nil, err
		}
		out.Opened = append(out.Opened, name)
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	p2, err := core.Load(folder, env)
	if err != nil {
		return nil, err
	}
	if _, err := refresh.Stage(p2, w, false); err != nil {
		return nil, err
	}
	out.Commit, err = w.Commit("finish " + sweep)
	return out, err
}
