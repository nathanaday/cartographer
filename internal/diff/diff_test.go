package diff

import (
	"reflect"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
)

func TestPresenceDiff(t *testing.T) {
	a := model.Entity{Aspects: []model.Aspect{{Kind: "sql"}, {Kind: "json"}, {Kind: "protobuf"}}}
	b := model.Entity{Aspects: []model.Aspect{{Kind: "sql"}, {Kind: "json"}}}
	got := Entities(a, b, "").Presence
	want := PresenceDiff{
		OnlyInA: []string{"protobuf"},
		OnlyInB: []string{},
		InBoth:  []string{"json", "sql"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presence = %+v, want %+v", got, want)
	}
}
