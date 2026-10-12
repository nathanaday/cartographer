// Package work updates observed work state: tasks, questions, a finding's chosen option,
// defers, and goal logs. It also applies the change that a user's answer or choice implies.
package work

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Work edits the documents of one project.
type Work struct {
	ix  *change.Index
	w   *vault.Writer
	now time.Time
	log []string
}

// New starts a set of work edits on a project folder.
func New(folder string, now time.Time) (*Work, error) {
	ix, err := change.NewIndex(folder)
	if err != nil {
		return nil, err
	}
	return &Work{ix: ix, w: vault.NewWriter(vault.Root(ix.Folder)), now: now}, nil
}

// Index returns the project's index.
func (k *Work) Index() *change.Index { return k.ix }

// Writer returns the writer, to commit or to refresh with.
func (k *Work) Writer() *vault.Writer { return k.w }

// Summary returns a line for each edit made.
func (k *Work) Summary() []string { return k.log }

func (k *Work) read(doc string) (string, string, error) {
	path, ok := k.ix.Paths[doc]
	if !ok {
		return "", "", fmt.Errorf("no document named %q in the project", doc)
	}
	text, err := k.w.Read(path)
	return path, text, err
}

func (k *Work) stamp() string { return k.now.Format("2006-01-02 15:04") }

var taskLine = regexp.MustCompile(`^- \[(.)\] (T\d+) `)

func setField(line, key, value string) string {
	re := regexp.MustCompile(`\[` + regexp.QuoteMeta(key) + `::(?:[^\]]|\]\])*\]`)
	field := "[" + key + ":: " + value + "]"
	if re.MatchString(line) {
		return re.ReplaceAllLiteralString(line, field)
	}
	if i := strings.LastIndex(line, " ^"); i >= 0 {
		return line[:i] + " " + field + line[i:]
	}
	return line + " " + field
}

func (k *Work) editTask(goal, id string, f func(box string, line string) (string, error)) (string, error) {
	path, text, err := k.read(goal)
	if err != nil {
		return "", err
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if m := taskLine.FindStringSubmatch(l); m != nil && m[2] == id {
			next, err := f(m[1], l)
			if err != nil {
				return "", err
			}
			lines[i] = next
			k.w.Put(path, strings.Join(lines, "\n"))
			return l, nil
		}
	}
	return "", fmt.Errorf("[[%s]] has no task %s", goal, id)
}

// Log adds a dated line to a goal's Log and makes it the goal's last activity.
func (k *Work) Log(goal, line string) error {
	path, text, err := k.read(goal)
	if err != nil {
		return err
	}
	if text, err = md.SetField(text, "updated", k.now.Format("2006-01-02T15:04:05")); err != nil {
		return err
	}
	k.w.Put(path, md.AppendToSection(text, "Log", "- "+k.stamp()+" "+strings.TrimSuffix(line, ".")+"."))
	return nil
}

// Done marks a task done with its commit.
func (k *Work) Done(goal, id, commit string) error {
	_, err := k.editTask(goal, id, func(box, l string) (string, error) {
		if box == "x" {
			return "", fmt.Errorf("%s is already done", id)
		}
		l = "- [x]" + l[5:]
		if commit != "" {
			l = setField(l, "commit", commit)
		}
		return setField(l, "done", k.now.Format("2006-01-02T15:04")), nil
	})
	if err != nil {
		return err
	}
	k.log = append(k.log, fmt.Sprintf("%s of [[%s]] done", id, goal))
	line := id + " done"
	if commit != "" {
		line += " at " + commit
	}
	return k.Log(goal, line)
}

// Drop drops a task with a reason. A task that waits on an open question for the user
// cannot be dropped (Specification 5.1.6).
func (k *Work) Drop(goal, id, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("dropping a task needs a reason")
	}
	_, err := k.editTask(goal, id, func(box, l string) (string, error) {
		for _, q := range md.Names(md.Fields(l)["blocked"]) {
			if path, ok := k.ix.Paths[q]; ok && k.ix.Types[q] == "question" {
				if d, err := md.Load(path); err == nil && md.Str(d.FM["status"]) == "open" {
					return "", fmt.Errorf("%s waits on [[%s]], which the user has not answered", id, q)
				}
			}
		}
		return setField("- [-]"+l[5:], "reason", reason), nil
	})
	if err != nil {
		return err
	}
	k.log = append(k.log, fmt.Sprintf("%s of [[%s]] dropped", id, goal))
	return k.Log(goal, id+" dropped: "+reason)
}

// Attempt records a failed attempt at a task.
func (k *Work) Attempt(goal, id, why string) error {
	n := 0
	_, err := k.editTask(goal, id, func(box, l string) (string, error) {
		if box != " " {
			return "", fmt.Errorf("%s of %s is closed; an attempt counts only on an open task", id, goal)
		}
		n, _ = strconv.Atoi(md.Fields(l)["attempts"])
		n++
		return setField(l, "attempts", strconv.Itoa(n)), nil
	})
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("%s failed (attempt %d): %s", id, n, why)
	if n >= 2 {
		msg += ". It is stuck and waits for the user"
	}
	return k.Log(goal, msg)
}

// Block names the question or finding a task waits for.
func (k *Work) Block(goal, id, blocker string) error {
	if _, ok := k.ix.Paths[blocker]; !ok {
		return fmt.Errorf("no document named %q", blocker)
	}
	_, err := k.editTask(goal, id, func(box, l string) (string, error) { return setField(l, "blocked", "[["+blocker+"]]"), nil })
	return err
}

// Add adds a task to a goal and returns its id. An upkeep task names its source.
func (k *Work) Add(goal, target, text string, fields map[string]string) (string, error) {
	if _, ok := k.ix.Paths[goal]; !ok && goal == "Upkeep" {
		k.upkeep()
	}
	path, body, err := k.read(goal)
	if err != nil {
		return "", err
	}
	if _, ok := k.ix.Paths[target]; !ok {
		return "", fmt.Errorf("no entity named %q", target)
	}
	d, _ := md.Parse(path, body)
	if b, _ := d.FM["standing"].(bool); b && fields["finding"] == "" && fields["question"] == "" && fields["by"] == "" {
		return "", errors.New("an upkeep task needs its source: finding, question, or by: user (V7)")
	}
	next := 1
	for _, m := range regexp.MustCompile(`(?m)^- \[.\] T(\d+) `).FindAllStringSubmatch(body, -1) {
		if n, _ := strconv.Atoi(m[1]); n >= next {
			next = n + 1
		}
	}
	id := fmt.Sprintf("T%d", next)
	line := fmt.Sprintf("- [ ] %s [[%s]] %s", id, target, strings.TrimSpace(text))
	for _, key := range []string{"for", "after", "blocked", "finding", "question", "by", "due", "every", "user", "repo"} {
		if v := fields[key]; v != "" {
			line += " [" + key + ":: " + v + "]"
		}
	}
	line += " ^" + strings.ToLower(id)
	k.w.Put(path, md.AppendToSection(body, "Tasks", line))
	k.log = append(k.log, fmt.Sprintf("%s added to [[%s]]", id, goal))
	for _, key := range []string{"finding", "question"} {
		if src := md.First(fields[key]); src != "" {
			if spath, stext, err := k.read(src); err == nil && strings.Contains(stext, "[apply:: pending]") {
				k.w.Put(spath, setApply(stext, "[["+goal+"#^"+strings.ToLower(id)+"]]"))
			}
		}
	}
	return id, k.Log(goal, id+" added: "+text)
}

// upkeep writes the project's standing goal, which the tool keeps: the first upkeep task
// of a project makes it.
func (k *Work) upkeep() {
	ts := k.now.Format("2006-01-02T15:04:05")
	path := filepath.Join(k.ix.Folder, "Goals", "Upkeep.md")
	k.w.Put(path, "---\nid: "+newID("goal-")+"\ntype: carto-goal\ndescription: Work in "+filepath.Base(k.ix.Folder)+" that belongs to no other goal.\ntags: []\n"+
		"created: "+ts+"\nupdated: "+ts+"\nstatus: active\nstanding: true\npriority: medium\nafter: []\ndefer: \"\"\nbranch: \"\"\nresolves: []\naffects: []\n"+
		"progress: \"\"\ntasks: \"\"\nlanded: \"\"\nopened: "+k.now.Format("2006-01-02")+"\nclosed: \"\"\n---\n\n"+
		"## Intent\n\nThe project's standing goal. Each task is already the user's decision, so it needs no approval and never closes. Each task has its own branch, `upkeep/t<n>`, and lands alone.\n\n"+
		"## Tasks\n\n## Task details\n\n## Progress\n\n```cartographer-progress\n```\n\n## Log\n")
	k.ix.Paths["Upkeep"], k.ix.Types["Upkeep"] = path, "goal"
	k.log = append(k.log, "made [[Upkeep]], the project's standing goal")
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

// Option is one option of a question.
type Option struct {
	Title       string `json:"title"`
	Text        string `json:"text,omitempty"`
	Change      string `json:"change,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

var unsafeName = regexp.MustCompile(`[\\/:*?"<>|#^\[\]]`)

// Ask writes a question for the user and blocks the named tasks on it.
func (k *Work) Ask(title, question, context string, options []Option, about []string, blocks [][2]string, session string) (string, error) {
	if len(options) < 2 {
		return "", errors.New("a question needs at least two options")
	}
	name := strings.TrimSpace(unsafeName.ReplaceAllString(strings.TrimSuffix(title, "?"), "-"))
	path := filepath.Join(k.ix.Folder, "Questions", name+".md")
	if _, exists := k.ix.Paths[name]; exists {
		return "", fmt.Errorf("a document named %q exists", name)
	}
	var abouts []string
	for _, a := range about {
		abouts = append(abouts, "[["+a+"]]")
	}
	var opts strings.Builder
	for i, o := range options {
		head := fmt.Sprintf("%d. **%s**", i+1, strings.TrimSuffix(o.Title, "."))
		if o.Recommended {
			head += " (recommended)"
		}
		head += "."
		if o.Text != "" {
			head += " " + o.Text
		}
		opts.WriteString(head + "\n")
		if o.Change != "" {
			opts.WriteString("   Change: " + o.Change + "\n")
		}
	}
	ts := k.now.Format("2006-01-02T15:04:05")
	text := strings.Join([]string{"---", "id: " + newID("q-"), "type: carto-question", "description: " + md.Value(strings.TrimSpace(question)),
		"tags: []", "created: " + ts, "updated: " + ts, "status: open", "about: " + md.Value(abouts), `holds: ""`,
		"asked_by: agent", "session: " + md.Value(session), `answered: ""`, `defer: ""`, "---", "",
		"## Question", "", strings.TrimSpace(question), "", "## Context", "", strings.TrimSpace(context), "",
		"## Options", "", strings.TrimRight(opts.String(), "\n"), "", "## Answer", "", "## Effects", ""}, "\n") + "\n"
	k.w.Put(path, text)
	k.ix.Paths[name], k.ix.Types[name] = path, "question"
	for _, b := range blocks {
		if err := k.Block(b[0], b[1], name); err != nil {
			return "", err
		}
	}
	k.log = append(k.log, "asked [["+name+"]]")
	return name, nil
}

// Answer records the user's answer to a question. It is the user's own act.
func (k *Work) Answer(question string, chosen int, words string) error {
	path, text, err := k.read(question)
	if err != nil {
		return err
	}
	d, err := md.Parse(path, text)
	if err != nil {
		return err
	}
	if md.Str(d.FM["status"]) != "open" {
		return fmt.Errorf("[[%s]] is not open", question)
	}
	if n := len(optionsOf(d.Body)); chosen < 1 || chosen > n {
		return fmt.Errorf("[[%s]] has options 1 to %d", question, n)
	}
	answer := fmt.Sprintf("[chosen:: %d] [on:: %s]", chosen, k.now.Format("2006-01-02")) + pendingApply(d.Body, chosen)
	if w := strings.TrimSpace(words); w != "" {
		answer += "\n\n> " + strings.ReplaceAll(w, "\n", "\n> ")
	}
	text = md.AppendToSection(text, "Answer", answer)
	for key, v := range map[string]any{"status": "answered", "answered": k.now.Format("2006-01-02T15:04"), "updated": k.now.Format("2006-01-02T15:04:05")} {
		if text, err = md.SetField(text, key, v); err != nil {
			return err
		}
	}
	k.w.Put(path, text)
	k.log = append(k.log, fmt.Sprintf("answered [[%s]] with option %d", question, chosen))
	return nil
}

// Choose records the user's choice of a finding's option. It is the user's own act.
func (k *Work) Choose(finding string, chosen int, words string) error {
	path, text, err := k.read(finding)
	if err != nil {
		return err
	}
	d, err := md.Parse(path, text)
	if err != nil {
		return err
	}
	if _, set := md.Fields(md.Section(d.Body, "Options"))["chosen"]; set {
		return fmt.Errorf("[[%s]] already has a chosen option", finding)
	}
	if n := len(optionsOf(d.Body)); chosen < 1 || chosen > n {
		return fmt.Errorf("[[%s]] has options 1 to %d", finding, n)
	}
	text = md.AppendToSection(text, "Options", fmt.Sprintf("\n[chosen:: %d] [by:: user] [on:: %s]", chosen, k.now.Format("2006-01-02"))+pendingApply(d.Body, chosen))
	line := fmt.Sprintf("- %s the user chose option %d", k.stamp(), chosen)
	if w := strings.TrimSpace(words); w != "" {
		line += ": “" + w + "”"
	}
	text = md.AppendToSection(text, "History", line+".")
	if text, err = md.SetField(text, "updated", k.now.Format("2006-01-02T15:04:05")); err != nil {
		return err
	}
	k.w.Put(path, text)
	k.log = append(k.log, fmt.Sprintf("chose option %d of [[%s]]", chosen, finding))
	return nil
}

// Defer hides a goal, question, or finding from Attention until a date. It is the user's
// own act.
func (k *Work) Defer(doc, until string) error {
	if md.Time(until).IsZero() {
		return fmt.Errorf("%q is not a date", until)
	}
	path, text, err := k.read(doc)
	if err != nil {
		return err
	}
	text, err = md.SetField(text, "defer", until)
	if err != nil {
		return err
	}
	k.w.Put(path, text)
	k.log = append(k.log, fmt.Sprintf("deferred [[%s]] until %s", doc, until))
	return nil
}

// Option text of a document: the "N. **title**" lines and their Change lines.
type parsedOption struct {
	N             int
	Title, Change string
}

var optRe = regexp.MustCompile(`(?m)^(\d+)\. \*\*(.+?)\*\*.*$`)

func optionsOf(body string) []parsedOption {
	sec := md.Section(body, "Options")
	var out []parsedOption
	lines := strings.Split(sec, "\n")
	for i, l := range lines {
		m := optRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		o := parsedOption{N: n, Title: m[2]}
		if i+1 < len(lines) {
			if c := strings.TrimSpace(lines[i+1]); strings.HasPrefix(c, "Change:") {
				o.Change = strings.TrimSpace(strings.TrimPrefix(c, "Change:"))
			}
		}
		out = append(out, o)
	}
	return out
}

// Chosen returns the option the user chose in an answered question or a finding, its
// Change text, and whether its change was already applied.
func Chosen(ix *change.Index, doc string) (int, string, bool, error) {
	path, ok := ix.Paths[doc]
	if !ok {
		return 0, "", false, fmt.Errorf("no document named %q", doc)
	}
	d, err := md.Load(path)
	if err != nil {
		return 0, "", false, err
	}
	section := "Options"
	if d.Type == "question" {
		section = "Answer"
	}
	fields := md.Fields(md.Section(d.Body, section))
	n, _ := strconv.Atoi(fields["chosen"])
	if n == 0 {
		return 0, "", false, fmt.Errorf("the user has not chosen an option of [[%s]]", doc)
	}
	for _, o := range optionsOf(d.Body) {
		if o.N == n {
			applied := fields["apply"] != "pending" && (fields["apply"] != "" || strings.Contains(md.Section(d.Body, "Effects")+md.Section(d.Body, "History"), "applied option "+strconv.Itoa(n)))
			return n, o.Change, applied, nil
		}
	}
	return 0, "", false, fmt.Errorf("[[%s]] has no option %d", doc, n)
}

// ApplyChoice applies the declared operations that carry out the option the user chose.
// The user's choice is the approval, so it applies at once as the user's act; it applies
// once.
func ApplyChoice(folder string, w *vault.Writer, doc string, ops []change.Op, now time.Time) (*change.Doc, error) {
	ix, err := change.NewIndex(folder)
	if err != nil {
		return nil, err
	}
	n, changeText, applied, err := Chosen(ix, doc)
	if err != nil {
		return nil, err
	}
	if applied {
		return nil, fmt.Errorf("option %d of [[%s]] was already applied", n, doc)
	}
	if changeText == "" {
		return nil, fmt.Errorf("option %d of [[%s]] carries no change", n, doc)
	}
	c, err := change.UserAct(ix, w, fmt.Sprintf("applied option %d of %s", n, doc), ops, now)
	if err != nil {
		return nil, err
	}
	k := &Work{ix: ix, w: w, now: now}
	path, text, err := k.read(doc)
	if err != nil {
		return nil, err
	}
	section := "History"
	if ix.Types[doc] == "question" {
		section = "Effects"
	}
	text = md.AppendToSection(text, section, fmt.Sprintf("- %s applied option %d: %s ([[%s]]).", k.stamp(), n, changeText, c.Name))
	k.w.Put(path, setApply(text, "[["+c.Name+"]]"))
	return c, nil
}
