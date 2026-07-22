package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPAHandlerServesSomethingAtRoot(t *testing.T) {
	cfg := seedStore(t)
	srv := newServer(cfg)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	// Either the built index.html (200) or the unbuilt notice (200) — never 404.
	if rec.Code != 200 {
		t.Fatalf("root status %d", rec.Code)
	}
	// API routes still win over the SPA catch-all
	apiRec := httptest.NewRecorder()
	srv.routes().ServeHTTP(apiRec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if apiRec.Code != 200 {
		t.Fatalf("api route shadowed by SPA: %d", apiRec.Code)
	}
}
