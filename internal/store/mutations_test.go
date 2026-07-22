package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func sqlAspect(raw string) model.Aspect {
	return model.Aspect{Kind: "sql", Raw: raw, Fields: []model.Field{{Name: "id", Type: "INT"}}}
}

func TestSetAspectReplaces(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	if err := s.SetAspect("edge-app", "SensorReading", sqlAspect("v1")); err != nil {
		t.Fatalf("set v1: %v", err)
	}
	if err := s.SetAspect("edge-app", "SensorReading", sqlAspect("v2")); err != nil {
		t.Fatalf("set v2: %v", err)
	}
	e, _ := s.GetEntity("edge-app", "SensorReading")
	if len(e.Aspects) != 1 {
		t.Fatalf("want 1 aspect after replace, got %d", len(e.Aspects))
	}
	a, _ := e.Aspect("sql")
	if a.Raw != "v2" {
		t.Fatalf("raw = %q, want v2", a.Raw)
	}
}

func TestSetAspectRequiresEntity(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	_ = s.RegisterEntity("edge-app", "Other", "") // project must exist so the error is entity-not-found, not project-not-found
	if err := s.SetAspect("edge-app", "Ghost", sqlAspect("x")); !errors.Is(err, ErrEntityNotFound) {
		t.Fatalf("want ErrEntityNotFound, got %v", err)
	}
}

func TestRemoveAspectDeletesSidecar(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(Config{Dir: dir})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	_ = s.SetAspect("edge-app", "SensorReading", sqlAspect("v1"))
	sidecar := filepath.Join(dir, "edge-app", "sensorreading", "sql.raw.sql")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("sidecar should exist: %v", err)
	}
	if err := s.RemoveAspect("edge-app", "SensorReading", "sql"); err != nil {
		t.Fatalf("remove aspect: %v", err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar should be gone, stat err = %v", err)
	}
	if err := s.RemoveAspect("edge-app", "SensorReading", "sql"); !errors.Is(err, ErrAspectNotFound) {
		t.Fatalf("want ErrAspectNotFound, got %v", err)
	}
}

func TestRemoveEntity(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(Config{Dir: dir})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	if err := s.RemoveEntity("edge-app", "SensorReading"); err != nil {
		t.Fatalf("remove entity: %v", err)
	}
	if _, err := s.GetEntity("edge-app", "SensorReading"); !errors.Is(err, ErrEntityNotFound) {
		t.Fatalf("want ErrEntityNotFound, got %v", err)
	}
	entDir := filepath.Join(dir, "edge-app", "sensorreading")
	if _, err := os.Stat(entDir); !os.IsNotExist(err) {
		t.Fatalf("entity dir should be gone, stat err = %v", err)
	}
}
