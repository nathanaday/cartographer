// Package mcpserver serves the agent tools of Specification 10.4 over MCP.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/app"
	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/consent"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/record"
	"github.com/nathanaday/cartographer/internal/refresh"
	"github.com/nathanaday/cartographer/internal/view"
	"github.com/nathanaday/cartographer/internal/work"
)

// Options configure a server.
type Options struct {
	Version string
	Sim     string           // a simulation file in place of the live environment
	Now     func() time.Time // the clock of writes
	Cwd     string           // the working folder of the agent session
}

// Server serves the tools.
type Server struct{ opts Options }

// New makes a server.
func New(o Options) *Server {
	if o.Now == nil {
		o.Now = func() time.Time {
			t := time.Now()
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		}
	}
	if o.Cwd == "" {
		o.Cwd, _ = os.Getwd()
	}
	return &Server{o}
}

func (s *Server) open(project string) (*core.Project, core.Env, string, error) {
	folder, err := app.Resolve(project, s.opts.Cwd)
	if err != nil {
		return nil, nil, "", err
	}
	p, env, err := app.Open(folder, s.opts.Sim)
	return p, env, folder, err
}

// openNew is open for the tools that work before the project document exists: the
// proposal that creates it, and its approval.
func (s *Server) openNew(project string) (core.Env, string, error) {
	p, env, folder, err := s.open(project)
	if p == nil && !errors.Is(err, core.ErrNoProject) {
		return nil, "", err
	}
	return env, folder, nil
}

// Text is a tool result for a person or an agent to read.
type Text struct {
	Text string `json:"text"`
}

type ProjectIn struct {
	Project string `json:"project,omitempty" jsonschema:"the project's folder or name; empty for the only configured project, or the project of the working folder"`
}

// ---------- read tools ----------

type MapIn struct {
	ProjectIn
	Entity string `json:"entity" jsonschema:"the name of a system, service, module, interface, or contract"`
}

func (s *Server) mapTool(_ context.Context, _ *mcp.CallToolRequest, in MapIn) (*mcp.CallToolResult, Text, error) {
	p, _, _, err := s.open(in.Project)
	if err != nil {
		return nil, Text{}, err
	}
	d, ok := p.Docs[in.Entity]
	if !ok {
		return nil, Text{}, fmt.Errorf("no document named %q in %s", in.Entity, p.Project)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s)\n\n%s\n\n", d.Name, d.Type, md.Str(d.FM["description"]))
	for _, k := range []string{"systems", "repository", "path", "entry", "interfaces", "uses", "stores", "adopts", "kind", "definition", "schemas", "external", "code"} {
		if vals := md.List(d.FM, k); len(vals) > 0 {
			fmt.Fprintf(&b, "- %s: %s\n", k, strings.Join(vals, ", "))
		}
	}
	for _, kind := range []string{"obligations", "adopters", "attached", "children", "work", "progress"} {
		if strings.Contains(d.Body, "```cartographer-"+kind) {
			lines, _ := view.Block(p, kind, d.Name)
			fmt.Fprintf(&b, "\n## %s\n\n%s\n", strings.ToUpper(kind[:1])+kind[1:], strings.Join(lines, "\n"))
		}
	}
	return nil, Text{b.String()}, nil
}

type ImpactIn struct {
	ProjectIn
	Target string `json:"target" jsonschema:"a contract, a clause as Contract#ID, an interface, or a module"`
}

type ImpactOut struct {
	Hits []core.Hit `json:"hits"`
}

func (s *Server) impactTool(_ context.Context, _ *mcp.CallToolRequest, in ImpactIn) (*mcp.CallToolResult, ImpactOut, error) {
	p, _, _, err := s.open(in.Project)
	if err != nil {
		return nil, ImpactOut{}, err
	}
	hits, err := p.Impact(in.Target)
	return nil, ImpactOut{hits}, err
}

func (s *Server) attentionTool(_ context.Context, _ *mcp.CallToolRequest, in ProjectIn) (*mcp.CallToolResult, Text, error) {
	p, _, _, err := s.open(in.Project)
	if err != nil {
		return nil, Text{}, err
	}
	var b strings.Builder
	for _, s := range [][2]string{{"since", "Since your last visit"}, {"attention", "Attention"}, {"goals", "Goals"}, {"health", "Health"}} {
		lines, _ := view.Block(p, s[0], p.Project)
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s[1], strings.Join(lines, "\n"))
	}
	return nil, Text{b.String()}, nil
}

type LintIn struct {
	ProjectIn
	Goal string `json:"goal,omitempty" jsonschema:"a draft goal: lint the map as the goal would leave it"`
}

func (s *Server) lintTool(_ context.Context, _ *mcp.CallToolRequest, in LintIn) (*mcp.CallToolResult, Text, error) {
	_, _, folder, err := s.open(in.Project)
	if err != nil {
		return nil, Text{}, err
	}
	p, err := lintProject(folder, in.Goal, s.opts.Sim, s.opts.Now())
	if err != nil {
		return nil, Text{}, err
	}
	var b strings.Builder
	for _, i := range p.Validation {
		fmt.Fprintf(&b, "- validation %s: %s\n", i.Rule, i.Message)
	}
	for _, i := range p.Lint {
		fmt.Fprintf(&b, "- lint %s: %s\n", i.Rule, i.Message)
	}
	if b.Len() == 0 {
		b.WriteString("No validation errors and no lint results.\n")
	}
	return nil, Text{b.String()}, nil
}

// ---------- declared state ----------

type ProposeIn struct {
	ProjectIn
	Title    string      `json:"title" jsonschema:"a short title of the change"`
	Ops      []change.Op `json:"ops" jsonschema:"the operations: create, add, remove, set, line, clause, goal.approve"`
	Because  string      `json:"because,omitempty" jsonschema:"an answered question or a finding with a chosen option: apply the change that option carries, as the user's act"`
	Replaces string      `json:"replaces,omitempty" jsonschema:"the id or name of an agent's proposed change that this one replaces: the tool withdraws it, so the user decides on one"`
	Session  string      `json:"session,omitempty"`
}

type ChangeOut struct {
	Change  string   `json:"change"`
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	Summary []string `json:"summary"`
}

func out(c *change.Doc) ChangeOut {
	var sum []string
	for _, a := range c.Acts {
		sum = append(sum, a.Summary...)
	}
	return ChangeOut{c.Name, c.ID, c.Status, sum}
}

func (s *Server) proposeTool(_ context.Context, _ *mcp.CallToolRequest, in ProposeIn) (*mcp.CallToolResult, ChangeOut, error) {
	env, folder, err := s.openNew(in.Project)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	if in.Because != "" {
		now := s.opts.Now()
		w := app.Writer(folder)
		defer w.Abort()
		c, err := work.ApplyChoice(folder, w, strings.Trim(in.Because, "[]"), in.Ops, now)
		if err != nil {
			return nil, ChangeOut{}, err
		}
		return nil, out(c), app.Commit(w, folder, env, c, now, "apply the choice of "+strings.Trim(in.Because, "[]"))
	}
	ix, err := change.NewIndex(folder)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	if change.DraftOnly(in.Ops) {
		now := s.opts.Now()
		w := app.Writer(folder)
		defer w.Abort()
		c, err := change.Draft(ix, w, in.Title, in.Ops, in.Session, now)
		if err != nil {
			return nil, ChangeOut{}, err
		}
		return nil, out(c), app.Commit(w, folder, env, c, now, "draft "+in.Title)
	}
	var c *change.Doc
	if in.Replaces != "" {
		c, err = change.Replace(ix, in.Replaces, in.Title, in.Ops, in.Session, s.opts.Now())
	} else {
		c, err = change.Propose(ix, in.Title, in.Ops, in.Session, s.opts.Now())
	}
	if err != nil {
		return nil, ChangeOut{}, err
	}
	return nil, out(c), nil
}

type ApproveIn struct {
	ProjectIn
	Change string `json:"change" jsonschema:"the change's id or name"`
	Reject bool   `json:"reject,omitempty" jsonschema:"reject instead of approve"`
}

func (s *Server) approveTool(_ context.Context, _ *mcp.CallToolRequest, in ApproveIn) (*mcp.CallToolResult, ChangeOut, error) {
	env, folder, err := s.openNew(in.Project)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	ix, err := change.NewIndex(folder)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	c, err := change.Find(ix, in.Change)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	now := s.opts.Now()
	verbs, act := []string{"approv", "accept"}, "approve"
	if in.Reject {
		verbs, act = []string{"reject", "decline"}, "reject"
	}
	said, ok := consent.Decides(now, verbs, consent.Change{ID: c.ID, Name: c.Name, Title: c.Title, Creates: c.Creates(), Proposed: c.ProposedAt()})
	if !ok {
		return nil, ChangeOut{}, fmt.Errorf("the user has not said to %s %q (%s) in a prompt in the last 30 minutes; ask the user to %s it in their own words, naming the change by its title or id, or to use Obsidian", act, c.Title, c.ID, act)
	}
	w := app.Writer(folder)
	defer w.Abort()
	verb := "approve "
	if in.Reject {
		verb = "reject "
		c, err = change.Reject(ix, w, c.ID, said.Text, now)
	} else {
		c, err = change.Approve(ix, w, c.ID, said.Text, now)
	}
	if err != nil {
		return nil, ChangeOut{}, err
	}
	return nil, out(c), app.Commit(w, folder, env, c, now, verb+c.Title)
}

func (s *Server) undoTool(_ context.Context, _ *mcp.CallToolRequest, in ProjectIn) (*mcp.CallToolResult, ChangeOut, error) {
	_, env, folder, err := s.open(in.Project)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	if _, ok := consent.Find(s.opts.Now(), "undo"); !ok {
		return nil, ChangeOut{}, errors.New("the user has not asked to undo in a prompt in the last 30 minutes")
	}
	ix, err := change.NewIndex(folder)
	if err != nil {
		return nil, ChangeOut{}, err
	}
	w := app.Writer(folder)
	defer w.Abort()
	c, err := change.Undo(ix, w, s.opts.Now())
	if err != nil {
		return nil, ChangeOut{}, err
	}
	return nil, out(c), app.Commit(w, folder, env, c, s.opts.Now(), "undo "+c.Title)
}

// ---------- observed state ----------

type RecordIn struct {
	ProjectIn
	Action   string            `json:"action" jsonschema:"check (write a check's report), start_sweep, or finish_sweep"`
	Report   *record.Report    `json:"report,omitempty"`
	Sweep    string            `json:"sweep,omitempty" jsonschema:"finish_sweep: the sweep's name"`
	Coverage []record.Coverage `json:"coverage,omitempty"`
	Notes    []string          `json:"notes,omitempty"`
}

type RecordOut struct {
	Check    string   `json:"check"`
	Opened   []string `json:"opened,omitempty"`
	Reopened []string `json:"reopened,omitempty"`
	Resolved []string `json:"resolved,omitempty"`
}

func (s *Server) recordTool(_ context.Context, _ *mcp.CallToolRequest, in RecordIn) (*mcp.CallToolResult, RecordOut, error) {
	_, env, folder, err := s.open(in.Project)
	if err != nil {
		return nil, RecordOut{}, err
	}
	now := s.opts.Now()
	switch in.Action {
	case "check":
		if in.Report == nil {
			return nil, RecordOut{}, errors.New("check needs a report")
		}
		o, err := record.Record(folder, env, *in.Report, now)
		if err != nil {
			return nil, RecordOut{}, err
		}
		return nil, RecordOut{o.Check, o.Opened, o.Reopened, o.Resolved}, nil
	case "start_sweep":
		name, err := record.StartSweep(folder, now)
		return nil, RecordOut{Check: name}, err
	case "finish_sweep":
		o, err := record.FinishSweep(folder, env, in.Sweep, in.Coverage, in.Notes, now)
		if err != nil {
			return nil, RecordOut{}, err
		}
		return nil, RecordOut{Check: o.Check, Opened: o.Opened}, nil
	}
	return nil, RecordOut{}, fmt.Errorf("no record action %q", in.Action)
}

type WorkIn struct {
	ProjectIn
	Action  string            `json:"action" jsonschema:"done, drop, add, attempt, block, log, ask, answer, choose, defer, close, or seen"`
	Goal    string            `json:"goal,omitempty"`
	Task    string            `json:"task,omitempty" jsonschema:"a task id such as T4"`
	Commit  string            `json:"commit,omitempty"`
	Reason  string            `json:"reason,omitempty"`
	Target  string            `json:"target,omitempty" jsonschema:"add: the entity the task runs on"`
	Text    string            `json:"text,omitempty" jsonschema:"add: the task; log: the line; ask: the question"`
	Fields  map[string]string `json:"fields,omitempty" jsonschema:"add: for, after, blocked, finding, question, by, due, every, user, repo"`
	Blocker string            `json:"blocker,omitempty"`
	Title   string            `json:"title,omitempty" jsonschema:"ask: the question, short, as its document name"`
	Context string            `json:"context,omitempty"`
	Options []work.Option     `json:"options,omitempty"`
	About   []string          `json:"about,omitempty"`
	Blocks  [][2]string       `json:"blocks,omitempty" jsonschema:"ask: [goal, task] pairs that wait on the question"`
	Doc     string            `json:"doc,omitempty" jsonschema:"answer, choose, defer: the question, finding, or goal"`
	Chosen  int               `json:"chosen,omitempty"`
	Words   string            `json:"words,omitempty" jsonschema:"answer, choose, defer: the user's own words, as they wrote them in a prompt"`
	Until   string            `json:"until,omitempty"`
	Session string            `json:"session,omitempty"`
}

type WorkOut struct {
	Done []string `json:"done"`
	ID   string   `json:"id,omitempty"`
}

func (s *Server) workTool(_ context.Context, _ *mcp.CallToolRequest, in WorkIn) (*mcp.CallToolResult, WorkOut, error) {
	p, env, folder, err := s.open(in.Project)
	if err != nil {
		return nil, WorkOut{}, err
	}
	now := s.opts.Now()
	if in.Action == "close" {
		return nil, WorkOut{}, app.Close(p, env, folder, in.Goal, now)
	}
	if in.Action == "seen" {
		if _, ok := consent.FindAny(now, "seen", "mark"); !ok {
			return nil, WorkOut{}, errors.New("the user has not asked to mark the dashboard seen")
		}
		type marker interface{ MarkSeen() error }
		if m, ok := env.(marker); ok {
			return nil, WorkOut{Done: []string{"marked seen"}}, m.MarkSeen()
		}
		return nil, WorkOut{}, errors.New("this environment keeps no Mark seen")
	}
	if in.Action == "answer" || in.Action == "choose" || in.Action == "defer" {
		if strings.TrimSpace(in.Words) == "" {
			return nil, WorkOut{}, errors.New("an answer, a choice, or a defer is the user's act; pass the user's own words")
		}
		if _, ok := consent.Find(now, in.Words); !ok {
			return nil, WorkOut{}, errors.New("those words are not in a prompt of the user in the last 30 minutes; quote the user exactly")
		}
	}
	k, err := work.New(folder, now)
	if err != nil {
		return nil, WorkOut{}, err
	}
	defer k.Writer().Abort()
	var id string
	switch in.Action {
	case "done":
		err = k.Done(in.Goal, in.Task, in.Commit)
	case "drop":
		err = k.Drop(in.Goal, in.Task, in.Reason)
	case "add":
		id, err = k.Add(in.Goal, in.Target, in.Text, in.Fields)
	case "attempt":
		err = k.Attempt(in.Goal, in.Task, in.Reason)
	case "block":
		err = k.Block(in.Goal, in.Task, in.Blocker)
	case "log":
		err = k.Log(in.Goal, in.Text)
	case "ask":
		id, err = k.Ask(in.Title, in.Text, in.Context, in.Options, in.About, in.Blocks, in.Session)
	case "answer":
		err = k.Answer(in.Doc, in.Chosen, in.Words)
	case "choose":
		err = k.Choose(in.Doc, in.Chosen, in.Words)
	case "defer":
		err = k.Defer(in.Doc, in.Until)
	default:
		err = fmt.Errorf("no work action %q", in.Action)
	}
	if err != nil {
		return nil, WorkOut{}, err
	}
	if err := k.Writer().Flush(); err != nil {
		return nil, WorkOut{}, err
	}
	p2, err := core.Load(folder, env)
	if err != nil {
		return nil, WorkOut{}, err
	}
	if _, err := refresh.Stage(p2, k.Writer(), false); err != nil {
		return nil, WorkOut{}, err
	}
	if _, err := k.Writer().Commit(in.Action + ": " + strings.Join(k.Summary(), "; ")); err != nil {
		return nil, WorkOut{}, err
	}
	return nil, WorkOut{Done: k.Summary(), ID: id}, nil
}

func readOnly() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{ReadOnlyHint: true} }

// MCP builds the protocol server with every tool.
func (s *Server) MCP() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "carto", Title: "Cartographer", Version: s.opts.Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "map", Annotations: readOnly(),
		Description: "One entity of the map (system, service, module, interface, contract): its relations, its obligations with status and evidence, its pending obligations, open findings, lint, and open work."}, safe("map", s.mapTool))
	mcp.AddTool(server, &mcp.Tool{Name: "impact", Annotations: readOnly(),
		Description: "Every entity that a contract, a clause (Contract#ID), an interface, or a module reaches, with why. Use it to scope a change before you plan it."}, safe("impact", s.impactTool))
	mcp.AddTool(server, &mcp.Tool{Name: "attention", Annotations: readOnly(),
		Description: "The project dashboard as text: Since your last visit, Attention (the decisions that wait for the user), Goals (progress, ready tasks, agents), and Health (each problem once)."}, safe("attention", s.attentionTool))
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: readOnly(),
		Description: "Validation and lint of the map; with goal, of the map as that draft goal would leave it. Run it on every draft goal before the user sees it."}, safe("lint", s.lintTool))
	mcp.AddTool(server, &mcp.Tool{Name: "propose",
		Description: "The only way an agent changes declared state (entities, relations, clauses, waivers, goals). It checks the operations and writes a proposed change for the user to approve. Operations that only create draft goals apply at once: the draft is the proposal, and the user approves the draft. With because, it applies the change of the option the user chose in a question or a finding, at once and once, as the user's act."}, safe("propose", s.proposeTool))
	mcp.AddTool(server, &mcp.Tool{Name: "approve",
		Description: "Apply (or, with reject, reject) a proposed change after the user approved it in their own words in this or any session, naming the change's id or name. The tool checks the user's recent prompts and accepts nothing else."}, safe("approve", s.approveTool))
	mcp.AddTool(server, &mcp.Tool{Name: "undo",
		Description: "Reverse the user's last applied act, when the user asked to undo and nothing has been built on it since."}, safe("undo", s.undoTool))
	mcp.AddTool(server, &mcp.Tool{Name: "record",
		Description: "Write a checker's report: a check document (or a section of a running sweep) with one row per obligation, and the findings it opens, reopens, or resolves. start_sweep and finish_sweep (with coverage) bracket a sweep. Refuses rows that break the format rules."}, safe("record", s.recordTool))
	mcp.AddTool(server, &mcp.Tool{Name: "work",
		Description: "Observed work: done, drop (with a reason), add, attempt (a failure), block, and log for tasks of goals and Upkeep; ask writes a question for the user; answer, choose, and defer record the user's own words from a prompt; close closes a goal whose obligations are met, tasks closed, and branches landed; seen marks the dashboard seen."}, safe("work", s.workTool))
	return server
}

// Serve runs the server over stdio until the client goes.
func (s *Server) Serve(ctx context.Context) error {
	err := s.MCP().Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, context.Canceled) || (err != nil && strings.Contains(err.Error(), "EOF")) {
		return nil
	}
	return err
}

// safe turns a panic in a tool handler into the call's error, so one bad call does not end
// the server for the rest of the session.
func safe[In, Out any](tool string, h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		defer func() {
			if p := recover(); p != nil {
				var zero Out
				res, out, err = nil, zero, fmt.Errorf("%s failed inside carto: %v; the server keeps running, and this is a bug", tool, p)
			}
		}()
		return h(ctx, req, in)
	}
}
