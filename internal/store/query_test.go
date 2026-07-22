package store

import (
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func seed(t *testing.T) *Store {
	t.Helper()
	s, _ := Open(Config{Dir: t.TempDir()})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	_ = s.SetAspect("edge-app", "SensorReading", model.Aspect{Kind: "sql"})
	_ = s.SetAspect("edge-app", "SensorReading", model.Aspect{Kind: "protobuf"})
	_ = s.RegisterEntity("edge-app", "DeviceConfig", "")
	_ = s.SetAspect("edge-app", "DeviceConfig", model.Aspect{Kind: "json"})
	_ = s.RegisterEntity("cloud-app", "SensorReading", "")
	_ = s.SetAspect("cloud-app", "SensorReading", model.Aspect{Kind: "sql"})
	return s
}

func TestListAll(t *testing.T) {
	s := seed(t)
	if got := len(s.ListEntities("", "")); got != 3 {
		t.Fatalf("list all = %d, want 3", got)
	}
}

func TestListByProject(t *testing.T) {
	s := seed(t)
	if got := len(s.ListEntities("edge-app", "")); got != 2 {
		t.Fatalf("list edge-app = %d, want 2", got)
	}
}

func TestListByKind(t *testing.T) {
	s := seed(t)
	got := s.ListEntities("", "protobuf")
	if len(got) != 1 || got[0].Name != "SensorReading" || got[0].Project != "edge-app" {
		t.Fatalf("list kind=protobuf = %+v, want only edge-app/SensorReading", got)
	}
}
