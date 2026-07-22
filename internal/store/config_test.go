package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUsesEnvOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "custom")
	t.Setenv("CARTOGRAPHER_STORE_DIR", dir)
	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Dir != dir {
		t.Fatalf("Dir = %q, want %q", cfg.Dir, dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Resolve must MkdirAll the store dir: %v", err)
	}
}
