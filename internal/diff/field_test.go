package diff

import (
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func TestFieldDiffAcrossSharedKind(t *testing.T) {
	a := model.Entity{Aspects: []model.Aspect{{Kind: "sql", Fields: []model.Field{
		{Name: "id", Type: "INT", Nullable: false},
		{Name: "name", Type: "VARCHAR(255)", Nullable: false},
		{Name: "legacy", Type: "TEXT"},
	}}}}
	b := model.Entity{Aspects: []model.Aspect{{Kind: "sql", Fields: []model.Field{
		{Name: "id", Type: "INT", Nullable: false},
		{Name: "name", Type: "VARCHAR(128)", Nullable: true}, // changed
		{Name: "created_at", Type: "TIMESTAMP"},              // only in B
	}}}}

	res := Entities(a, b, "")
	if len(res.Fields) != 1 || res.Fields[0].Kind != "sql" {
		t.Fatalf("expected one sql field diff, got %+v", res.Fields)
	}
	fd := res.Fields[0]
	if len(fd.OnlyInA) != 1 || fd.OnlyInA[0].Name != "legacy" {
		t.Fatalf("OnlyInA = %+v", fd.OnlyInA)
	}
	if len(fd.OnlyInB) != 1 || fd.OnlyInB[0].Name != "created_at" {
		t.Fatalf("OnlyInB = %+v", fd.OnlyInB)
	}
	if len(fd.Changed) != 1 || fd.Changed[0].Name != "name" {
		t.Fatalf("Changed = %+v", fd.Changed)
	}
}

func TestFieldDiffKindFilter(t *testing.T) {
	a := model.Entity{Aspects: []model.Aspect{
		{Kind: "sql", Fields: []model.Field{{Name: "id", Type: "INT"}}},
		{Kind: "json", Fields: []model.Field{{Name: "id", Type: "number"}}},
	}}
	b := a
	res := Entities(a, b, "json")
	if len(res.Fields) != 1 || res.Fields[0].Kind != "json" {
		t.Fatalf("kind filter should yield only json, got %+v", res.Fields)
	}
}
