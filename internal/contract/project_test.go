package contract

import (
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func TestProjectContract(t *testing.T) {
	ents := []model.Entity{
		{Name: "SensorReading", Aspects: []model.Aspect{{Kind: "sql", Fields: []model.Field{{Name: "id", Type: "INT"}}}}},
		{Name: "DeviceConfig", Aspects: []model.Aspect{{Kind: "json", Fields: []model.Field{{Name: "ttl", Type: "number"}}}}},
	}
	got, err := Project("edge-app", ents)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "project: edge-app") || !strings.Contains(s, "SensorReading") || !strings.Contains(s, "DeviceConfig") {
		t.Fatalf("project contract missing content:\n%s", s)
	}
}
