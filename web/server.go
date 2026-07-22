package main

import (
	"net/http"
	"sync"

	"github.com/nathanaday/cartographer/internal/store"
)

// server holds a cached view of the store, reloaded when the schema fingerprint
// changes. Schema is read-only here; only the layout handler mutates state.
type server struct {
	cfg store.Config

	mu sync.Mutex
	st *store.Store
	fp string
}

func newServer(cfg store.Config) *server {
	return &server{cfg: cfg}
}

// current returns the cached store, reloading it if the schema changed.
func (s *server) current() (*store.Store, string, error) {
	fp, err := fingerprint(s.cfg.Dir)
	if err != nil {
		return nil, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st == nil || fp != s.fp {
		st, err := store.Open(s.cfg)
		if err != nil {
			return nil, "", err
		}
		s.st, s.fp = st, fp
	}
	return s.st, s.fp, nil
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/map", s.handleMap)
	mux.HandleFunc("GET /api/projects/{slug}/layout", s.handleGetLayout)
	mux.HandleFunc("PUT /api/projects/{slug}/layout", s.handlePutLayout)
	mux.Handle("/", s.spaHandler())
	return mux
}
