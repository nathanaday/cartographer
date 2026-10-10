// Package hooks serves the agent hooks of Specification 10.2: prompt (capture the user's
// words), before (give the rules of the entity a file belongs to), guard (keep agents off
// declared state), and stop (remind the agent to check what it changed).
package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/app"
	"github.com/nathanaday/cartographer/internal/change"
	"github.com/nathanaday/cartographer/internal/config"
	"github.com/nathanaday/cartographer/internal/consent"
	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Input is what Claude Code sends a hook on stdin.
type Input struct {
	SessionID      string         `json:"session_id"`
	Cwd            string         `json:"cwd"`
	HookEventName  string         `json:"hook_event_name"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
	Prompt         string         `json:"prompt"`
	StopHookActive bool           `json:"stop_hook_active"`
}

// Options configure the hooks for tests.
type Options struct {
	Now func() time.Time
	Sim string
}

// Run runs one hook. A hook never fails the agent's work: on an error of its own it writes
// nothing and exits 0.
func Run(name string, stdin io.Reader, stdout io.Writer, o Options) int {
	if o.Now == nil {
		o.Now = func() time.Time {
			t := time.Now()
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		}
	}
	var in Input
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return 0
	}
	var out any
	switch name {
	case "prompt":
		consent.Save(in.SessionID, in.Prompt, o.Now())
	case "before":
		out = before(in, o)
	case "guard":
		out = guard(in)
	case "stop":
		out = stop(in, o)
	default:
		fmt.Fprintf(os.Stderr, "carto: no hook %q\n", name)
		return 0
	}
	if out != nil {
		json.NewEncoder(stdout).Encode(out)
	}
	return 0
}

func addContext(event, text string) map[string]any {
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": event, "additionalContext": text}}
}

func deny(reason string) map[string]any {
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse",
		"permissionDecision": "deny", "permissionDecisionReason": reason}}
}

func filePath(in Input) string {
	for _, k := range []string{"file_path", "notebook_path", "path"} {
		if s, ok := in.ToolInput[k].(string); ok && s != "" {
			if !filepath.IsAbs(s) && in.Cwd != "" {
				s = filepath.Join(in.Cwd, s)
			}
			return s
		}
	}
	return ""
}

// ---------- session state ----------

type state struct {
	Started  time.Time            `json:"started"`
	Seen     map[string]bool      `json:"seen"`
	Touched  map[string][]string  `json:"touched"` // project|entity -> files
	First    map[string]time.Time `json:"first"`   // project|entity -> first touch
	Reminded bool                 `json:"reminded"`
}

func statePath(session string) string {
	return filepath.Join(config.Home(), "sessions", filepath.Base(session)+".json")
}

func load(session string, now time.Time) *state {
	s := &state{Started: now, Seen: map[string]bool{}, Touched: map[string][]string{}, First: map[string]time.Time{}}
	if b, err := os.ReadFile(statePath(session)); err == nil {
		json.Unmarshal(b, s)
	}
	return s
}

func (s *state) save(session string) {
	os.MkdirAll(filepath.Dir(statePath(session)), 0o700)
	b, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(statePath(session), b, 0o600)
}

// ---------- before ----------

func before(in Input, o Options) any {
	path := filePath(in)
	if path == "" || in.SessionID == "" {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	folder, remote, rel := app.ForPath(cfg, path)
	if folder == "" {
		return nil
	}
	p, _, err := app.Open(folder, o.Sim)
	if err != nil {
		return nil
	}
	now := o.Now()
	st := load(in.SessionID, now)
	defer st.save(in.SessionID)
	owner := app.Owner(p, remote, rel)
	key := p.Project + "|" + owner
	if owner == "" {
		key = p.Project + "|" + remote
	}
	if _, ok := st.First[key]; !ok {
		st.First[key] = now
	}
	st.Touched[key] = appendOnce(st.Touched[key], rel)
	if st.Seen[key] {
		return nil
	}
	st.Seen[key] = true
	if owner == "" {
		return addContext("PreToolUse", fmt.Sprintf("Cartographer: %s is in %s, a repository of the project %s, but no service or module owns this path, so no rule of the map covers it. If it is a new entry point or deploy unit, the coverage check will report it; you can propose an entity for it.", rel, remote, p.Project))
	}
	return addContext("PreToolUse", Rules(p, owner, rel))
}

func appendOnce(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}

// Rules is what an agent needs before it edits an entity: its active and pending
// obligations, open findings, and waivers.
func Rules(p *core.Project, e, rel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Cartographer: %s belongs to %s (%s) in the project %s. Keep these rules while you work here.\n", rel, e, p.Type(e), p.Project)
	obs := p.ActiveOf(e)
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].Clause.Key() < obs[j].Clause.Key() })
	if len(obs) > 0 {
		b.WriteString("\nObligations:\n")
		for _, o := range obs {
			fmt.Fprintf(&b, "- %s (%s): %s [%s]\n", o.Clause.ID, o.Clause.Owner, o.Clause.Statement, o.Status)
		}
	}
	if pend := p.PendingOf(e); len(pend) > 0 {
		b.WriteString("\nPending, for goals in progress:\n")
		for _, o := range pend {
			fmt.Fprintf(&b, "- %s (%s, goal %s): %s\n", o.Clause.ID, o.Clause.Owner, o.Goal, o.Clause.Statement)
		}
	}
	var fs []string
	for _, n := range p.Of("finding") {
		if f := p.Finding[n]; f.Entity == e && f.Status == "open" {
			fs = append(fs, n)
		}
	}
	if len(fs) > 0 {
		b.WriteString("\nOpen findings: " + strings.Join(fs, "; ") + "\n")
	}
	for _, w := range p.Waivers {
		if w.Entity == e {
			fmt.Fprintf(&b, "\nWaived: %s until %s: %s\n", w.Target, w.F["until"], w.Reason)
		}
	}
	b.WriteString("\nWhen you are done, check this entity (the carto-check skill) before you finish.")
	return b.String()
}

// ---------- guard ----------

var (
	writeCmd = regexp.MustCompile(`(>|\btee\b|\bsed\s+-i|\bmv\b|\bcp\b|\brm\b|\bpython3?\b|\bperl\b|\bruby\b|\bnode\b|\btruncate\b|\bdd\b|\bawk\b.*-i)`)
	userAct  = regexp.MustCompile(`\bcarto\s+(approve|reject|undo|act|answer|choose|defer|seen)\b`)
)

func guard(in Input) any {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	if in.ToolName == "Bash" {
		cmd, _ := in.ToolInput["command"].(string)
		if userAct.MatchString(cmd) {
			return deny("Cartographer: approve, reject, undo, act, answer, choose, defer, and seen are the user's acts. Use the approve, undo, or work tool, which check the user's own words, or ask the user to act in Obsidian.")
		}
		for _, p := range cfg.Projects {
			for _, root := range []string{p, filepath.Join(vault.Root(p), ".carto")} {
				if strings.Contains(cmd, root) && writeCmd.MatchString(cmd) {
					return deny(fmt.Sprintf("Cartographer: this command may write in %s. Its documents change only through the Cartographer tools (propose, record, work); read them with cat or the map tool.", root))
				}
			}
		}
		return nil
	}
	path := filePath(in)
	if path == "" {
		return nil
	}
	abs, _ := filepath.Abs(path)
	for _, p := range cfg.Projects {
		if strings.HasPrefix(abs, filepath.Join(vault.Root(p), ".carto")+string(filepath.Separator)) {
			return deny("Cartographer: .carto/ holds the runner's and the plugin's state. Do not edit it.")
		}
		if !strings.HasPrefix(abs, p+string(filepath.Separator)) {
			continue
		}
		if !strings.HasSuffix(abs, ".md") {
			if strings.HasSuffix(abs, ".canvas") {
				return deny("Cartographer: canvases are generated from the documents. Change the documents through the tools, and the canvas follows.")
			}
			return nil
		}
		old, _ := os.ReadFile(abs)
		next, ok := after(in, string(old))
		if !ok {
			return deny("Cartographer: this edit cannot be checked against the rules of the map; change Cartographer documents through the tools (propose, record, work).")
		}
		if why := Refuse(abs, string(old), next); why != "" {
			return deny("Cartographer: " + why)
		}
	}
	return nil
}

// after computes the text a Write, Edit, or MultiEdit would leave.
func after(in Input, old string) (string, bool) {
	switch in.ToolName {
	case "Write":
		s, ok := in.ToolInput["content"].(string)
		return s, ok
	case "Edit":
		return replace(old, in.ToolInput)
	case "MultiEdit":
		edits, _ := in.ToolInput["edits"].([]any)
		text := old
		for _, e := range edits {
			m, _ := e.(map[string]any)
			var ok bool
			if text, ok = replace(text, m); !ok {
				return "", false
			}
		}
		return text, true
	}
	return "", false
}

func replace(text string, m map[string]any) (string, bool) {
	from, _ := m["old_string"].(string)
	to, _ := m["new_string"].(string)
	if from == "" || !strings.Contains(text, from) {
		return "", false
	}
	if all, _ := m["replace_all"].(bool); all {
		return strings.ReplaceAll(text, from, to), true
	}
	return strings.Replace(text, from, to, 1), true
}

// computed are the tool-written fields of each type.
var computed = map[string][]string{
	"finding": {"status", "contract", "resolution", "resolved"},
	"goal":    {"affects", "progress", "tasks", "landed"},
	"service": {"compliance", "violated", "checked"},
	"module":  {"compliance", "violated", "checked"},
	"system":  {"compliance", "rollup", "violated"},
	"project": {"rollup", "violated"},
}

// Refuse returns why an agent may not make this edit of a Cartographer document, or "".
func Refuse(path, old, next string) string {
	if old == next {
		return ""
	}
	if old == "" {
		return "a new Cartographer document is declared state; create it with the propose tool (a create operation)."
	}
	a, err1 := md.Parse(path, old)
	b, err2 := md.Parse(path, next)
	if err1 != nil || err2 != nil {
		return "the edit breaks or starts from a frontmatter that is not valid YAML."
	}
	switch a.Type {
	case "check":
		return "a check is never changed after it is written."
	case "change":
		return "a change document changes only through the propose, approve, and undo tools."
	}
	for _, f := range change.DeclaredFields {
		if md.Str(a.FM[f]) != md.Str(b.FM[f]) || strings.Join(md.List(a.FM, f), ",") != strings.Join(md.List(b.FM, f), ",") {
			return fmt.Sprintf("`%s` is declared state; change it with the propose tool, and the user approves it.", f)
		}
	}
	for _, f := range computed[a.Type] {
		if md.Str(a.FM[f]) != md.Str(b.FM[f]) || strings.Join(md.List(a.FM, f), ",") != strings.Join(md.List(b.FM, f), ",") {
			return fmt.Sprintf("`%s` is written by the tool; it follows from the documents.", f)
		}
	}
	if a.Type != "finding" && md.Str(a.FM["status"]) != md.Str(b.FM["status"]) {
		switch a.Type {
		case "goal", "question":
			return "a goal's or question's status changes through the work tool (close, answer) or the user's approval."
		default:
			return "an entity's status is declared state; remove or restore it with the propose tool."
		}
	}
	for _, s := range change.DeclaredSections {
		if md.Section(a.Body, s) != md.Section(b.Body, s) {
			return fmt.Sprintf("the %s section is declared state; change it with the propose tool.", s)
		}
	}
	if a.Type == "goal" && md.Section(a.Body, "Tasks") != md.Section(b.Body, "Tasks") {
		return "tasks change through the work tool (done, drop, add, attempt, block)."
	}
	if (a.Type == "finding" || a.Type == "question") && (md.Section(a.Body, "Options") != md.Section(b.Body, "Options") || md.Section(a.Body, "Answer") != md.Section(b.Body, "Answer")) {
		return "options and answers are the user's; record them with the work tool (answer, choose)."
	}
	block := regexp.MustCompile("(?s)```cartographer-\\w+\n```\n(?:\n> \\[!example\\][^\n]*\n(?:>[^\n]*\n)*)?")
	if strings.Join(block.FindAllString(a.Body, -1), "") != strings.Join(block.FindAllString(b.Body, -1), "") {
		return "rendered blocks are filled by the plugin; leave them as they are."
	}
	return ""
}

// ---------- stop ----------

func stop(in Input, o Options) any {
	if in.StopHookActive || in.SessionID == "" {
		return nil
	}
	now := o.Now()
	st := load(in.SessionID, now)
	if st.Reminded || len(st.Touched) == 0 {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	var need []string
	opened := map[string]*core.Project{}
	for key := range st.Touched {
		project, entity, _ := strings.Cut(key, "|")
		if entity == "" || strings.Contains(entity, "/") {
			continue
		}
		var p *core.Project
		for _, folder := range cfg.Projects {
			if filepath.Base(folder) == project || strings.HasSuffix(folder, string(filepath.Separator)+project) {
				if opened[folder] == nil {
					opened[folder], _, _ = app.Open(folder, o.Sim)
				}
				p = opened[folder]
			}
		}
		if p == nil || p.Docs[entity] == nil || len(p.ActiveOf(entity))+len(p.PendingOf(entity)) == 0 {
			continue
		}
		checked := false
		for _, c := range p.Checks {
			if c.Target == entity && !c.Date.Before(st.First[key]) {
				checked = true
			}
		}
		if !checked {
			need = append(need, entity)
		}
	}
	if len(need) == 0 {
		return nil
	}
	sort.Strings(need)
	st.Reminded = true
	st.save(in.SessionID)
	return map[string]any{"decision": "block", "reason": fmt.Sprintf("Cartographer: this session changed files of %s, which have obligations, and no check of them was recorded since. Check them with the carto-check skill before you finish, or say in one line why a check is not needed.", strings.Join(need, ", "))}
}
