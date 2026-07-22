package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nathanaday/cartographer/internal/model"
	"github.com/nathanaday/cartographer/internal/store"
)

func seedStore(t *testing.T) store.Config {
	t.Helper()
	cfg := store.Config{Dir: t.TempDir()}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.RegisterEntity("p3-edge", "DataObject", "envelope"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.SetAspect("p3-edge", "DataObject", model.Aspect{
		Kind: "protobuf", Raw: "message DataObject {}", SourceRef: "sdk/schema/data_objects.proto",
		Fields: []model.Field{{Name: "track_id", Type: "string"}, {Name: "geometry", Type: "Geometry", References: "Geometry.kind"}},
	}); err != nil {
		t.Fatalf("set aspect: %v", err)
	}
	return cfg
}

func TestHandleMapShape(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/map", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var m mapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Fingerprint == "" {
		t.Fatal("missing fingerprint")
	}
	if len(m.Projects) != 1 || m.Projects[0].Slug != "p3-edge" {
		t.Fatalf("projects: %+v", m.Projects)
	}
	if len(m.Entities) != 1 || m.Entities[0].Name != "DataObject" {
		t.Fatalf("entities: %+v", m.Entities)
	}
	if len(m.Entities[0].Aspects) != 1 || m.Entities[0].Aspects[0].Kind != "protobuf" {
		t.Fatalf("aspects: %+v", m.Entities[0].Aspects)
	}
	// raw text MUST NOT be exposed in the map
	if strings.Contains(rec.Body.String(), "message DataObject") {
		t.Fatal("raw aspect text leaked into /api/map")
	}
	// references are carried so the client can draw edges
	if strings.Contains(rec.Body.String(), "Geometry.kind") == false {
		t.Fatal("field references not carried in /api/map")
	}
}

func TestHandleVersionMatchesMap(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	var v struct {
		Fingerprint string `json:"fingerprint"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Fingerprint == "" {
		t.Fatal("no fingerprint")
	}
}
