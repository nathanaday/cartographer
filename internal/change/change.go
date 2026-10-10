package change

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Act is one applied set of operations: an approved proposal, or one act of the user.
// Base is the vault's commit before the act. Files maps each file the operations wrote
// (relative to the vault) to the hash of its text after the act, or "" when the act
// removed it. Undo restores those files from Base while each still has that text.
type Act struct {
	At      string            `json:"at"`
	Summary []string          `json:"summary"`
	Ops     []Op              `json:"ops"`
	Base    string            `json:"base,omitempty"`
	Files   map[string]string `json:"files,omitempty"`
	Undone  string            `json:"undone,omitempty"`
}

// Doc is a change document.
type Doc struct {
	Path, Name, ID, Status, By, Title string
	Acts                              []Act
	fresh                             *Act // the act applied in this run, which Seal records
}

var opsRe = regexp.MustCompile("(?s)```cartographer-acts\n(.*?)\n```")

// Load reads a change document.
func Load(path string) (*Doc, error) {
	d, err := md.Load(path)
	if err != nil {
		return nil, err
	}
	if d.Type != "change" {
		return nil, fmt.Errorf("%s is not a change document", d.Name)
	}
	c := &Doc{Path: path, Name: d.Name, ID: md.Str(d.FM["id"]), Status: md.Str(d.FM["status"]), By: md.Str(d.FM["by"]), Title: md.Str(d.FM["description"])}
	m := opsRe.FindStringSubmatch(d.Body)
	if m == nil {
		return nil, fmt.Errorf("%s has no acts block", d.Name)
	}
	if err := json.Unmarshal([]byte(m[1]), &c.Acts); err != nil {
		return nil, fmt.Errorf("%s: the acts block is not valid JSON: %w", d.Name, err)
	}
	return c, nil
}

func newID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return "chg-" + string(b)
}

func render(c *Doc, created, updated, proposed, applied string, log []string) string {
	acts, _ := json.MarshalIndent(c.Acts, "", "  ")
	var summary []string
	for _, a := range c.Acts {
		for _, s := range a.Summary {
			line := "- " + s
			if a.Undone != "" {
				line += " (undone " + a.Undone + ")"
			}
			summary = append(summary, line)
		}
	}
	fm := []string{
		"---",
		"id: " + c.ID,
		"type: carto-change",
		"description: " + md.Value(c.Title),
		"tags: []",
		"created: " + created,
		"updated: " + updated,
		"status: " + c.Status,
		"by: " + c.By,
		"proposed: " + md.Value(proposed),
		"applied: " + md.Value(applied),
		"---",
	}
	return strings.Join(fm, "\n") + "\n\n## Summary\n\n" + strings.Join(summary, "\n") +
		"\n\n## Acts\n\n```cartographer-acts\n" + string(acts) + "\n```\n\n## Log\n\n" + strings.Join(log, "\n") + "\n"
}

func stamp(t time.Time) string { return t.Format("2006-01-02T15:04:05") }

var unsafeName = regexp.MustCompile(`[\\/:*?"<>|#^\[\]]`)

func fileTitle(s string) string {
	s = strings.TrimSpace(unsafeName.ReplaceAllString(s, "-"))
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// Propose writes a proposed change for the user to approve. It checks the operations
// first, on a scratch writer, so that a proposal that cannot apply is refused now.
func Propose(ix *Index, title string, ops []Op, session string, now time.Time) (*Doc, error) {
	if len(ops) == 0 {
		return nil, errors.New("a change needs at least one operation")
	}
	root := vault.Root(ix.Folder)
	scratch := vault.NewScratch(root)
	summary, err := Apply(cloneIndex(ix), scratch, ops, now)
	if err != nil {
		return nil, err
	}
	c := &Doc{ID: newID(), Status: "proposed", By: "agent", Title: title, Acts: []Act{{At: stamp(now), Summary: summary, Ops: ops}}}
	path := filepath.Join(ix.Folder, "Changes", now.Format("2006-01"), now.Format("2006-01-02 1504")+" "+fileTitle(title)+".md")
	c.Path, c.Name = path, strings.TrimSuffix(filepath.Base(path), ".md")
	who := "the agent"
	if session != "" {
		who += " in session " + session
	}
	w := vault.NewWriter(root)
	w.Put(path, render(c, stamp(now), stamp(now), stamp(now), "", []string{"- " + now.Format("2006-01-02 15:04") + " proposed by " + who + "."}))
	if _, err := w.Commit("propose " + title); err != nil {
		return nil, err
	}
	return c, nil
}

func cloneIndex(ix *Index) *Index {
	c := &Index{Folder: ix.Folder, Paths: map[string]string{}, Types: map[string]string{}}
	for k, v := range ix.Paths {
		c.Paths[k] = v
	}
	for k, v := range ix.Types {
		c.Types[k] = v
	}
	return c
}

// Find returns a change document by its id or its name.
func Find(ix *Index, key string) (*Doc, error) {
	files, _ := filepath.Glob(filepath.Join(ix.Folder, "Changes", "*", "*.md"))
	for _, f := range files {
		c, err := Load(f)
		if err != nil {
			continue
		}
		if c.ID == key || c.Name == key {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no change %q in the project", key)
}

// applyAct writes operations into w and returns the act, with its base and its files.
func applyAct(ix *Index, w *vault.Writer, ops []Op, now time.Time) (Act, error) {
	if err := w.Hold(); err != nil {
		return Act{}, err
	}
	if !vault.IsRepo(w.Root) {
		return Act{}, errors.New("the vault must be a git repository, so that every act can be undone")
	}
	before := map[string]bool{}
	for _, p := range w.Paths() {
		before[p] = true
	}
	summary, err := Apply(ix, w, ops, now)
	if err != nil {
		return Act{}, err
	}
	files := map[string]string{}
	var rels []string
	for _, p := range w.Paths() {
		if !before[p] {
			files[p] = ""
			rels = append(rels, p)
		}
	}
	if err := vault.CommitDirty(w.Root, rels, "your edits before an act"); err != nil {
		return Act{}, err
	}
	return Act{At: stamp(now), Summary: summary, Ops: ops, Base: vault.Head(w.Root), Files: files}, nil
}

func hash(text string) string {
	s := sha256.Sum256([]byte(text))
	return hex.EncodeToString(s[:8])
}

// Seal records the text of each file of the act applied in this run as it is in w now.
// Call it after the map's computed state is staged in w, just before the commit. It does
// nothing when no act was applied.
func Seal(w *vault.Writer, c *Doc, now time.Time) error {
	a := c.fresh
	if a == nil {
		return nil
	}
	for rel := range a.Files {
		a.Files[rel] = ""
		if text, err := w.Read(filepath.Join(w.Root, filepath.FromSlash(rel))); err == nil {
			a.Files[rel] = hash(text)
		}
	}
	return put(w, c, now, "")
}

// Approve applies a proposed change into w. words are the user's own words of approval.
func Approve(ix *Index, w *vault.Writer, key, words string, now time.Time) (*Doc, error) {
	c, err := Find(ix, key)
	if err != nil {
		return nil, err
	}
	if c.Status != "proposed" {
		return nil, fmt.Errorf("%s is %s, not proposed", c.Name, c.Status)
	}
	act, err := applyAct(ix, w, c.Acts[0].Ops, now)
	if err != nil {
		return nil, err
	}
	c.Acts[0] = act
	c.Status = "applied"
	c.fresh = &c.Acts[0]
	return c, put(w, c, now, fmt.Sprintf("- %s approved by the user: “%s”. Applied.", now.Format("2006-01-02 15:04"), strings.TrimSpace(words)))
}

// Reject marks a proposed change rejected, in w.
func Reject(ix *Index, w *vault.Writer, key, words string, now time.Time) (*Doc, error) {
	c, err := Find(ix, key)
	if err != nil {
		return nil, err
	}
	if c.Status != "proposed" {
		return nil, fmt.Errorf("%s is %s, not proposed", c.Name, c.Status)
	}
	c.Status = "rejected"
	return c, put(w, c, now, fmt.Sprintf("- %s rejected by the user: “%s”.", now.Format("2006-01-02 15:04"), strings.TrimSpace(words)))
}

// put writes c into w, keeping the dates and the Log it has, and adds a Log line.
func put(w *vault.Writer, c *Doc, now time.Time, line string) error {
	created, proposed, applied := stamp(now), "", ""
	var log []string
	if text, err := w.Read(c.Path); err == nil {
		d, err := md.Parse(c.Path, text)
		if err != nil {
			return err
		}
		created, proposed, applied = md.Str(d.FM["created"]), md.Str(d.FM["proposed"]), md.Str(d.FM["applied"])
		for _, l := range strings.Split(strings.TrimSpace(md.Section(d.Body, "Log")), "\n") {
			if l != "" {
				log = append(log, l)
			}
		}
	}
	if c.Status == "applied" && applied == "" {
		applied = stamp(now)
	}
	if line != "" {
		log = append(log, line)
	}
	w.Put(c.Path, render(c, created, stamp(now), proposed, applied, log))
	return nil
}

// UserAct applies the user's own act into w at once, and records it in this hour's
// change document of the user's acts.
func UserAct(ix *Index, w *vault.Writer, title string, ops []Op, now time.Time) (*Doc, error) {
	act, err := applyAct(ix, w, ops, now)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(ix.Folder, "Changes", now.Format("2006-01"), now.Format("2006-01-02 15")+"00 Your edits.md")
	c, err := Load(path)
	if err == nil {
		c.Acts = append(c.Acts, act)
	} else {
		c = &Doc{Path: path, Name: strings.TrimSuffix(filepath.Base(path), ".md"), ID: newID(), Status: "applied", By: "user",
			Title: "Your edits, " + now.Format("2006-01-02") + " from " + now.Format("15") + ":00", Acts: []Act{act}}
	}
	c.fresh = &c.Acts[len(c.Acts)-1]
	return c, put(w, c, now, "- "+now.Format("2006-01-02 15:04")+" "+title+".")
}

// later reports whether act i of c came after act j of d. Times have one second of
// resolution: in one document the order of the acts decides, and across documents the
// base of the later act descends from the base of the earlier.
func later(root string, c *Doc, i int, d *Doc, j int) bool {
	a, b := c.Acts[i], d.Acts[j]
	switch {
	case a.At != b.At:
		return a.At > b.At
	case c.Path == d.Path:
		return i > j
	}
	return a.Base != b.Base && vault.IsAncestor(root, b.Base, a.Base)
}

// Undo takes back the last act that is not undone, into w: it restores each file of the
// act from the act's base. It refuses when a later write changed one of those files.
func Undo(ix *Index, w *vault.Writer, now time.Time) (*Doc, error) {
	if err := w.Hold(); err != nil {
		return nil, err
	}
	files, _ := filepath.Glob(filepath.Join(ix.Folder, "Changes", "*", "*.md"))
	var last *Doc
	at := -1
	for _, f := range files {
		c, err := Load(f)
		if err != nil {
			continue
		}
		for i, a := range c.Acts {
			if a.Base != "" && a.Undone == "" && (last == nil || later(w.Root, c, i, last, at)) {
				last, at = c, i
			}
		}
	}
	if last == nil {
		return nil, errors.New("there is no act to undo")
	}
	act := last.Acts[at]
	var rels []string
	for rel := range act.Files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		now := ""
		if text, err := w.Read(filepath.Join(w.Root, filepath.FromSlash(rel))); err == nil {
			now = hash(text)
		}
		if now != act.Files[rel] {
			return nil, fmt.Errorf("%s changed after “%s”; undo the later act first, or change it by hand", rel, strings.Join(act.Summary, "; "))
		}
	}
	for _, rel := range rels {
		abs := filepath.Join(w.Root, filepath.FromSlash(rel))
		if text, ok := vault.At(w.Root, act.Base, rel); ok {
			w.Put(abs, text)
		} else {
			w.Remove(abs)
		}
	}
	last.Acts[at].Undone = stamp(now)
	if last.By != "user" {
		last.Status = "undone"
	}
	return last, put(w, last, now, "- "+now.Format("2006-01-02 15:04")+" undone: "+strings.Join(act.Summary, "; ")+".")
}
