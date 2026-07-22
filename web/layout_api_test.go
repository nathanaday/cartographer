package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLayoutRoundTripAnd404(t *testing.T) {
	cfg := seedStore(t) // registers project p3-edge
	srv := newServer(cfg)

	// PUT a layout
	body, _ := json.Marshal(map[string]map[string]float64{"DataObject": {"x": 12, "y": 34}})
	putReq := httptest.NewRequest(http.MethodPut, "/api/projects/p3-edge/layout", bytes.NewReader(body))
	putRec := httptest.NewRecorder()
	srv.routes().ServeHTTP(putRec, putReq)
	if putRec.Code != 200 {
		t.Fatalf("PUT status %d: %s", putRec.Code, putRec.Body)
	}

	// GET it back
	getRec := httptest.NewRecorder()
	srv.routes().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/projects/p3-edge/layout", nil))
	if getRec.Code != 200 {
		t.Fatalf("GET status %d", getRec.Code)
	}
	var got map[string]struct{ X, Y float64 }
	_ = json.Unmarshal(getRec.Body.Bytes(), &got)
	if got["DataObject"].X != 12 || got["DataObject"].Y != 34 {
		t.Fatalf("round-trip: %+v", got)
	}
}

func TestLayoutUnknownProject404(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/projects/nope/layout", nil))
	if rec.Code != 404 {
		t.Fatalf("want 404 for unknown project, got %d", rec.Code)
	}
}

func TestLayoutPutUnknownProject404(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	body, _ := json.Marshal(map[string]map[string]float64{"X": {"x": 1, "y": 2}})
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/projects/nope/layout", bytes.NewReader(body)))
	if rec.Code != 404 {
		t.Fatalf("want 404 for unknown project, got %d", rec.Code)
	}
}

func TestLayoutPutBadBody400(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/projects/p3-edge/layout", bytes.NewReader([]byte("not json"))))
	if rec.Code != 400 {
		t.Fatalf("want 400 for bad body, got %d", rec.Code)
	}
}
