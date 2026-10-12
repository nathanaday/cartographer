// Package runner is `carto run` (Specification 6): it takes the work that is ready and
// starts headless agent sessions for it, each in its own worktree, with a claim, a session
// log, and a time budget. It records failed attempts, holds a lease so that one machine
// runs it at a time, and runs until only work that waits on the user is left.
package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nathanaday/cartographer/internal/core"
	"github.com/nathanaday/cartographer/internal/land"
	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
	"github.com/nathanaday/cartographer/internal/work"
)

// DefaultAgent is the command of a headless Claude Code session. It edits and runs
// commands without prompts only for the tools it lists.
// {dirs} becomes one --add-dir for each folder of Runner.Dirs.
var DefaultAgent = []string{"claude", "-p", "{prompt}", "--output-format", "json", "--session-id", "{session}",
	"--permission-mode", "acceptEdits",
	"--allowedTools", "Bash,Read,Edit,Write,Grep,Glob,Task,mcp__plugin_cartographer_carto", "{dirs}"}

// Runner runs the work of one project.
type Runner struct {
	Folder string
	Load   func() (*core.Project, error) // loads the project with its environment
	Close  func(goal string) error       // closes a goal whose work is met, done, and landed
	Clone  func(repo string) string      // the local clone of a repository by short name
	Agent  []string                      // the agent command
	Now    func() time.Time
	Out    io.Writer
	Home   string        // the machine folder: worktrees live here
	Budget time.Duration // a task's time budget
	Max    int           // agents at once
	Lease  time.Duration // renewed each third of it; each renewal is a commit
	Dirs   []string      // folders an agent works in besides its own: the vault and the clones
	host   string
	mu     sync.Mutex
}

// Job is one agent session the runner starts.
type Job struct {
	Kind    string // task, land, sweep
	Goal    string
	Task    *core.Task
	Repo    string
	Branch  string
	Dir     string
	Session string
	Log     string // the session log document's name
	Policy  string // land: the project's landing policy
	Local   bool   // land: the project merges in the clones, with no pull requests
}

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (r *Runner) root() string { return vault.Root(r.Folder) }

func (r *Runner) say(format string, args ...any) {
	if r.Out != nil {
		fmt.Fprintf(r.Out, format+"\n", args...)
	}
}

// ---------- lease ----------

type lease struct {
	Host  string `json:"host"`
	PID   int    `json:"pid"`
	Until string `json:"until"`
}

func (r *Runner) leasePath() string { return filepath.Join(r.root(), ".carto", "runner.json") }

func (r *Runner) take() error {
	r.host, _ = os.Hostname()
	if b, err := os.ReadFile(r.leasePath()); err == nil {
		var l lease
		if json.Unmarshal(b, &l) == nil && md.Time(l.Until).After(r.Now()) && !(l.Host == r.host && l.PID == os.Getpid()) {
			if l.Host != r.host || processAlive(l.PID) {
				return fmt.Errorf("the runner runs on %s (pid %d) until %s", l.Host, l.PID, l.Until)
			}
		}
	}
	return r.renew()
}

func (r *Runner) renew() error {
	b, _ := json.MarshalIndent(lease{r.host, os.Getpid(), r.Now().Add(r.Lease).Format("2006-01-02T15:04:05")}, "", "  ")
	return r.write(r.leasePath(), b, "the runner's lease")
}

func (r *Runner) release() { r.write(r.leasePath(), nil, "the runner ended") }

// write writes or, with nil, removes one file of .carto, and commits it: claims and the
// lease are versioned, so that every machine sees them (Specification 6.2).
func (r *Runner) write(path string, b []byte, message string) error {
	w := vault.NewWriter(r.root())
	defer w.Abort()
	if b == nil {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		w.Remove(path)
	} else {
		w.Put(path, string(b)+"\n")
	}
	_, err := w.Commit(message)
	return err
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(sysSignal0)
	return err == nil || errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "operation not permitted")
}

// ---------- claims ----------

func (r *Runner) claimPath(session string) string {
	return filepath.Join(r.root(), ".carto", "claims", session+".json")
}

func (r *Runner) claim(j *Job) error {
	c := core.Claim{Goal: j.Goal, Session: j.Session, Repo: j.Repo, Log: j.Log}
	if j.Task != nil {
		c.Task = j.Task.ID
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return r.write(r.claimPath(j.Session), b, "claim for session "+j.Log)
}

// unclaim ends a job's claim. A claim left behind holds its task until the next runner, so
// it tries again while another write holds the vault.
func (r *Runner) unclaim(j *Job) {
	var err error
	for wait := time.Second; wait <= 9*time.Second; wait *= 3 {
		if err = r.write(r.claimPath(j.Session), nil, "end the claim of session "+j.Log); err == nil {
			return
		}
		time.Sleep(wait)
	}
	r.say("could not end the claim of %s: %v", j.Log, err)
}

// sweepStale removes the claims left by a runner that ended, and counts each claimed
// task as a failed attempt (Specification 6.2).
func (r *Runner) sweepStale() error {
	files, _ := filepath.Glob(filepath.Join(r.root(), ".carto", "claims", "*.json"))
	if len(files) == 0 {
		return nil
	}
	k, err := work.New(r.Folder, r.Now())
	if err != nil {
		return err
	}
	defer k.Writer().Abort()
	n := 0
	for _, f := range files {
		var c core.Claim
		if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &c) == nil && c.Task != "" {
			if err := k.Attempt(c.Goal, c.Task, "the runner that ran it ended before the task did"); err == nil {
				n++
			}
		}
		k.Writer().Remove(f)
	}
	claims, tasks := "claims", "tasks count"
	if len(files) == 1 {
		claims = "claim"
	}
	if n == 1 {
		tasks = "task counts"
	}
	_, err = k.Writer().Commit(fmt.Sprintf("remove %d %s of an ended runner; %d %s a failed attempt", len(files), claims, n, tasks))
	return err
}

// ---------- the queue ----------

// Queue returns the jobs that are ready now, in the order of Specification 6.1.
func Queue(p *core.Project) []*Job {
	var closing, apply, landing, upkeepNow, tasks, upkeepDue []*Job
	for _, d := range p.ToApply() {
		apply = append(apply, &Job{Kind: "apply", Goal: d})
	}
	goals := p.Of("goal")
	sort.SliceStable(goals, func(i, j int) bool {
		pi, pj := priority(p.S(goals[i], "priority")), priority(p.S(goals[j], "priority"))
		if pi != pj {
			return pi < pj
		}
		return p.S(goals[i], "opened") < p.S(goals[j], "opened")
	})
	for _, g := range goals {
		if !p.Live(g) {
			continue
		}
		if !blockedByAfter(p, g) {
			for _, t := range p.Ready(g) {
				j := &Job{Kind: "task", Goal: g, Task: t, Repo: p.TaskRepo(t), Branch: p.TaskBranch(t)}
				switch {
				case p.Standing(g) && t.F["due"] == "":
					upkeepNow = append(upkeepNow, j)
				case p.Standing(g):
					upkeepDue = append(upkeepDue, j)
				default:
					tasks = append(tasks, j)
				}
			}
		}
		if _, whole := p.ReadyToLand(g); whole && (!p.Reviewed(g) || p.S(p.Project, "landing") == "agent") {
			landing = append(landing, &Job{Kind: "land", Goal: g, Policy: p.S(p.Project, "landing"), Local: !land.PullRequests(p)})
		}
		if !p.Standing(g) {
			if d, k := p.GoalTasks(g, time.Time{}); k > 0 && d == k {
				all := true
				for _, b := range p.GoalBranches(g) {
					all = all && b.State == "landed"
				}
				if all {
					closing = append(closing, &Job{Kind: "close", Goal: g})
				}
			}
		}
	}
	sort.SliceStable(upkeepDue, func(i, j int) bool { return upkeepDue[i].Task.F["due"] < upkeepDue[j].Task.F["due"] })
	out := append(append(append(append(append(closing, apply...), landing...), upkeepNow...), tasks...), upkeepDue...)
	if p.SweepQueue() > 0 {
		out = append(out, &Job{Kind: "sweep"})
	}
	return out
}

func priority(s string) int {
	switch s {
	case "high":
		return 0
	case "low":
		return 2
	}
	return 1
}

func blockedByAfter(p *core.Project, g string) bool {
	for _, a := range md.Names(p.FM(g, "after")) {
		if p.S(a, "status") != "done" {
			return true
		}
	}
	return false
}

// ---------- running ----------

// Run takes the lease and runs jobs until only work that waits on the user is left, or
// once, when once is true.
func (r *Runner) Run(ctx context.Context, once bool) error {
	if r.Max <= 0 {
		r.Max = 3
	}
	if r.Budget <= 0 {
		r.Budget = 2 * time.Hour
	}
	if r.Lease <= 0 {
		r.Lease = time.Hour
	}
	if err := r.take(); err != nil {
		return err
	}
	defer r.release()
	if err := r.sweepStale(); err != nil {
		return err
	}
	running := map[string]*Job{} // key -> job
	done := make(chan *Job)
	tried := map[string]bool{} // sweeps and landings run once per runner
	var wg sync.WaitGroup
	for {
		p, err := r.Load()
		if err != nil {
			return err
		}
		busy := map[string]bool{}
		for _, j := range running {
			busy[j.Repo+"|"+j.Branch] = true
		}
		started := 0
		for _, j := range Queue(p) {
			if len(running) >= r.Max {
				break
			}
			key := j.Kind + "|" + j.Goal
			if j.Task != nil {
				key += "|" + j.Task.ID
			}
			if running[key] != nil || (j.Kind != "task" && tried[key]) || (j.Repo != "" && busy[j.Repo+"|"+j.Branch]) {
				continue
			}
			if j.Kind == "close" {
				tried[key] = true
				r.say("close %s", j.Goal)
				r.closeGoal(j.Goal)
				continue
			}
			if err := r.start(ctx, p, j); err != nil {
				var held *busyError
				if errors.As(err, &held) {
					r.say("wait with %s: %v", key, err)
					tried[key] = true
					continue
				}
				r.say("could not start %s: %v", key, err)
				if j.Task != nil {
					r.attempt(j, err.Error())
				}
				tried[key] = true
				continue
			}
			tried[key] = true
			running[key] = j
			busy[j.Repo+"|"+j.Branch] = true
			started++
			wg.Add(1)
			go func(j *Job) {
				defer wg.Done()
				outcome, result := r.exec(ctx, j)
				r.finish(j, outcome, result)
				done <- j
			}(j)
		}
		if len(running) == 0 {
			r.say("nothing left that the runner can do")
			return nil
		}
		renew := time.NewTicker(r.Lease / 3)
	wait:
		for {
			select {
			case j := <-done:
				for k, x := range running {
					if x == j {
						delete(running, k)
					}
				}
				break wait
			case <-renew.C:
				r.renew()
			case <-ctx.Done():
				renew.Stop()
				wg.Wait()
				return ctx.Err()
			}
		}
		renew.Stop()
		if once && len(running) == 0 {
			return nil
		}
	}
}

func (r *Runner) attempt(j *Job, why string) {
	k, err := work.New(r.Folder, r.Now())
	if err != nil {
		return
	}
	defer k.Writer().Abort()
	if err := k.Attempt(j.Goal, j.Task.ID, why); err != nil {
		r.say("no attempt counted for %s of %s: %v", j.Task.ID, j.Goal, err)
		return
	}
	if _, err := k.Writer().Commit(fmt.Sprintf("%s of %s failed", j.Task.ID, j.Goal)); err != nil {
		r.say("could not record the attempt of %s of %s: %v", j.Task.ID, j.Goal, err)
	}
}

func (r *Runner) closeGoal(goal string) {
	if r.Close == nil {
		return
	}
	if err := r.Close(goal); err != nil {
		r.say("close %s: %v", goal, err)
	}
}

// start prepares the worktree, the claim, and the session log of a job.
func (r *Runner) start(ctx context.Context, p *core.Project, j *Job) error {
	j.Session = newUUID()
	j.Dir = r.Folder
	if j.Kind == "task" {
		clone := r.Clone(j.Repo)
		if clone == "" {
			return fmt.Errorf("no local clone of %s on this machine; run carto setup --root", j.Repo)
		}
		dir, err := r.worktree(clone, j.Repo, j.Branch, p.Default, !land.PullRequests(p))
		if err != nil {
			return err
		}
		j.Dir = dir
	}
	now := r.Now()
	j.Log = now.Format("2006-01-02 1504") + " " + j.Session[:6]
	if err := r.writeLog(j, now, "running", "", "Started."); err != nil {
		return err
	}
	return r.claim(j)
}

// worktree checks out branch in its own folder. A new branch starts at origin's base, or at
// the clone's own base when the project lands locally.
func (r *Runner) worktree(clone, repo, branch, base string, local bool) (string, error) {
	dir := filepath.Join(r.Home, "worktrees", repo, strings.ReplaceAll(branch, "/", "-"))
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if list, err := exec.Command("git", "-C", clone, "worktree", "list", "--porcelain").Output(); err == nil {
		at := ""
		for _, l := range strings.Split(string(list), "\n") {
			if strings.HasPrefix(l, "worktree ") {
				at = strings.TrimPrefix(l, "worktree ")
			}
			if l == "branch refs/heads/"+branch && !samePath(at, dir) {
				return "", &busyError{branch, at}
			}
		}
	}
	exists := exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
	var args []string
	if exists {
		args = []string{"-C", clone, "worktree", "add", dir, branch}
	} else {
		start := base
		if !local && exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", "origin/"+base).Run() == nil {
			start = "origin/" + base
		}
		args = []string{"-C", clone, "worktree", "add", "-b", branch, dir, start}
	}
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %v: %s", err, bytes.TrimSpace(out))
	}
	return dir, nil
}

func samePath(a, b string) bool {
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return a == b || ra != "" && ra == rb
}

// busyError says that another session works on the branch, in a worktree of its own.
type busyError struct{ branch, at string }

func (e *busyError) Error() string {
	return fmt.Sprintf("the branch %s is checked out at %s, by another session", e.branch, e.at)
}

// Prompt is what the agent of a job is told.
func Prompt(folder string, j *Job) string {
	switch j.Kind {
	case "task":
		return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. Goal: [[%s]]. Task: %s, on [[%s]]: %s. "+
			"You are in a worktree of the branch %s of %s.\n\n"+
			"Follow the carto-goal skill's Run steps for this one task only: do the task, commit, check the entity's affected obligations on the branch "+
			"(carto-check), then call the work tool with action done, the goal, the task, and the commit. If you cannot finish, call work with action attempt "+
			"and the reason. If you need a decision of the user that changes the scope, call work with action ask and block this task. Do not start other "+
			"tasks, and do not edit Cartographer documents directly. End with one line.",
			folder, j.Goal, j.Task.ID, j.Task.Target, j.Task.Text, j.Branch, j.Repo)
	case "land":
		if j.Local {
			end := "Do not land: the landing policy is user, so the user lands the goal from Attention."
			if j.Policy == "agent" {
				end = "Then land it with `carto land \"" + j.Goal + "\"`, check the goal at the default branch, and close it."
			}
			return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. Every task of the goal [[%s]] is done. "+
				"Follow the carto-goal skill's Land steps up to the merge: rebase each branch on the clone's own default branch (not origin's), re-check stale pending obligations, "+
				"send the reviewer, and add its findings as tasks. This project has no pull requests (pull_requests: false): do not push and do not "+
				"open any. Log \"ready to land\" with the branches. %s End with one line.", folder, j.Goal, end)
		}
		if j.Policy == "agent" {
			return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. Every task of the goal [[%s]] is done. "+
				"Follow the carto-goal skill's Land and close steps: rebase, re-check stale pending obligations, review, open the pull requests, "+
				"merge each when its checks pass (definition repositories first), log each merge as \"landed <repository> <merge commit>\", "+
				"check the goal at the default branch, and close it. End with one line.", folder, j.Goal)
		}
		return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. Every task of the goal [[%s]] is done. "+
			"Follow the carto-goal skill's Land and close steps up to the merge: rebase each branch, re-check stale pending obligations, "+
			"send the reviewer, add its findings as tasks, and open one pull request in each repository. Do not merge: the landing policy is "+
			"user, so the user lands the goal from Attention. Log \"ready to land\" with the pull requests. End with one line.", folder, j.Goal)
	case "apply":
		return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. The user chose an option of [[%s]], and its change waits "+
			"to be carried out. Read the document's Options and the chosen option's Change line. For a declared change (an adoption, a waiver, an "+
			"entity, an entry), call propose with because set to [[%s]] and the operations; it applies at once as the user's act. For an upkeep "+
			"task, call work with action add on Upkeep, with the document as its source. Do nothing else. End with one line.", folder, j.Goal, j.Goal)
	case "sweep":
		return fmt.Sprintf("You are a Cartographer agent working unattended. Project: %s. Run a sweep with the carto-check skill: "+
			"check the unverified and stale obligations in the order of the sweep queue, then the coverage of each repository. End with one line.", folder)
	}
	return ""
}

func (r *Runner) command(j *Job) []string {
	agent := r.Agent
	if len(agent) == 0 {
		agent = DefaultAgent
	}
	var out []string
	for _, a := range agent {
		if a == "{dirs}" {
			for _, d := range r.Dirs {
				if d != j.Dir {
					out = append(out, "--add-dir", d)
				}
			}
			continue
		}
		a = strings.ReplaceAll(a, "{prompt}", Prompt(r.Folder, j))
		a = strings.ReplaceAll(a, "{session}", j.Session)
		out = append(out, a)
	}
	return out
}

type agentResult struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
}

// exec runs the agent of a job within its budget.
func (r *Runner) exec(ctx context.Context, j *Job) (string, string) {
	c, cancel := context.WithTimeout(ctx, r.Budget)
	defer cancel()
	argv := r.command(j)
	cmd := exec.CommandContext(c, argv[0], argv[1:]...)
	cmd.Dir = j.Dir
	cmd.Env = append(os.Environ(), "CARTO_SESSION="+j.Session, "CARTO_PROJECT="+r.Folder)
	if os.Getenv("CARTO_BIN") == "" {
		if self, err := os.Executable(); err == nil {
			cmd.Env = append(cmd.Env, "CARTO_BIN="+self)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	what := j.Kind
	if j.Goal != "" {
		what += " " + j.Goal
	}
	if j.Task != nil {
		what += " " + j.Task.ID
	}
	r.say("start %s in %s", what, j.Dir)
	err := cmd.Run()
	var res agentResult
	json.Unmarshal(stdout.Bytes(), &res)
	text := strings.TrimSpace(res.Result)
	if text == "" {
		text = strings.TrimSpace(stdout.String())
	}
	switch {
	case errors.Is(c.Err(), context.DeadlineExceeded):
		return "stopped", fmt.Sprintf("the session ran past its budget of %s", r.Budget)
	case err != nil:
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "failed", msg
	case res.IsError:
		return "failed", text
	}
	return "done", text
}

// finish records the end of a job: the session log, the claim, and a failed attempt
// when a task is not done.
func (r *Runner) finish(j *Job, outcome, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unclaim(j)
	if j.Task != nil {
		p, err := r.Load()
		if err == nil {
			for _, t := range p.Tasks[j.Goal] {
				if t.ID == j.Task.ID && t.Box == " " && core.Attempts(t) == core.Attempts(j.Task) && len(p.Blocked(t)) == 0 {
					if outcome == "done" {
						outcome = "failed"
						result = "the session ended and the task is not done: " + result
					}
					r.attempt(j, firstLine(result))
				}
			}
		}
	}
	r.writeLog(j, r.Now(), outcome, commitsOf(j), result)
	r.say("%s %s %s: %s", outcome, j.Kind, j.Goal, firstLine(result))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func commitsOf(j *Job) string {
	if j.Kind != "task" {
		return ""
	}
	out, err := exec.Command("git", "-C", j.Dir, "log", "--format=%h %s", "-n", "10", "--since=6 hours ago").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writeLog writes the session log document (Specification 6.2).
func (r *Runner) writeLog(j *Job, now time.Time, outcome, commits, result string) error {
	path := filepath.Join(r.Folder, "Sessions", now.Format("2006-01"), j.Log+".md")
	if b, err := os.ReadFile(path); err == nil {
		text := string(b)
		for k, v := range map[string]any{"outcome": outcome, "ended": now.Format("2006-01-02T15:04:05"), "updated": now.Format("2006-01-02T15:04:05")} {
			text, _ = md.SetField(text, k, v)
		}
		if commits != "" {
			text = md.AppendToSection(text, "Commits", "```\n"+commits+"\n```")
		}
		text = md.AppendToSection(text, "Outcome", strings.TrimSpace(result))
		w := vault.NewWriter(r.root())
		w.Put(path, text)
		_, err := w.Commit("end session " + j.Log)
		return err
	}
	task := ""
	if j.Task != nil {
		task = "[[" + j.Goal + "#^" + j.Task.Block + "]]"
	}
	ts := now.Format("2006-01-02T15:04:05")
	desc := fmt.Sprintf("%s of %s", j.Kind, j.Goal)
	if j.Task != nil {
		desc = fmt.Sprintf("%s of %s", j.Task.ID, j.Goal)
	}
	if j.Kind == "sweep" {
		desc = "A sweep"
	}
	text := strings.Join([]string{"---", "id: ses-" + j.Session[:6], "type: carto-session", "description: " + md.Value(desc+"."), "tags: []",
		"created: " + ts, "updated: " + ts, "task: " + md.Value(task), "repository: " + md.Value(j.Repo), "branch: " + md.Value(j.Branch),
		"started: " + ts, `ended: ""`, "outcome: " + outcome, "session: " + j.Session, "---", "",
		"## Instructions", "", Prompt(r.Folder, j), "", "## Actions", "", "The session runs headless; its transcript is the Claude Code session " + j.Session + ".", "",
		"## Commits", "", "## Outcome", ""}, "\n") + "\n"
	w := vault.NewWriter(r.root())
	w.Put(path, text)
	_, err := w.Commit("start session " + j.Log)
	return err
}
