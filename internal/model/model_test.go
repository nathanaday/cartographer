package model

import (
	"reflect"
	"testing"
)

func TestKindsReturnsSortedKinds(t *testing.T) {
	e := Entity{
		Name: "SensorReading",
		Aspects: []Aspect{
			{Kind: "sql"},
			{Kind: "json"},
			{Kind: "protobuf"},
		},
	}
	got := e.Kinds()
	want := []string{"json", "protobuf", "sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Kinds() = %v, want %v", got, want)
	}
}

func TestAspectLookup(t *testing.T) {
	e := Entity{Aspects: []Aspect{{Kind: "sql", SourceRef: "db/schema.sql:42"}}}
	a, ok := e.Aspect("sql")
	if !ok || a.SourceRef != "db/schema.sql:42" {
		t.Fatalf("Aspect(sql) = %+v, %v", a, ok)
	}
	if _, ok := e.Aspect("json"); ok {
		t.Fatal("Aspect(json) should not be found")
	}
}
