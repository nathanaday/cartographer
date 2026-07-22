package main

import (
	"encoding/json"
	"net/http"

	"github.com/nathanaday/cartographer/internal/layout"
)

func (s *server) handleGetLayout(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	st, _, err := s.current()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !st.HasProject(slug) {
		writeErr(w, 404, "project not found")
		return
	}
	positions, err := layout.Load(s.cfg, slug)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, positions)
}

func (s *server) handlePutLayout(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	st, _, err := s.current()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !st.HasProject(slug) {
		writeErr(w, 404, "project not found")
		return
	}
	var positions map[string]layout.Position
	if err := json.NewDecoder(r.Body).Decode(&positions); err != nil {
		writeErr(w, 400, "invalid layout body")
		return
	}
	if err := layout.Save(s.cfg, slug, positions); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"saved": true})
}
