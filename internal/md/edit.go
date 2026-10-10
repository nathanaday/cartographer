package md

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// List returns a frontmatter list field as texts.
func List(fm map[string]any, field string) []string {
	var out []string
	switch v := fm[field].(type) {
	case []any:
		for _, x := range v {
			out = append(out, Str(x))
		}
	case string:
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func frontmatter(text string) (map[string]any, string, string, error) {
	raw, body, ok := split(text)
	if !ok {
		return nil, "", "", fmt.Errorf("the document has no frontmatter")
	}
	fm := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
		return nil, "", "", err
	}
	return fm, raw, body, nil
}

// SetField sets a top-level frontmatter field, adding it at the end when it is missing.
func SetField(text, field string, value any) (string, error) {
	_, raw, body, err := frontmatter(text)
	if err != nil {
		return "", err
	}
	lines := strings.Split(raw, "\n")
	line := field + ": " + Value(value)
	for i, l := range lines {
		if strings.HasPrefix(l, field+": ") || l == field+":" {
			lines[i] = line
			return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body, nil
		}
	}
	lines = append(lines, line)
	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body, nil
}

// AddToList adds a value to a frontmatter list field, once.
func AddToList(text, field, value string) (string, error) {
	fm, _, _, err := frontmatter(text)
	if err != nil {
		return "", err
	}
	list := List(fm, field)
	for _, x := range list {
		if x == value {
			return text, nil
		}
	}
	return SetField(text, field, append(list, value))
}

// RemoveFromList removes a value from a frontmatter list field.
func RemoveFromList(text, field, value string) (string, error) {
	fm, _, _, err := frontmatter(text)
	if err != nil {
		return "", err
	}
	kept := []string{}
	for _, x := range List(fm, field) {
		if x != value {
			kept = append(kept, x)
		}
	}
	return SetField(text, field, kept)
}

// AppendToSection adds a line at the end of the "## title" section, after its last
// non-blank line. It adds the section at the end of the document when it is missing.
func AppendToSection(text, title, line string) string {
	head := "## " + title + "\n"
	start := -1
	if strings.HasPrefix(text, head) {
		start = 0
	} else if i := strings.Index(text, "\n"+head); i >= 0 {
		start = i + 1
	}
	if start < 0 {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + "\n" + head + "\n" + line + "\n"
	}
	content := start + len(head)
	end := len(text)
	if strings.HasPrefix(text[content:], "## ") {
		end = content
	} else if i := strings.Index(text[content:], "\n## "); i >= 0 {
		end = content + i + 1
	}
	section := text[content:end]
	trimmed := strings.TrimRight(section, "\n")
	rest := section[len(trimmed):]
	if strings.TrimSpace(trimmed) == "" {
		if end < len(text) {
			return text[:content] + "\n" + line + "\n\n" + text[end:]
		}
		return text[:content] + "\n" + line + "\n"
	}
	if rest == "" {
		rest = "\n"
	}
	return text[:content] + trimmed + "\n" + line + rest + text[end:]
}

// ReplaceLine replaces the first line for which match returns true.
func ReplaceLine(text string, match func(string) bool, repl string) (string, bool) {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if match(l) {
			lines[i] = repl
			return strings.Join(lines, "\n"), true
		}
	}
	return text, false
}
