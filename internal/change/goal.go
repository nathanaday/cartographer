package change

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

var actLineRe = regexp.MustCompile(`^- \*\*(add|revise|retire|adopt|attach|use|join|create)\*\* (.*)$`)

// relationField is the field each relation act writes, and which side owns it.
var relationField = map[string]string{"adopt": "adopts", "attach": "interfaces", "use": "uses", "join": "systems"}

// approveGoal applies a draft goal's Map changes (Specification 5.1.3): it numbers new
// clauses and writes them as pending, writes pending revisions and retirements, applies
// relations, and makes the goal active.
func approveGoal(ix *Index, w *vault.Writer, goal string, now time.Time) ([]string, error) {
	path, err := ix.path(goal)
	if err != nil {
		return nil, err
	}
	text, err := w.Read(path)
	if err != nil {
		return nil, err
	}
	d, err := md.Parse(path, text)
	if err != nil {
		return nil, err
	}
	if s := md.Str(d.FM["status"]); s != "draft" {
		return nil, fmt.Errorf("[[%s]] is %s, not a draft", goal, s)
	}
	var out []string
	lines := strings.Split(md.Section(d.Body, "Map changes"), "\n")
	rewrites := map[string]string{}
	for _, line := range lines {
		m := actLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		verb, rest := m[1], m[2]
		links := md.Links(rest)
		_, after, _ := strings.Cut(rest, " · ")
		switch verb {
		case "add", "revise", "retire":
			if len(links) == 0 {
				return nil, fmt.Errorf("%q names no clause", line)
			}
			op := Op{Op: "clause", Action: verb, Doc: links[0].Name, ID: links[0].Anchor, Goal: goal,
				Statement: md.StripFields(after), Fields: md.Fields(after)}
			if verb == "add" {
				op.ID = ""
			}
			id, err := clause(ix, w, op, now)
			if err != nil {
				return nil, err
			}
			if verb == "add" {
				rewrites[line] = strings.Replace(line, "#"+links[0].Anchor+"]]", "#"+id+"]]", 1)
			}
			out = append(out, fmt.Sprintf("**%s** [[%s#%s]] (pending, for [[%s]])", verb, links[0].Name, id, goal))
		case "adopt", "attach", "use", "join":
			if len(links) != 2 {
				return nil, fmt.Errorf("%q needs two links", line)
			}
			s, err := apply(ix, w, Op{Op: "add", Doc: links[1].Name, Field: relationField[verb], Value: "[[" + links[0].Name + "]]"}, now)
			if err != nil {
				return nil, err
			}
			out = append(out, s...)
		case "create":
			if len(links) == 0 {
				return nil, fmt.Errorf("%q names no document", line)
			}
			s, err := apply(ix, w, Op{Op: "set", Doc: links[0].Name, Field: "status", Value: "active"}, now)
			if err != nil {
				return nil, err
			}
			out = append(out, s...)
		}
	}
	text, err = w.Read(path)
	if err != nil {
		return nil, err
	}
	for from, to := range rewrites {
		text = strings.Replace(text, from, to, 1)
	}
	if text, err = md.SetField(text, "status", "active"); err != nil {
		return nil, err
	}
	if text, err = md.SetField(text, "opened", now.Format("2006-01-02")); err != nil {
		return nil, err
	}
	text = md.AppendToSection(text, "Log", "- "+now.Format("2006-01-02 15:04")+" the user approved the goal.")
	w.Put(path, text)
	return append(out, fmt.Sprintf("**set** `status` active on [[%s]]", goal)), nil
}

// CloseGoal makes a goal's pending clause acts active and marks it done (Specification
// 5.1.3). The user approved these acts with the goal, so closing needs no approval.
func CloseGoal(ix *Index, w *vault.Writer, goal string, now time.Time) ([]string, error) {
	var out []string
	for name, t := range ix.Types {
		if t != "contract" && t != "system" && t != "service" && t != "module" {
			continue
		}
		path := ix.Paths[name]
		text, err := w.Read(path)
		if err != nil {
			return nil, err
		}
		next, acts := settle(text, goal, now)
		if next != text {
			w.Put(path, next)
			for _, a := range acts {
				out = append(out, fmt.Sprintf("**%s** [[%s#%s]]", a[0], name, a[1]))
			}
		}
	}
	path, err := ix.path(goal)
	if err != nil {
		return nil, err
	}
	text, err := w.Read(path)
	if err != nil {
		return nil, err
	}
	if text, err = md.SetField(text, "status", "done"); err != nil {
		return nil, err
	}
	if text, err = md.SetField(text, "closed", now.Format("2006-01-02")); err != nil {
		return nil, err
	}
	w.Put(path, md.AppendToSection(text, "Log", "- "+now.Format("2006-01-02 15:04")+" every obligation met and every task closed: the goal is done."))
	return append(out, fmt.Sprintf("**set** `status` done on [[%s]]", goal)), nil
}

// settle makes the pending clause acts of a goal in one document active.
func settle(text, goal string, now time.Time) (string, [][2]string) {
	var acts [][2]string
	link := "[[" + goal + "]]"
	for _, m := range regexp.MustCompile(`(?m)^### (\S+)\n`).FindAllStringSubmatch(text, -1) {
		id := m[1]
		start, end, ok := clauseBlock(text, id)
		if !ok {
			continue
		}
		head := "### " + id + "\n"
		body := text[start+len(head) : end]
		if !strings.Contains(body, link) {
			continue
		}
		var stmt, flds, pend []string
		for _, l := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			switch {
			case strings.TrimSpace(l) == "":
			case strings.HasPrefix(l, "["):
				flds = append(flds, l)
			case strings.HasPrefix(l, ">"):
				pend = append(pend, l)
			default:
				stmt = append(stmt, l)
			}
		}
		fl := strings.Join(flds, " ")
		f := md.Fields(fl)
		act := ""
		switch {
		case f["state"] == "pending" && md.First(f["goal"]) == goal:
			fl = dropInline(dropInline(fl, "state"), "goal")
			act = "activate"
		case md.First(f["pending_goal"]) == goal:
			var s []string
			for _, p := range pend {
				s = append(s, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(p, ">")), "[!pending]")))
			}
			stmt, pend = []string{strings.Join(s, " ")}, nil
			fl = setInline(dropInline(dropInline(fl, "pending_rev"), "pending_goal"), "rev", f["pending_rev"])
			act = "revise"
		case md.First(f["pending_retire"]) == goal:
			for i, s := range stmt {
				stmt[i] = "~~" + strings.Trim(s, "~") + "~~"
			}
			fl = setInline(setInline(dropInline(fl, "pending_retire"), "state", "retired"), "retired", now.Format("2006-01-02"))
			act = "retire"
		default:
			continue
		}
		block := head + strings.Join(stmt, "\n")
		if fl != "" {
			block += "\n" + fl
		}
		if len(pend) > 0 {
			block += "\n" + strings.Join(pend, "\n")
		}
		text = text[:start] + block + "\n" + trailing(text[start:end]) + text[end:]
		acts = append(acts, [2]string{act, id})
	}
	return text, acts
}

// dropDraft drops a draft goal. A draft has applied nothing, so only its status changes.
func dropDraft(ix *Index, w *vault.Writer, goal string, now time.Time) ([]string, error) {
	path, err := ix.path(goal)
	if err != nil {
		return nil, err
	}
	text, err := w.Read(path)
	if err != nil {
		return nil, err
	}
	d, err := md.Parse(path, text)
	if err != nil {
		return nil, err
	}
	if s := md.Str(d.FM["status"]); s != "draft" {
		return nil, fmt.Errorf("[[%s]] is %s; only a draft goal is dropped from Attention", goal, s)
	}
	for k, v := range map[string]any{"status": "dropped", "closed": stamp(now), "updated": stamp(now)} {
		if text, err = md.SetField(text, k, v); err != nil {
			return nil, err
		}
	}
	w.Put(path, md.AppendToSection(text, "Log", "- "+now.Format("2006-01-02 15:04")+" the user dropped the draft."))
	return []string{"dropped the draft goal [[" + goal + "]]"}, nil
}
