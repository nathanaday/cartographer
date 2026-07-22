package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func TestEntityContractMatchesGolden(t *testing.T) {
	e := model.Entity{
		Name:        "SensorReading",
		Description: "a device reading",
		Aspects: []model.Aspect{
			{
				Kind:      "sql",
				SourceRef: "db/schema.sql:42",
				Fields: []model.Field{
					{Name: "device_id", Type: "VARCHAR(64)", Nullable: false, Tags: []string{"primary_key"}},
					{Name: "value", Type: "DOUBLE", Nullable: true},
				},
			},
		},
	}
	got, err := Entity(e)
	if err != nil {
		t.Fatalf("Entity: %v", err)
	}
	golden := filepath.Join("testdata", "sensor_reading.golden.yaml")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("contract mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
