package store

import (
	"errors"
	"strings"
	"testing"
)

func TestRegisterAndGetEntity(t *testing.T) {
	s, err := Open(Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.RegisterEntity("edge-app", "SensorReading", "a reading"); err != nil {
		t.Fatalf("RegisterEntity: %v", err)
	}
	e, err := s.GetEntity("edge-app", "SensorReading")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if e.Name != "SensorReading" || e.Description != "a reading" {
		t.Fatalf("unexpected entity: %+v", e)
	}
}

func TestGetEntityNotFound(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	if _, err := s.GetEntity("nope", "Nope"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("want ErrProjectNotFound, got %v", err)
	}
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	if _, err := s.GetEntity("edge-app", "Ghost"); !errors.Is(err, ErrEntityNotFound) {
		t.Fatalf("want ErrEntityNotFound, got %v", err)
	}
}

func TestRegisterEntitySlugCollision(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	if err := s.RegisterEntity("edge-app", "Sensor Reading", ""); err != nil {
		t.Fatalf("register first: %v", err)
	}
	if err := s.RegisterEntity("edge-app", "Sensor-Reading", ""); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("want collision error, got %v", err)
	}
	if err := s.RegisterEntity("edge-app", "Sensor Reading", "updated"); err != nil {
		t.Fatalf("re-register same name should succeed: %v", err)
	}
}

func TestRegisterEntityRejectsEmptyProject(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	if err := s.RegisterEntity("   ", "SensorReading", ""); err == nil {
		t.Fatalf("want error for empty project, got nil")
	}
}

func TestOpenReloadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	s1, _ := Open(Config{Dir: dir})
	if err := s1.RegisterEntity("edge-app", "SensorReading", "a reading"); err != nil {
		t.Fatalf("register: %v", err)
	}
	s2, err := Open(Config{Dir: dir})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	e, err := s2.GetEntity("edge-app", "SensorReading")
	if err != nil || e.Description != "a reading" {
		t.Fatalf("reload failed: %+v err=%v", e, err)
	}
}

func TestHasProject(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	if s.HasProject("edge-app") {
		t.Fatalf("want false for unknown project")
	}
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	if !s.HasProject("edge-app") {
		t.Fatalf("want true after register")
	}
}

func TestGetEntityReturnsIsolatedAspects(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	_ = s.SetAspect("edge-app", "SensorReading", sqlAspect("v1"))

	e1, err := s.GetEntity("edge-app", "SensorReading")
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}

	// A later store-side mutation (in-place replace of the same aspect kind)
	// must not retroactively change the earlier snapshot's backing array.
	if err := s.SetAspect("edge-app", "SensorReading", sqlAspect("v2")); err != nil {
		t.Fatalf("set v2: %v", err)
	}

	a, ok := e1.Aspect("sql")
	if !ok {
		t.Fatalf("e1 lost its sql aspect")
	}
	if a.Raw != "v1" {
		t.Fatalf("earlier GetEntity snapshot mutated by later store write: raw = %q, want v1", a.Raw)
	}

	e2, err := s.GetEntity("edge-app", "SensorReading")
	if err != nil {
		t.Fatalf("get entity again: %v", err)
	}
	if len(e2.Aspects) != 1 {
		t.Fatalf("store aspects corrupted: got %d, want 1", len(e2.Aspects))
	}
}
