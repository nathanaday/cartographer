package layout

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nathanaday/cartographer/internal/store"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	cfg := store.Config{Dir: t.TempDir()}
	got, err := Load(cfg, "edge-app")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := store.Config{Dir: t.TempDir()}
	// the project directory must exist (the store created it in real use)
	if err := os.MkdirAll(filepath.Join(cfg.Dir, "edge-app"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := map[string]Position{
		"DataObject":  {X: 120, Y: 40},
		"CameraInput": {X: 400.5, Y: -10},
	}
	if err := Save(cfg, "edge-app", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(cfg, "edge-app")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip: got %v want %v", got, want)
	}
	// the layout file must be the dotfile, and must NOT be a schema file
	if _, err := os.Stat(filepath.Join(cfg.Dir, "edge-app", ".layout.json")); err != nil {
		t.Fatalf(".layout.json not written: %v", err)
	}
}
