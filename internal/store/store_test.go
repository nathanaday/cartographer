package store

import (
	"errors"
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
