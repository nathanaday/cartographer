package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config locates the on-disk store.
type Config struct {
	Dir string
}

// Resolve determines the store directory from CARTOGRAPHER_STORE_DIR (falling
// back to ~/.cartographer/store) and ensures it exists.
func Resolve() (Config, error) {
	dir := os.Getenv("CARTOGRAPHER_STORE_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".cartographer", "store")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create store dir %q: %w", dir, err)
	}
	return Config{Dir: dir}, nil
}
