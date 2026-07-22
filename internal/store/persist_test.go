package store

import (
	"reflect"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func TestEntityRoundTrip(t *testing.T) {
	cfg := Config{Dir: t.TempDir()}
	proj := model.Project{Slug: "edge-app", Name: "Edge App"}
	if err := writeProject(cfg, proj); err != nil {
		t.Fatalf("writeProject: %v", err)
	}
	ent := model.Entity{
		Name:        "SensorReading",
		Description: "a reading",
		Aspects: []model.Aspect{
			{
				Kind:      "sql",
				Raw:       "CREATE TABLE sensor_reading (\n  device_id VARCHAR(64) PRIMARY KEY\n);",
				SourceRef: "db/schema.sql:42",
				Fields: []model.Field{
					{Name: "device_id", Type: "VARCHAR(64)", Nullable: false, Tags: []string{"primary_key"}},
				},
			},
			{
				Kind: "json",
				Raw:  "{\"device_id\": \"abc\"}",
				Fields: []model.Field{
					{Name: "device_id", Type: "string", Nullable: false},
				},
			},
		},
	}
	if err := writeEntity(cfg, proj.Slug, ent); err != nil {
		t.Fatalf("writeEntity: %v", err)
	}

	got, err := readEntity(cfg, proj.Slug, Slugify(ent.Name))
	if err != nil {
		t.Fatalf("readEntity: %v", err)
	}
	if !reflect.DeepEqual(got, ent) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, ent)
	}

	gotProj, err := readProject(cfg, proj.Slug)
	if err != nil {
		t.Fatalf("readProject: %v", err)
	}
	if gotProj != proj {
		t.Fatalf("project round-trip: got %+v want %+v", gotProj, proj)
	}
}
