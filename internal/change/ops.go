// Package change applies declared state: the operations of a change document, the user's
// own acts, approval, undo, and the transitions of a goal (approve, close).
package change

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/md"
	"github.com/nathanaday/cartographer/internal/vault"
)

// Op is one declared write.
type Op struct {
	Op        string            `json:"op"` // create, add, remove, set, line, clause, goal.approve
	Doc       string            `json:"doc,omitempty"`
	Path      string            `json:"path,omitempty"` // create: relative to the project folder
	Text      string            `json:"text,omitempty"` // create: the document
	Field     string            `json:"field,omitempty"`
	Value     string            `json:"value,omitempty"`
	Section   string            `json:"section,omitempty"`
	Line      string            `json:"line,omitempty"`
	Action    string            `json:"action,omitempty"` // clause: add, revise, retire
	ID        string            `json:"id,omitempty"`
	Statement string            `json:"statement,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
	Goal      string            `json:"goal,omitempty"` // clause: inside this goal, the act is pending
}

// Declared lists the frontmatter fields and sections that are declared state: only a
// change or the user's own act writes them.
var (
	DeclaredFields   = []string{"systems", "adopts", "interfaces", "uses", "stores", "platform", "entry", "project", "code", "schemas"}
	DeclaredSections = []string{"Clauses", "Requirements", "Waivers", "Map changes"}
)

// Index maps the documents of a project folder by name.
type Index struct {
	Folder string
	Paths  map[string]string
	Types  map[string]string
}

// NewIndex reads the names and types of a project's documents.
func NewIndex(folder string) (*Index, error) {
	abs, _ := filepath.Abs(folder)
	ix := &Index{Folder: abs, Paths: map[string]string{}, Types: map[string]string{}}
	if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
		return ix, nil
	}
	err := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		doc, err := md.Load(path)
		if err != nil {
			return nil
		}
		ix.Paths[doc.Name] = path
		ix.Types[doc.Name] = doc.Type
		return nil
	})
	return ix, err
}

// Project returns the name of the project document.
func (ix *Index) Project() string {
	for n, t := range ix.Types {
		if t == "project" {
			return n
		}
	}
	return ""
}

func (ix *Index) path(doc string) (string, error) {
	p, ok := ix.Paths[doc]
	if !ok {
		return "", fmt.Errorf("no document named %q in the project", doc)
	}
	return p, nil
}

// Apply stages the operations in the writer and returns a summary line for each.
func Apply(ix *Index, w *vault.Writer, ops []Op, now time.Time) ([]string, error) {
	var out []string
	for _, op := range ops {
		s, err := apply(ix, w, op, now)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op.Op, err)
		}
		out = append(out, s...)
	}
	return out, nil
}

func apply(ix *Index, w *vault.Writer, op Op, now time.Time) ([]string, error) {
	switch op.Op {
	case "create":
		path := filepath.Join(ix.Folder, filepath.FromSlash(op.Path))
		if !strings.HasPrefix(path, ix.Folder+string(filepath.Separator)) || !strings.HasSuffix(path, ".md") {
			return nil, errors.New("create needs a .md path inside the project folder")
		}
		if _, exists := ix.Paths[strings.TrimSuffix(filepath.Base(path), ".md")]; exists {
			return nil, fmt.Errorf("a document named %q exists", filepath.Base(path))
		}
		text := md.SetFields(op.Text, map[string]any{"created": stamp(now), "updated": stamp(now)})
		d, err := md.Parse(path, text)
		if err != nil {
			return nil, err
		}
		if d.Type == "project" && (ix.Project() != "" || filepath.Dir(path) != ix.Folder) {
			return nil, errors.New("a project folder holds one project document, at its top")
		}
		w.Put(path, text)
		ix.Paths[d.Name], ix.Types[d.Name] = path, d.Type
		return []string{fmt.Sprintf("**create** %s [[%s]]", d.Type, d.Name)}, nil
	case "add", "remove", "set":
		return edit(ix, w, op.Doc, func(text string) (string, error) {
			switch op.Op {
			case "add":
				return md.AddToList(text, op.Field, op.Value)
			case "remove":
				return md.RemoveFromList(text, op.Field, op.Value)
			}
			return md.SetField(text, op.Field, op.Value)
		}, fmt.Sprintf("**%s** `%s` %s on [[%s]]", op.Op, op.Field, op.Value, op.Doc))
	case "line":
		return edit(ix, w, op.Doc, func(text string) (string, error) {
			return md.AppendToSection(text, op.Section, op.Line), nil
		}, fmt.Sprintf("**add** to %s of [[%s]]: %s", op.Section, op.Doc, op.Line))
	case "clause":
		id, err := clause(ix, w, op, now)
		if err != nil {
			return nil, err
		}
		s := fmt.Sprintf("**%s** [[%s#%s]]", op.Action, op.Doc, id)
		if op.Goal != "" {
			s += " (pending, for [[" + op.Goal + "]])"
		}
		return []string{s}, nil
	case "goal.approve":
		return approveGoal(ix, w, op.Doc, now)
	case "goal.drop":
		return dropDraft(ix, w, op.Doc, now)
	}
	return nil, fmt.Errorf("no operation %q", op.Op)
}

func edit(ix *Index, w *vault.Writer, doc string, f func(string) (string, error), summary string) ([]string, error) {
	path, err := ix.path(doc)
	if err != nil {
		return nil, err
	}
	text, err := w.Read(path)
	if err != nil {
		return nil, err
	}
	next, err := f(text)
	if err != nil {
		return nil, err
	}
	w.Put(path, next)
	return []string{summary}, nil
}

// ---------- clauses ----------

var headRe = regexp.MustCompile(`(?m)^### ([A-Z][A-Z0-9]*)-(\d+)\s*$`)

// clauseBlock finds the "### id" block of a text: its start, its end, and its lines.
func clauseBlock(text, id string) (int, int, bool) {
	head := "### " + id + "\n"
	start := strings.Index(text, "\n"+head)
	if start < 0 {
		return 0, 0, false
	}
	start++
	end := len(text)
	rest := text[start+len(head):]
	for _, sep := range []string{"\n### ", "\n## "} {
		if i := strings.Index(rest, sep); i >= 0 && start+len(head)+i+1 < end {
			end = start + len(head) + i + 1
		}
	}
	return start, end, true
}

func setInline(fieldsLine, key, value string) string {
	re := regexp.MustCompile(`\[` + regexp.QuoteMeta(key) + `::[^\]]*(?:\]\][^\]]*)*\]`)
	field := "[" + key + ":: " + value + "]"
	if re.MatchString(fieldsLine) {
		return re.ReplaceAllLiteralString(fieldsLine, field)
	}
	if strings.TrimSpace(fieldsLine) == "" {
		return field
	}
	return fieldsLine + " " + field
}

func dropInline(fieldsLine, key string) string {
	re := regexp.MustCompile(`\s*\[` + regexp.QuoteMeta(key) + `::(?:[^\]]|\]\])*\]`)
	return strings.TrimSpace(re.ReplaceAllString(fieldsLine, ""))
}

func fieldsLine(f map[string]string) string {
	var keys []string
	for k := range f {
		keys = append(keys, k)
	}
	order := map[string]int{"verify": 0, "scope": 1, "evidence": 2, "on": 3, "rev": 4, "state": 5, "goal": 6}
	sort.Slice(keys, func(i, j int) bool {
		a, aok := order[keys[i]]
		b, bok := order[keys[j]]
		if aok && bok {
			return a < b
		}
		if aok != bok {
			return aok
		}
		return keys[i] < keys[j]
	})
	var parts []string
	for _, k := range keys {
		parts = append(parts, "["+k+":: "+f[k]+"]")
	}
	return strings.Join(parts, " ")
}

func clause(ix *Index, w *vault.Writer, op Op, now time.Time) (string, error) {
	path, err := ix.path(op.Doc)
	if err != nil {
		return "", err
	}
	text, err := w.Read(path)
	if err != nil {
		return "", err
	}
	d, err := md.Parse(path, text)
	if err != nil {
		return "", err
	}
	switch op.Action {
	case "add":
		code := md.Str(d.FM["code"])
		if code == "" {
			return "", fmt.Errorf("[[%s]] has no code, so its clauses cannot be numbered", op.Doc)
		}
		next := 1
		for _, m := range headRe.FindAllStringSubmatch(text, -1) {
			if n, _ := strconv.Atoi(m[2]); m[1] == code && n >= next {
				next = n + 1
			}
		}
		id := fmt.Sprintf("%s-%d", code, next)
		f := map[string]string{}
		for k, v := range op.Fields {
			f[k] = v
		}
		if op.Goal != "" {
			f["state"], f["goal"] = "pending", "[["+op.Goal+"]]"
		}
		block := "### " + id + "\n" + strings.TrimSpace(op.Statement)
		if fl := fieldsLine(f); fl != "" {
			block += "\n" + fl
		}
		section := "Requirements"
		if d.Type == "contract" {
			section = "Clauses"
		}
		w.Put(path, appendBlock(text, section, block))
		return id, nil
	case "revise", "retire":
		start, end, ok := clauseBlock(text, op.ID)
		if !ok {
			return "", fmt.Errorf("[[%s]] has no clause %s", op.Doc, op.ID)
		}
		head := "### " + op.ID + "\n"
		var stmt, flds, pend []string
		for _, l := range strings.Split(strings.TrimRight(text[start+len(head):end], "\n"), "\n") {
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
		rev, _ := strconv.Atoi(f["rev"])
		if rev == 0 {
			rev = 1
		}
		switch {
		case op.Action == "revise" && op.Goal != "":
			if _, busy := f["pending_rev"]; busy {
				return "", fmt.Errorf("%s already has a pending revision", op.ID)
			}
			fl = setInline(setInline(fl, "pending_rev", strconv.Itoa(rev+1)), "pending_goal", "[["+op.Goal+"]]")
			pend = []string{"> [!pending] " + strings.TrimSpace(op.Statement)}
		case op.Action == "revise":
			stmt = []string{strings.TrimSpace(op.Statement)}
			fl = setInline(fl, "rev", strconv.Itoa(rev+1))
		case op.Goal != "":
			fl = setInline(fl, "pending_retire", "[["+op.Goal+"]]")
		default:
			for i, s := range stmt {
				stmt[i] = "~~" + strings.Trim(s, "~") + "~~"
			}
			fl = setInline(dropInline(fl, "goal"), "state", "retired")
			fl = setInline(fl, "retired", now.Format("2006-01-02"))
		}
		block := head + strings.Join(stmt, "\n") + "\n" + fl
		if len(pend) > 0 {
			block += "\n" + strings.Join(pend, "\n")
		}
		w.Put(path, text[:start]+block+"\n"+trailing(text[start:end])+text[end:])
		return op.ID, nil
	}
	return "", fmt.Errorf("no clause action %q", op.Action)
}

// trailing keeps the blank lines that separated a block from the next one.
func trailing(block string) string {
	t := strings.TrimRight(block, "\n")
	n := len(block) - len(t) - 1
	if n < 0 {
		n = 0
	}
	return strings.Repeat("\n", n)
}

func appendBlock(text, section, block string) string {
	head := "## " + section + "\n"
	if !strings.Contains(text, "\n"+head) && !strings.HasPrefix(text, head) {
		return strings.TrimRight(text, "\n") + "\n\n" + head + "\n" + block + "\n"
	}
	return md.AppendToSection(text, section, "\n"+block)
}

func newWriter(root string) *vault.Writer { return vault.NewWriter(root) }
