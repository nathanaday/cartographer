package work

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nathanaday/cartographer/internal/md"
)

var chosenLine = regexp.MustCompile(`(?m)^\[chosen:: \d+\].*$`)

// pendingApply returns the apply field of a new choice: pending when the chosen option
// carries a change that an agent must carry out.
func pendingApply(body string, chosen int) string {
	for _, o := range optionsOf(body) {
		if o.N == chosen && o.Change != "" {
			return " [apply:: pending]"
		}
	}
	return ""
}

func setApply(text, value string) string {
	return chosenLine.ReplaceAllStringFunc(text, func(l string) string { return setField(l, "apply", value) })
}

// SplitTask splits "Goal#T2" into the goal and the task.
func SplitTask(key string) (string, string, error) {
	goal, task, ok := strings.Cut(key, "#")
	if !ok || !regexp.MustCompile(`^T\d+$`).MatchString(task) {
		return "", "", fmt.Errorf("%q does not name a task; write Goal#T2", key)
	}
	return goal, task, nil
}

// DeferTask hides a task's Land or Unstick line from Attention until a date. It is the
// user's own act.
func (k *Work) DeferTask(key, until string) error {
	if md.Time(until).IsZero() {
		return fmt.Errorf("%q is not a date", until)
	}
	goal, id, err := SplitTask(key)
	if err != nil {
		return err
	}
	if _, err := k.editTask(goal, id, func(_, l string) (string, error) { return setField(l, "defer", until), nil }); err != nil {
		return err
	}
	k.log = append(k.log, fmt.Sprintf("deferred %s of [[%s]] until %s", id, goal, until))
	return nil
}

// Retry clears a stuck task's attempts, so the runner takes it again, with the user's
// hint in its Task details. It is the user's own act.
func (k *Work) Retry(key, hint string) error {
	goal, id, err := SplitTask(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(hint) == "" {
		return errors.New("a retry needs a hint for the agent")
	}
	if _, err := k.editTask(goal, id, func(box, l string) (string, error) {
		if box != " " {
			return "", fmt.Errorf("%s is not open", id)
		}
		return regexp.MustCompile(` \[attempts:: \d+\]`).ReplaceAllString(l, ""), nil
	}); err != nil {
		return err
	}
	if err := k.detail(goal, id, "Hint from the user: "+strings.TrimSpace(hint)); err != nil {
		return err
	}
	k.log = append(k.log, fmt.Sprintf("%s of [[%s]] retried with a hint", id, goal))
	return k.Log(goal, "the user asked for a retry of "+id+" with a hint")
}

// Take gives a task to the user: the runner no longer starts it. It is the user's own act.
func (k *Work) Take(key string) error {
	goal, id, err := SplitTask(key)
	if err != nil {
		return err
	}
	if _, err := k.editTask(goal, id, func(box, l string) (string, error) {
		if box != " " {
			return "", fmt.Errorf("%s is not open", id)
		}
		return setField(l, "user", "true"), nil
	}); err != nil {
		return err
	}
	k.log = append(k.log, fmt.Sprintf("%s of [[%s]] taken by the user", id, goal))
	return k.Log(goal, "the user took "+id)
}

// Request records the changes the user asks for before a landing. For an upkeep task it
// opens the task again with the request in its details; for a goal it adds a task. It is
// the user's own act.
func (k *Work) Request(key, text, target string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("a request for changes needs the changes")
	}
	if goal, id, err := SplitTask(key); err == nil {
		if _, err := k.editTask(goal, id, func(box, l string) (string, error) {
			if box != "x" {
				return "", fmt.Errorf("%s is not done", id)
			}
			l = "- [ ]" + l[5:]
			for _, f := range []string{"done", "commit", "attempts"} {
				l = regexp.MustCompile(` \[`+f+`::[^\]]*\]`).ReplaceAllString(l, "")
			}
			return l, nil
		}); err != nil {
			return err
		}
		if err := k.detail(goal, id, "Changes the user asked for before landing: "+text); err != nil {
			return err
		}
		k.log = append(k.log, fmt.Sprintf("%s of [[%s]] opened again for changes", id, goal))
		return k.Log(goal, "the user asked for changes to "+id+" before landing")
	}
	if target == "" {
		return errors.New("a request for changes to a goal names the entity the new task is on")
	}
	id, err := k.Add(key, target, text, map[string]string{"by": "user"})
	if err != nil {
		return err
	}
	return k.Log(key, "the user asked for changes before landing: "+id)
}

// detail adds a line under a task's heading in Task details.
func (k *Work) detail(goal, id, line string) error {
	path, text, err := k.read(goal)
	if err != nil {
		return err
	}
	section := md.Section(text, "Task details")
	head := "### " + id
	if !strings.Contains(section, head) {
		k.w.Put(path, md.AppendToSection(text, "Task details", head+"\n\n"+line))
		return nil
	}
	lines := strings.Split(text, "\n")
	in := false
	for i, l := range lines {
		switch {
		case l == head:
			in = true
		case in && (strings.HasPrefix(l, "### ") || strings.HasPrefix(l, "## ")):
			lines = append(lines[:i], append([]string{line, ""}, lines[i:]...)...)
			k.w.Put(path, strings.Join(lines, "\n"))
			return nil
		}
	}
	return fmt.Errorf("[[%s]] has a heading %s that does not end", goal, head)
}

// DecideLint records the user's decision on a lint result as an answered question that
// holds it, so that an agent carries out the chosen option. It is the user's own act.
func (k *Work) DecideLint(rule, entity, message string, chosen int) (string, error) {
	options := []Option{
		{Title: "Fix it on the map", Recommended: true, Change: fmt.Sprintf("propose the map change that ends lint %s on [[%s]].", rule, entity)},
		{Title: "Waive", Change: fmt.Sprintf("add a waiver of lint %s to [[%s]].", rule, entity)},
	}
	name, err := k.Ask(fmt.Sprintf("Lint %s on %s", rule, entity), message, "The user decided on this lint result in Attention.", options, []string{entity}, nil, "")
	if err != nil {
		return "", err
	}
	path, text, err := k.read(name)
	if err != nil {
		return "", err
	}
	text, err = md.SetField(text, "holds", fmt.Sprintf("lint %s [[%s]]", rule, entity))
	if err != nil {
		return "", err
	}
	text, err = md.SetField(text, "asked_by", "user")
	if err != nil {
		return "", err
	}
	k.w.Put(path, text)
	return name, k.Answer(name, chosen, "")
}
