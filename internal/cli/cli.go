// Package cli is the carto command line: the commands that the Obsidian plugin, the
// hooks, the MCP host, CI, and a person run.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/app"
	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/gesture"
	"github.com/nathanaday/cartographer/internal/hooks"
	"github.com/nathanaday/cartographer/internal/land"
	"github.com/nathanaday/cartographer/internal/live"
	"github.com/nathanaday/cartographer/internal/mcpserver"
	"github.com/nathanaday/cartographer/internal/record"
	"github.com/nathanaday/cartographer/internal/refresh"
	"github.com/nathanaday/cartographer/internal/runner"
	"github.com/nathanaday/cartographer/internal/templates"
	"github.com/nathanaday/cartographer/internal/vault"
	"github.com/nathanaday/cartographer/internal/view"
	"github.com/nathanaday/cartographer/internal/work"
)

// Version is set at build time.
var Version = "dev"

const usage = `carto: Cartographer, a map of a software ecosystem in an Obsidian vault.

Setup
  carto setup --project FOLDER [--root DIR]...   record a project and the folders to scan for clones
  carto version

Read
  carto status [--project P]              the dashboard: since your last visit, attention, goals, health
  carto block KIND DOCUMENT [--project P]  one rendered block (obligations, work, progress, ...)
  carto impact TARGET [--project P]        what a contract, clause, interface, or module reaches
  carto check [--project P]                validation and lint; exit 1 on a validation error

Write
  carto render [--project P] [--examples]  write the tool fields and the canvases
  carto record --report FILE [--project P] write a check from a report (CI, or by hand)
  carto record --inbox [--project P]       record the reports that CI left in .carto/inbox/
  carto propose --title T --ops FILE       propose declared operations for the user to approve (FILE "-" reads stdin);
                                           --replaces C withdraws an agent's proposed change C

The user's own acts (the Obsidian plugin runs these; the guard refuses them to agents)
  carto approve CHANGE|GOAL | reject CHANGE|GOAL | undo   (a draft goal: approve starts it, reject drops it)
  carto act --title TITLE --ops FILE       apply operations at once (FILE "-" reads stdin)
  carto new KIND NAME --description D      make a system (--code), service (--repository --path [--system S]...),
                                           module (--repository --path), interface, or contract (--code)
  carto answer QUESTION N [--words W]     choose option N of a question
  carto choose FINDING... N [--words W]   choose option N of one or more findings
  carto defer DOCUMENT|GOAL#TN... DATE     hide goals, questions, findings, or tasks' lines until DATE
  carto retry GOAL#TN --words HINT         clear a stuck task's attempts, with a hint for the agent
  carto take GOAL#TN                       do a task yourself: the runner no longer starts it
  carto drop GOAL#TN --words REASON        drop a task
  carto request GOAL[#TN] --words W [--on ENTITY]   ask for changes before a landing
  carto lint RULE ENTITY N                 decide on a lint result: 1 fix it on the map, 2 waive
  carto gesture CANVAS                     read an edit of a canvas as one act, and lay the canvas out again
  carto seen                               mark the dashboard seen
  carto snapshot                           commit the edits of documents made outside an act
                                           (the plugin runs it after a pause, the Stop hook after a turn)
  carto close GOAL                         close a goal whose work is met, done, and landed
  carto land GOAL | Upkeep#TN              merge the pull requests of a goal or an upkeep task

The runner
  carto run [--project P] [--once]         start agent sessions for the ready work until only
                                           work that waits on you is left

Agents
  carto mcp                                the MCP server of the agent tools
  carto hook prompt|before|guard|stop      the agent hooks (stdin: the hook's JSON)

Every command takes --project (a folder or a configured name), --sim FILE (a simulation in
place of git, the claims, and the clock), and --json where it prints.
`

type opts struct {
	project, sim, title, ops, report, words, on   string
	description, code, repository, path, replaces string
	examples, inbox, asJSON, once                 bool
	roots, systems                                multi
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(s string) error { *m = append(*m, s); return nil }

func parse(name string, args []string) (*opts, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := &opts{}
	fs.StringVar(&o.project, "project", "", "")
	fs.StringVar(&o.sim, "sim", "", "")
	fs.StringVar(&o.title, "title", "", "")
	fs.StringVar(&o.ops, "ops", "", "")
	fs.StringVar(&o.replaces, "replaces", "", "")
	fs.StringVar(&o.report, "report", "", "")
	fs.StringVar(&o.words, "words", "", "")
	fs.StringVar(&o.on, "on", "", "")
	fs.BoolVar(&o.examples, "examples", false, "")
	fs.BoolVar(&o.inbox, "inbox", false, "")
	fs.BoolVar(&o.asJSON, "json", false, "")
	fs.BoolVar(&o.once, "once", false, "")
	fs.Var(&o.roots, "root", "")
	fs.Var(&o.systems, "system", "")
	fs.StringVar(&o.description, "description", "", "")
	fs.StringVar(&o.code, "code", "", "")
	fs.StringVar(&o.repository, "repository", "", "")
	fs.StringVar(&o.path, "path", "", "")
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	return o, pos, nil
}

func now() time.Time { return live.Wall() }

var errExit1 = errors.New("exit 1")

// Run runs one command and returns the exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, rest := args[0], args[1:]
	if cmd == "hook" {
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "carto: hook takes one name")
			return 0
		}
		return hooks.Run(rest[0], stdin, stdout, hooks.Options{})
	}
	o, pos, err := parse(cmd, rest)
	if err == nil {
		err = dispatch(cmd, o, pos, stdin, stdout)
	}
	if errors.Is(err, errExit1) {
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "carto:", err)
		return 1
	}
	return 0
}

func want(pos []string, n int, name string) error {
	if len(pos) != n {
		return fmt.Errorf("%s takes %d arguments; run carto help", name, n)
	}
	return nil
}

type session struct {
	o      *opts
	pos    []string
	cwd    string
	stdin  io.Reader
	stdout io.Writer
}

func (s *session) folder(cmd string) (string, error) {
	key := s.o.project
	if key == "" && len(s.pos) > 0 && (cmd == "render" || cmd == "check" || cmd == "status") {
		key = s.pos[0]
	}
	return app.Resolve(key, s.cwd)
}

func (s *session) open(cmd string) (*core.Project, core.Env, string, error) {
	f, err := s.folder(cmd)
	if err != nil {
		return nil, nil, "", err
	}
	p, env, err := app.Open(f, s.o.sim)
	return p, env, f, err
}

func (s *session) print(v any, text string) {
	if s.o.asJSON {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(s.stdout, string(b))
		return
	}
	fmt.Fprintln(s.stdout, text)
}

func dispatch(cmd string, o *opts, pos []string, stdin io.Reader, stdout io.Writer) error {
	cwd, _ := os.Getwd()
	s := &session{o, pos, cwd, stdin, stdout}
	switch cmd {
	case "version":
		fmt.Fprintln(stdout, Version)
		return nil
	case "setup":
		return setup(o, stdout)
	case "mcp":
		return mcpserver.New(mcpserver.Options{Version: Version, Sim: o.sim, Cwd: cwd}).Serve(context.Background())
	case "status":
		return s.status()
	case "block":
		return s.block()
	case "impact":
		return s.impact()
	case "check":
		return s.check()
	case "render":
		p, _, _, err := s.open(cmd)
		if err != nil {
			return err
		}
		r, err := refresh.Run(p, o.examples, "render")
		if err != nil {
			return err
		}
		s.print(r, fmt.Sprintf("%d documents updated, %d canvases written", r.Documents, r.Canvases))
		return nil
	case "record":
		return s.record()
	case "approve", "reject", "undo", "act", "new", "answer", "choose", "defer", "seen", "close", "retry", "take", "drop", "request", "lint":
		return s.userAct(cmd)
	case "land":
		return s.land()
	case "gesture":
		return s.gesture()
	case "propose":
		return s.propose()
	case "snapshot":
		f, err := s.folder(cmd)
		if err != nil {
			return err
		}
		n, err := vault.Snapshot(vault.Root(f), f)
		if err != nil {
			return err
		}
		msg := fmt.Sprintf("%d files committed", n)
		if n == 1 {
			msg = "1 file committed"
		}
		s.print(map[string]int{"files": n}, msg)
		return nil
	case "run":
		return s.run()
	}
	return fmt.Errorf("no command %q; run carto help", cmd)
}

func (s *session) status() error {
	p, _, _, err := s.open("status")
	if err != nil {
		return err
	}
	var b strings.Builder
	out := map[string]string{}
	for _, x := range [][2]string{{"since", "Since your last visit"}, {"attention", "Attention"}, {"goals", "Goals"}, {"health", "Health"}} {
		lines, _ := view.Block(p, x[0], p.Project)
		out[x[0]] = strings.Join(lines, "\n")
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", x[1], out[x[0]])
	}
	s.print(out, strings.TrimRight(b.String(), "\n"))
	return nil
}

func (s *session) block() error {
	if err := want(s.pos, 2, "block"); err != nil {
		return err
	}
	p, _, _, err := s.open("block")
	if err != nil {
		return err
	}
	lines, err := view.Block(p, s.pos[0], s.pos[1])
	if err != nil {
		return err
	}
	md := strings.Join(lines, "\n")
	out := map[string]any{"kind": s.pos[0], "document": s.pos[1], "markdown": md}
	if s.pos[0] == "attention" {
		items, deferred := view.AttentionItems(p)
		out["items"], out["deferred"] = orEmpty(items), orEmpty(deferred)
	}
	s.print(out, md)
	return nil
}

func (s *session) impact() error {
	if err := want(s.pos, 1, "impact"); err != nil {
		return err
	}
	p, _, _, err := s.open("impact")
	if err != nil {
		return err
	}
	hits, err := p.Impact(s.pos[0])
	if err != nil {
		return err
	}
	var lines []string
	for _, h := range hits {
		lines = append(lines, h.Entity+": "+h.Why)
	}
	s.print(hits, strings.Join(lines, "\n"))
	return nil
}

var linkMarks = regexp.MustCompile(`\[\[|\]\]`)

func (s *session) check() error {
	p, _, _, err := s.open("check")
	if err != nil {
		return err
	}
	var lines []string
	for _, i := range p.Validation {
		lines = append(lines, fmt.Sprintf("validation %s: %s", i.Rule, i.Message))
	}
	for _, i := range p.Lint {
		lines = append(lines, fmt.Sprintf("lint %s: %s", i.Rule, linkMarks.ReplaceAllString(i.Message, "")))
	}
	s.print(map[string]any{"validation": p.Validation, "lint": p.Lint}, strings.Join(lines, "\n"))
	if len(p.Validation) > 0 {
		return errExit1
	}
	return nil
}

func setup(o *opts, stdout io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if o.project != "" {
		abs, _ := filepath.Abs(o.project)
		if _, err := os.Stat(abs); err != nil {
			return err
		}
		if len(app.Repositories(abs)) == 0 {
			return fmt.Errorf("%s holds no project document with repositories", abs)
		}
		cfg.AddProject(abs)
	}
	for _, r := range o.roots {
		cfg.AddRoot(r)
	}
	n := cfg.Scan()
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d projects, %d roots, %d clones (%d new). Settings: %s\n", len(cfg.Projects), len(cfg.Roots), len(cfg.Repos), n, filepath.Join(config.Home(), "config.json"))
	for _, p := range cfg.Projects {
		var missing []string
		for _, r := range app.Repositories(p) {
			if cfg.Clone(r) == "" {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			fmt.Fprintf(stdout, "%s: no clone found for %s; add a --root that holds them.\n", filepath.Base(p), strings.Join(missing, ", "))
		}
	}
	return nil
}

func (s *session) record() error {
	f, err := s.folder("record")
	if err != nil {
		return err
	}
	var files []string
	switch {
	case s.o.report != "":
		files = []string{s.o.report}
	case s.o.inbox:
		files, _ = filepath.Glob(filepath.Join(vault.Root(f), ".carto", "inbox", "*.json"))
		sort.Strings(files)
	default:
		return errors.New("record needs --report FILE or --inbox")
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var r record.Report
		if err := json.Unmarshal(b, &r); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		if r.By == "" {
			r.By = "ci"
		}
		_, env, err := app.Open(f, s.o.sim)
		if err != nil {
			return err
		}
		out, err := record.Record(f, env, r, now())
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		if s.o.inbox {
			os.Remove(file)
		}
		fmt.Fprintf(s.stdout, "%s: opened %d, reopened %d, resolved %d\n", out.Check, len(out.Opened), len(out.Reopened), len(out.Resolved))
	}
	return nil
}

func (s *session) userAct(cmd string) error {
	p, env, folder, err := s.open(cmd)
	if err != nil && !(errors.Is(err, core.ErrNoProject) && (cmd == "approve" || cmd == "reject")) {
		return err
	}
	t := now()
	ix, err := change.NewIndex(folder)
	if err != nil {
		return err
	}
	w := app.Writer(folder)
	defer w.Abort()
	report := func(c *change.Doc, msg string) error {
		if err := app.Commit(w, folder, env, c, t, msg); err != nil {
			return err
		}
		var sum []string
		for _, a := range c.Acts {
			sum = append(sum, a.Summary...)
		}
		s.print(map[string]any{"change": c.Name, "id": c.ID, "status": c.Status, "summary": sum}, c.Name+": "+c.Status)
		return nil
	}
	switch cmd {
	case "approve", "reject":
		if err := want(s.pos, 1, cmd); err != nil {
			return err
		}
		words := s.o.words
		if words == "" {
			words = cmd + "d in Obsidian"
		}
		var c *change.Doc
		if ix.Types[s.pos[0]] == "goal" {
			op := map[string]string{"approve": "goal.approve", "reject": "goal.drop"}[cmd]
			if c, err = change.UserAct(ix, w, cmd+" the goal "+s.pos[0], []change.Op{{Op: op, Doc: s.pos[0]}}, t); err != nil {
				return err
			}
			return report(c, cmd+" the goal "+s.pos[0])
		} else if cmd == "approve" {
			c, err = change.Approve(ix, w, s.pos[0], words, t)
		} else {
			c, err = change.Reject(ix, w, s.pos[0], words, t)
		}
		if err != nil {
			return err
		}
		return report(c, cmd+" "+c.Title)
	case "undo":
		c, err := change.Undo(ix, w, t)
		if err != nil {
			return err
		}
		return report(c, "undo "+c.Title)
	case "act":
		ops, err := s.readOps()
		if err != nil {
			return err
		}
		c, err := change.UserAct(ix, w, s.o.title, ops, t)
		if err != nil {
			return err
		}
		return report(c, s.o.title)
	case "new":
		if err := want(s.pos, 2, "new"); err != nil {
			return err
		}
		text, rel, err := templates.Text(s.pos[0], s.pos[1], templates.Fields{Description: s.o.description, Project: p.Project, Code: s.o.code,
			Systems: s.o.systems, Repository: s.o.repository, Path: s.o.path}, t)
		if err != nil {
			return err
		}
		title := "new " + s.pos[0] + " " + s.pos[1]
		c, err := change.UserAct(ix, w, title, []change.Op{{Op: "create", Path: rel, Text: text}}, t)
		if err != nil {
			return err
		}
		return report(c, title)
	case "seen":
		m, ok := env.(interface{ MarkSeen() error })
		if !ok {
			return errors.New("this environment keeps no Mark seen")
		}
		if err := m.MarkSeen(); err != nil {
			return err
		}
		fmt.Fprintln(s.stdout, "marked seen")
		return nil
	case "close":
		if err := want(s.pos, 1, "close"); err != nil {
			return err
		}
		if err := app.Close(p, env, folder, s.pos[0], t); err != nil {
			return err
		}
		fmt.Fprintf(s.stdout, "%s: done\n", s.pos[0])
		return nil
	}
	k, err := work.New(folder, t)
	if err != nil {
		return err
	}
	defer k.Writer().Abort()
	switch cmd {
	case "answer", "choose":
		if len(s.pos) < 2 || cmd == "answer" && len(s.pos) != 2 {
			return fmt.Errorf("%s takes a document and an option number; run carto help", cmd)
		}
		last := s.pos[len(s.pos)-1]
		n, err := strconv.Atoi(last)
		if err != nil {
			return fmt.Errorf("%q is not an option number", last)
		}
		for _, doc := range s.pos[:len(s.pos)-1] {
			if cmd == "answer" {
				err = k.Answer(doc, n, s.o.words)
			} else {
				err = k.Choose(doc, n, s.o.words)
			}
			if err != nil {
				return err
			}
		}
	case "defer":
		if len(s.pos) < 2 {
			return errors.New("defer takes documents and a date; run carto help")
		}
		until := s.pos[len(s.pos)-1]
		for _, doc := range s.pos[:len(s.pos)-1] {
			if strings.Contains(doc, "#") {
				err = k.DeferTask(doc, until)
			} else {
				err = k.Defer(doc, until)
			}
			if err != nil {
				return err
			}
		}
	case "retry", "take", "drop", "request":
		if err := want(s.pos, 1, cmd); err != nil {
			return err
		}
		switch cmd {
		case "retry":
			err = k.Retry(s.pos[0], s.o.words)
		case "take":
			err = k.Take(s.pos[0])
		case "drop":
			goal, id, serr := work.SplitTask(s.pos[0])
			if serr != nil {
				return serr
			}
			err = k.Drop(goal, id, s.o.words)
		case "request":
			err = k.Request(s.pos[0], s.o.words, s.o.on)
		}
		if err != nil {
			return err
		}
	case "lint":
		if err := want(s.pos, 3, "lint"); err != nil {
			return err
		}
		n, err := strconv.Atoi(s.pos[2])
		if err != nil {
			return fmt.Errorf("%q is not an option number", s.pos[2])
		}
		var message string
		for _, i := range p.Lint {
			if i.Rule == s.pos[0] && i.Entity == s.pos[1] {
				message = linkMarks.ReplaceAllString(i.Message, "") + "."
			}
		}
		if message == "" {
			return fmt.Errorf("no lint %s on %s", s.pos[0], s.pos[1])
		}
		if _, err := k.DecideLint(s.pos[0], s.pos[1], message, n); err != nil {
			return err
		}
	}
	if err := app.Commit(k.Writer(), folder, env, nil, t, cmd+": "+strings.Join(k.Summary(), "; ")); err != nil {
		return err
	}
	fmt.Fprintln(s.stdout, strings.Join(k.Summary(), "\n"))
	return nil
}

type cloner interface{ Clone(repo string) string }

func (s *session) land() error {
	if err := want(s.pos, 1, "land"); err != nil {
		return err
	}
	p, env, folder, err := s.open("land")
	if err != nil {
		return err
	}
	c, ok := env.(cloner)
	if !ok {
		return errors.New("land needs real clones; it does not run with --sim")
	}
	landed, err := land.Land(folder, p, s.pos[0], c.Clone, land.System, now())
	if len(landed) > 0 {
		fmt.Fprintf(s.stdout, "landed: %s\n", strings.Join(landed, ", "))
		if p2, lerr := core.Load(folder, env); lerr == nil {
			refresh.Run(p2, false, "refresh after land")
		}
	}
	return err
}

func (s *session) run() error {
	folder, err := s.folder("run")
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	p, env, err := app.Open(folder, s.o.sim)
	if err != nil {
		return err
	}
	c, ok := env.(cloner)
	if !ok {
		return errors.New("run needs real clones; it does not run with --sim")
	}
	max, _ := p.FM(p.Project, "max_agents").(int)
	budget, _ := p.FM(p.Project, "task_budget").(int)
	if budget <= 0 {
		budget = 120
	}
	load := func() (*core.Project, core.Env, error) { return app.Open(folder, s.o.sim) }
	r := &runner.Runner{
		Folder: folder,
		Load: func() (*core.Project, error) {
			p, _, err := load()
			return p, err
		},
		Close: func(goal string) error {
			p, env, err := load()
			if err != nil {
				return err
			}
			return app.Close(p, env, folder, goal, now())
		},
		Clone:  c.Clone,
		Agent:  cfg.Agent,
		Now:    now,
		Out:    s.stdout,
		Home:   config.Home(),
		Max:    max,
		Budget: time.Duration(budget) * time.Minute,
		Dirs:   []string{vault.Root(folder)},
	}
	for _, repo := range app.Repositories(folder) {
		if dir := c.Clone(repo); dir != "" {
			r.Dirs = append(r.Dirs, dir)
		}
	}
	return r.Run(context.Background(), s.o.once)
}

func orEmpty(items []view.Item) []view.Item {
	if items == nil {
		return []view.Item{}
	}
	return items
}

// gesture reads the user's edit of a canvas. On the project canvas the relations it
// changed become one act; on any canvas the layout is written again.
func (s *session) gesture() error {
	if err := want(s.pos, 1, "gesture"); err != nil {
		return err
	}
	p, env, folder, err := s.open("gesture")
	if err != nil {
		return err
	}
	path, _ := filepath.Abs(s.pos[0])
	edited, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	t := now()
	w := app.Writer(folder)
	defer w.Abort()
	res := gesture.Result{Ops: []change.Op{}, Notes: []string{}, Offers: []gesture.Offer{}}
	if path == filepath.Join(p.Folder, p.Project+".canvas") {
		if res, err = gesture.Read(p, vault.Root(folder), edited); err != nil {
			return err
		}
	} else {
		res.Notes = append(res.Notes, "This canvas is a view; Cartographer writes it again from the map.")
	}
	out := map[string]any{"change": "", "summary": []string{}, "notes": res.Notes, "offers": res.Offers}
	var c *change.Doc
	msg := "lay out " + filepath.Base(path)
	if len(res.Ops) > 0 {
		ix, err := change.NewIndex(folder)
		if err != nil {
			return err
		}
		title := gesture.Title(res.Ops)
		if c, err = change.UserAct(ix, w, title, res.Ops, t); err != nil {
			return err
		}
		out["change"], out["summary"], msg = c.Name, c.Acts[len(c.Acts)-1].Summary, title
	}
	if err := app.Commit(w, folder, env, c, t, msg); err != nil {
		return err
	}
	var lines []string
	if sum, _ := out["summary"].([]string); len(sum) > 0 {
		lines = append(lines, sum...)
	}
	s.print(out, strings.Join(append(lines, res.Notes...), "\n"))
	return nil
}

func (s *session) readOps() ([]change.Op, error) {
	if s.o.title == "" || s.o.ops == "" {
		return nil, errors.New("this command needs --title and --ops FILE (or - for stdin)")
	}
	var b []byte
	var err error
	if s.o.ops == "-" {
		b, err = io.ReadAll(s.stdin)
	} else {
		b, err = os.ReadFile(s.o.ops)
	}
	if err != nil {
		return nil, err
	}
	var ops []change.Op
	if err := json.Unmarshal(b, &ops); err != nil {
		return nil, fmt.Errorf("--ops: %w", err)
	}
	return ops, nil
}

func (s *session) propose() error {
	folder, err := s.folder("propose")
	if err != nil {
		return err
	}
	ops, err := s.readOps()
	if err != nil {
		return err
	}
	ix, err := change.NewIndex(folder)
	if err != nil {
		return err
	}
	var c *change.Doc
	if change.DraftOnly(ops) {
		_, env, _, err := s.open("propose")
		if err != nil {
			return err
		}
		t := now()
		w := app.Writer(folder)
		defer w.Abort()
		if c, err = change.Draft(ix, w, s.o.title, ops, "", t); err != nil {
			return err
		}
		if err := app.Commit(w, folder, env, c, t, "draft "+s.o.title); err != nil {
			return err
		}
	} else if s.o.replaces != "" {
		if c, err = change.Replace(ix, s.o.replaces, s.o.title, ops, "", now()); err != nil {
			return err
		}
	} else if c, err = change.Propose(ix, s.o.title, ops, "", now()); err != nil {
		return err
	}
	s.print(map[string]any{"change": c.Name, "id": c.ID, "status": c.Status, "summary": c.Acts[0].Summary}, c.Name+": "+c.Status)
	return nil
}
