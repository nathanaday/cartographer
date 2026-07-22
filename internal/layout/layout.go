// Package layout persists presentation state (card positions) for the web
// canvas. It is deliberately separate from schema: it reads and writes only
// the per-project dotfile .layout.json and never touches entity or project
// files, so arranging cards can never mutate the source-of-truth schema.
package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nathanaday/cartographer/internal/store"
)

// Position is a card's placement on the canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func layoutPath(cfg store.Config, projectSlug string) string {
	return filepath.Join(cfg.Dir, projectSlug, ".layout.json")
}

// Load reads a project's saved card positions, keyed by entity name. A missing
// file yields an empty map and no error.
func Load(cfg store.Config, projectSlug string) (map[string]Position, error) {
	data, err := os.ReadFile(layoutPath(cfg, projectSlug))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Position{}, nil
		}
		return nil, fmt.Errorf("read layout: %w", err)
	}
	out := map[string]Position{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal layout: %w", err)
	}
	return out, nil
}

// Save writes a project's card positions atomically (temp file + rename).
func Save(cfg store.Config, projectSlug string, positions map[string]Position) error {
	data, err := json.MarshalIndent(positions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal layout: %w", err)
	}
	dir := filepath.Join(cfg.Dir, projectSlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".layout-*.tmp")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, layoutPath(cfg, projectSlug)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename layout: %w", err)
	}
	return nil
}
