// Package md reads and writes the parts of a Markdown document that Cartographer uses:
// frontmatter, sections, wikilinks, inline fields, and tables.
package md

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	linkRe  = regexp.MustCompile(`\[\[([^\]|#]*)(?:#([^\]|]*))?(?:\|[^\]]*)?\]\]`)
	fieldRe = regexp.MustCompile(`\[(\w+)::\s*((?:\[\[[^\]]*\]\]|[^\]])*)\]`)
	blockRe = regexp.MustCompile(`\s*\^\w+\s*$`)
	aliasRe = regexp.MustCompile(`\[\[([^\]|#]*)[^\]]*\]\]`)
)

// Doc is one Markdown document of a vault.
type Doc struct {
	Path string         // absolute path
	Name string         // file name without extension; what a wikilink names
	FM   map[string]any // decoded frontmatter
	Raw  string         // frontmatter text between the fences
	Body string         // everything after the closing fence
	Type string         // frontmatter type without the "carto-" prefix
}

// Load reads one document. A document with no frontmatter has an empty FM.
func Load(path string) (*Doc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, string(b))
}

// Parse reads one document from its text.
func Parse(path, text string) (*Doc, error) {
	d := &Doc{Path: path, Name: strings.TrimSuffix(filepath.Base(path), ".md"), FM: map[string]any{}, Body: text}
	raw, body, ok := split(text)
	if !ok {
		return d, nil
	}
	if err := yaml.Unmarshal([]byte(raw), &d.FM); err != nil {
		return nil, fmt.Errorf("%s: frontmatter is not valid YAML: %w", filepath.Base(path), err)
	}
	if d.FM == nil {
		d.FM = map[string]any{}
	}
	d.Raw, d.Body = raw, body
	d.Type = strings.TrimPrefix(Str(d.FM["type"]), "carto-")
	return d, nil
}

func split(text string) (raw, body string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text, false
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return "", text, false
	}
	return text[4 : 4+end], text[4+end+5:], true
}

// Str renders a frontmatter value as text, as a reader sees it.
func Str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(x)
	}
}

// Time reads a frontmatter or field value as a time. It returns the zero time for none.
func Time(v any) time.Time {
	switch x := v.(type) {
	case time.Time:
		return x.UTC()
	case string:
		s := strings.TrimSpace(x)
		for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// Link is one wikilink: the document it names and the heading or block after '#'.
type Link struct {
	Name, Anchor string
}

// Links returns the wikilinks in a value: a string, or a list of strings.
func Links(v any) []Link {
	var out []Link
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		for _, e := range x {
			out = append(out, Links(e)...)
		}
		return out
	case []string:
		for _, e := range x {
			out = append(out, Links(e)...)
		}
		return out
	}
	for _, m := range linkRe.FindAllStringSubmatch(Str(v), -1) {
		out = append(out, Link{m[1], strings.TrimPrefix(m[2], "^")})
	}
	return out
}

// Names returns the documents that the wikilinks in a value name.
func Names(v any) []string {
	var out []string
	for _, l := range Links(v) {
		out = append(out, l.Name)
	}
	return out
}

// First returns the first name a value links, or "".
func First(v any) string {
	if n := Names(v); len(n) > 0 {
		return n[0]
	}
	return ""
}

// Fields reads the inline fields [key:: value] of a text. A later field wins.
func Fields(text string) map[string]string {
	out := map[string]string{}
	for _, m := range fieldRe.FindAllStringSubmatch(text, -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	return out
}

// StripFields removes the inline fields and a trailing block id from a line.
func StripFields(text string) string {
	return strings.TrimSpace(blockRe.ReplaceAllString(fieldRe.ReplaceAllString(text, ""), ""))
}

// BlockID returns the trailing block id of a line (^t4 gives "t4").
func BlockID(line string) string {
	m := regexp.MustCompile(`\^(\w+)\s*$`).FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

// Section returns the text of the "## title" section of a body, up to the next "## ".
func Section(body, title string) string {
	return sectionAt(body, "## "+title+"\n", "## ")
}

// Subsection returns the text of a "## title" or "### title" heading, up to the next
// heading of level two or three.
func Subsection(body, title string) string {
	for _, h := range []string{"## ", "### "} {
		if s, ok := find(body, h+title+"\n"); ok {
			return until(s, "## ", "### ")
		}
	}
	return ""
}

func sectionAt(body, head, next string) string {
	s, ok := find(body, head)
	if !ok {
		return ""
	}
	return until(s, next)
}

func find(body, head string) (string, bool) {
	if strings.HasPrefix(body, head) {
		return body[len(head):], true
	}
	if i := strings.Index(body, "\n"+head); i >= 0 {
		return body[i+1+len(head):], true
	}
	return "", false
}

func until(s string, prefixes ...string) string {
	pos := 0
	for pos < len(s) {
		for _, p := range prefixes {
			if strings.HasPrefix(s[pos:], p) {
				return s[:pos]
			}
		}
		nl := strings.IndexByte(s[pos:], '\n')
		if nl < 0 {
			return s
		}
		pos += nl + 1
	}
	return s
}

// Sections returns the "## " sections of a body in order, as (title, text) pairs.
func Sections(body string) [][2]string {
	var out [][2]string
	lines := strings.SplitAfter(body, "\n")
	cur, title := -1, ""
	var b strings.Builder
	flush := func() {
		if cur >= 0 {
			out = append(out, [2]string{title, b.String()})
		}
		b.Reset()
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			flush()
			cur++
			title = strings.TrimSpace(strings.TrimPrefix(l, "## "))
			continue
		}
		if cur >= 0 {
			b.WriteString(l)
		}
	}
	flush()
	return out
}

// TableRows returns the cells of each row of the Markdown tables in a text, header rows
// included and separator rows left out.
func TableRows(text string) [][]string {
	var out [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), " | ")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		out = append(out, cells)
	}
	return out
}

// SplitList splits "a, b, [[c, d]]" on commas that are not inside a wikilink.
func SplitList(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], "[["):
			depth++
			i++
		case strings.HasPrefix(s[i:], "]]"):
			depth--
			i++
		case s[i] == ',' && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" {
		out = append(out, last)
	}
	var kept []string
	for _, x := range out {
		if x != "" {
			kept = append(kept, x)
		}
	}
	return kept
}

// Words returns the first n words of a statement, with wikilinks shown as their names.
func Words(text string, n int) string {
	w := strings.Fields(strings.ReplaceAll(aliasRe.ReplaceAllString(text, "$1"), "`", ""))
	if len(w) <= n {
		return strings.Join(w, " ")
	}
	return strings.Join(w[:n], " ") + " …"
}

// SetFields rewrites top-level frontmatter fields that already exist, in place.
// A list is written as a JSON-style flow list; a text that YAML would misread is quoted.
func SetFields(text string, values map[string]any) string {
	raw, body, ok := split(text)
	if !ok {
		return text
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := strings.Split(raw, "\n")
	for _, k := range keys {
		for i, l := range lines {
			if strings.HasPrefix(l, k+": ") {
				lines[i] = k + ": " + Value(values[k])
			}
		}
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body
}

// Value writes one frontmatter value: a flow list for a list, a quoted text when YAML
// would misread it, else the plain text.
func Value(v any) string {
	switch x := v.(type) {
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = Quote(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case string:
		if x == "" || strings.Contains(x, ": ") || strings.ContainsAny(x[:1], "[{\"#") {
			return Quote(x)
		}
		return x
	default:
		return fmt.Sprint(x)
	}
}

// Quote writes a text as a JSON string, which YAML reads back unchanged.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
