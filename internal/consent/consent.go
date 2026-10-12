// Package consent keeps the user's own recent prompts, captured by the prompt hook, so that
// a tool that acts on the user's behalf (approve, undo, an answer given in chat) can check
// that the user said it (Specification 8.1).
package consent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/config"
)

// Window is how long a captured prompt counts.
const Window = 30 * time.Minute

// Prompt is one prompt the user submitted.
type Prompt struct {
	Session string    `json:"session"`
	Text    string    `json:"text"`
	At      time.Time `json:"at"`
}

func dir() string { return filepath.Join(config.Home(), "prompts") }

// Save records a prompt of a session. Each session keeps its last 20 prompts.
func Save(session, text string, at time.Time) error {
	if session == "" {
		session = "unknown"
	}
	if err := os.MkdirAll(dir(), 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir(), filepath.Base(session)+".jsonl")
	prompts := read(path)
	prompts = append(prompts, Prompt{session, text, at})
	if len(prompts) > 20 {
		prompts = prompts[len(prompts)-20:]
	}
	var b strings.Builder
	for _, p := range prompts {
		line, _ := json.Marshal(p)
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func read(path string) []Prompt {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Prompt
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var p Prompt
		if json.Unmarshal(s.Bytes(), &p) == nil {
			out = append(out, p)
		}
	}
	return out
}

// Recent returns the prompts of every session within the window before now, newest first.
func Recent(now time.Time) []Prompt {
	files, _ := filepath.Glob(filepath.Join(dir(), "*.jsonl"))
	var out []Prompt
	for _, f := range files {
		for _, p := range read(f) {
			if age := now.Sub(p.At); age >= 0 && age <= Window {
				out = append(out, p)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

func norm(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// Find returns the newest recent prompt that holds every one of the phrases.
func Find(now time.Time, phrases ...string) (Prompt, bool) {
	for _, p := range Recent(now) {
		text := norm(p.Text)
		all := true
		for _, ph := range phrases {
			if !strings.Contains(text, norm(ph)) {
				all = false
				break
			}
		}
		if all {
			return p, true
		}
	}
	return Prompt{}, false
}

// FindAny returns the newest recent prompt that holds any one of the phrases.
func FindAny(now time.Time, phrases ...string) (Prompt, bool) {
	for _, p := range Recent(now) {
		text := norm(p.Text)
		for _, ph := range phrases {
			if ph != "" && strings.Contains(text, norm(ph)) {
				return p, true
			}
		}
	}
	return Prompt{}, false
}

var stopWords = map[string]bool{"a": true, "an": true, "the": true, "of": true, "to": true, "for": true, "and": true, "in": true, "on": true, "from": true, "with": true, "as": true, "by": true, "at": true, "its": true, "their": true, "is": true, "are": true}

// words splits a text into lowercase words; a hyphen, a dot, and an underscore stay
// inside a word, so that names such as p3-cloud stay whole.
func words(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_' || r > 127)
	}) {
		if w = strings.Trim(w, ".-_"); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// Change is what a prompt may name to decide on a change.
type Change struct {
	ID, Name, Title string
	Creates         []string  // the documents the change creates
	Proposed        time.Time // when the agent proposed it
}

var allWords = map[string]bool{"all": true, "every": true, "each": true, "both": true, "them": true}

// Decides returns the newest recent prompt in which the user decides on a change: it holds
// one of the verbs, and it names the change by its id, its name, every word of its title,
// or a document it creates; or, after the change was proposed, it decides on them all.
func Decides(now time.Time, verbs []string, c Change) (Prompt, bool) {
	var need []string
	for _, w := range words(c.Title) {
		if !stopWords[w] && w != "suggested" {
			need = append(need, w)
		}
	}
	for _, p := range Recent(now) {
		text := norm(p.Text)
		verb := false
		for _, v := range verbs {
			verb = verb || strings.Contains(text, v)
		}
		if !verb {
			continue
		}
		if strings.Contains(text, norm(c.ID)) || c.Name != "" && strings.Contains(text, norm(c.Name)) {
			return p, true
		}
		have := map[string]bool{}
		for _, w := range words(p.Text) {
			have[w] = true
		}
		all := len(need) > 0
		for _, w := range need {
			all = all && have[w]
		}
		if all {
			return p, true
		}
		for _, d := range c.Creates {
			if d != "" && strings.Contains(" "+strings.Join(words(p.Text), " ")+" ", " "+strings.Join(words(d), " ")+" ") {
				return p, true
			}
		}
		for w := range have {
			if allWords[w] && !c.Proposed.IsZero() && p.At.After(c.Proposed) {
				return p, true
			}
		}
	}
	return Prompt{}, false
}
