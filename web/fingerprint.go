package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// fingerprint hashes the schema files under dir into a stable string. Any path
// element beginning with "." is skipped, so presentation dotfiles like
// .layout.json never affect it — a card drag must not read as a schema change.
func fingerprint(dir string) (string, error) {
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if strings.HasPrefix(part, ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		lines = append(lines, fmt.Sprintf("%s:%d:%d", rel, info.ModTime().UnixNano(), info.Size()))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk store: %w", err)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return fmt.Sprintf("%x", sum[:8]), nil
}
