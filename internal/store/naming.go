package store

import (
	"regexp"
	"strings"
)

var nonSlugChar = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a display name into a filesystem-safe, lowercase slug:
// non-alphanumeric runs collapse to a single hyphen, and leading/trailing
// hyphens are trimmed.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugChar.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// rawExt maps a kind to the sidecar file extension used for its raw text, so
// artifacts open with correct syntax highlighting. Unknown kinds fall back to
// ".txt".
var rawExt = map[string]string{
	"sql":        "sql",
	"json":       "json",
	"protobuf":   "proto",
	"proto":      "proto",
	"pydantic":   "py",
	"python":     "py",
	"typescript": "ts",
	"cpp_struct": "h",
	"go":         "go",
	"avro":       "avsc",
	"graphql":    "graphql",
	"yaml":       "yaml",
}

// RawFilename returns the sidecar filename for an aspect's raw text, e.g.
// RawFilename("sql") == "sql.raw.sql".
func RawFilename(kind string) string {
	ext, ok := rawExt[kind]
	if !ok {
		ext = "txt"
	}
	return kind + ".raw." + ext
}
