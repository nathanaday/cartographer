// Package templates writes new entity documents in the format of Specification 9.3, for
// the side panel's New commands.
package templates

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/nathanaday/cartographer/internal/md"
)

// Folders maps each kind the side panel creates to its folder in the project.
var Folders = map[string]string{
	"system":    "Systems",
	"service":   "Services",
	"module":    "Modules",
	"interface": "Interfaces",
	"contract":  "Contracts",
}

var prefixes = map[string]string{"system": "sys-", "service": "svc-", "module": "mod-", "interface": "ifc-", "contract": "con-"}

// Fields are what the user gives for a new entity. Only the fields of its kind are used.
type Fields struct {
	Description string
	Project     string // system
	Code        string // system, contract
	Systems     []string
	Repository  string // service, module
	Path        string // service, module
}

func id(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return prefix + string(b)
}

func links(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "[[" + n + "]]"
	}
	return out
}

// Text returns a new document of a kind, and its path relative to the project folder.
func Text(kind, name string, f Fields, now time.Time) (string, string, error) {
	folder, ok := Folders[kind]
	if !ok {
		return "", "", fmt.Errorf("the side panel makes systems, services, modules, interfaces, and contracts, not %q", kind)
	}
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, `\/:*?"<>|#^[]`) {
		return "", "", fmt.Errorf("%q is not a document name", name)
	}
	desc := strings.TrimSpace(f.Description)
	if desc == "" {
		return "", "", fmt.Errorf("a new %s needs a description of one sentence", kind)
	}
	if !strings.HasSuffix(desc, ".") {
		desc += "."
	}
	ts := now.Format("2006-01-02T15:04:05")
	fm := [][2]any{{"id", id(prefixes[kind])}, {"type", "carto-" + kind}, {"description", desc}, {"tags", []string{}}, {"created", ts}, {"updated", ts}}
	var body []string
	switch kind {
	case "system":
		if f.Code == "" {
			return "", "", fmt.Errorf("a new system needs a code, such as SHOP")
		}
		fm = append(fm, [2]any{"code", strings.ToUpper(f.Code)}, [2]any{"project", "[[" + f.Project + "]]"}, [2]any{"adopts", []string{}},
			[2]any{"compliance", ""}, [2]any{"rollup", ""}, [2]any{"violated", 0})
		body = []string{"## Purpose", "", "## Requirements", "", "## Services", "", "```cartographer-children", "```", "", "## Obligations", "", "```cartographer-obligations", "```", "", "## Waivers", "", "## Notes"}
	case "service":
		if f.Repository == "" || f.Path == "" {
			return "", "", fmt.Errorf("a new service needs its repository and its path")
		}
		fm = append(fm, [2]any{"systems", links(f.Systems)}, [2]any{"repository", f.Repository}, [2]any{"path", f.Path}, [2]any{"entry", []string{}},
			[2]any{"runtime", ""}, [2]any{"deploy", ""}, [2]any{"interfaces", []string{}}, [2]any{"uses", []string{}}, [2]any{"stores", []string{}},
			[2]any{"adopts", []string{}}, [2]any{"compliance", ""}, [2]any{"violated", 0}, [2]any{"checked", ""})
		body = []string{"## Purpose", "", "## Requirements", "", "## Obligations", "", "```cartographer-obligations", "```", "", "## Waivers", "", "## Work", "", "```cartographer-work", "```", "", "## Notes"}
	case "module":
		if f.Repository == "" || f.Path == "" {
			return "", "", fmt.Errorf("a new module needs its repository and its path")
		}
		fm = append(fm, [2]any{"repository", f.Repository}, [2]any{"path", f.Path}, [2]any{"package", ""}, [2]any{"adopts", []string{}},
			[2]any{"compliance", ""}, [2]any{"violated", 0}, [2]any{"checked", ""})
		body = []string{"## Purpose", "", "## Used by", "", "```cartographer-children", "```", "", "## Requirements", "", "## Obligations", "", "```cartographer-obligations", "```", "", "## Waivers", "", "## Notes"}
	case "interface":
		fm = append(fm, [2]any{"kind", ""}, [2]any{"definition", ""}, [2]any{"stability", "stable"}, [2]any{"schemas", []string{}}, [2]any{"external", ""})
		body = []string{"## Purpose", "", "## Wire", "", "| | |", "|---|---|", "| Transport | |", "| Address | |", "| Messages | |", "| Auth | |", "| Delivery | |", "| Ordering | |", "| Versioning | |", "",
			"## Attached services", "", "```cartographer-attached", "```", "", "## Notes"}
	case "contract":
		if f.Code == "" {
			return "", "", fmt.Errorf("a new contract needs a code, such as SEC")
		}
		fm = append(fm, [2]any{"code", strings.ToUpper(f.Code)}, [2]any{"kind", ""}, [2]any{"definition", ""}, [2]any{"status", "active"})
		body = []string{"## Purpose", "", "## Clauses", "", "## Adopters", "", "```cartographer-adopters", "```", "", "## History"}
	}
	lines := []string{"---"}
	for _, kv := range fm {
		lines = append(lines, kv[0].(string)+": "+md.Value(kv[1]))
	}
	lines = append(lines, "---", "")
	return strings.Join(append(lines, body...), "\n") + "\n", folder + "/" + name + ".md", nil
}
