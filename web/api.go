package main

import (
	"encoding/json"
	"net/http"

	"github.com/nathanaday/cartographer/internal/model"
)

type projectDTO struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type fieldDTO struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Nullable   bool     `json:"nullable"`
	Tags       []string `json:"tags,omitempty"`
	References string   `json:"references,omitempty"`
}

type aspectDTO struct {
	Kind      string     `json:"kind"`
	SourceRef string     `json:"source_ref,omitempty"`
	Fields    []fieldDTO `json:"fields"`
}

type entityDTO struct {
	Project     string      `json:"project"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Aspects     []aspectDTO `json:"aspects"`
}

type mapResponse struct {
	Fingerprint string       `json:"fingerprint"`
	Projects    []projectDTO `json:"projects"`
	Entities    []entityDTO  `json:"entities"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func aspectToDTO(a model.Aspect) aspectDTO {
	fields := make([]fieldDTO, 0, len(a.Fields))
	for _, f := range a.Fields {
		fields = append(fields, fieldDTO{
			Name: f.Name, Type: f.Type, Nullable: f.Nullable,
			Tags: f.Tags, References: f.References,
		})
	}
	return aspectDTO{Kind: a.Kind, SourceRef: a.SourceRef, Fields: fields}
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	_, fp, err := s.current()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"fingerprint": fp})
}

func (s *server) handleMap(w http.ResponseWriter, _ *http.Request) {
	st, fp, err := s.current()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	resp := mapResponse{Fingerprint: fp, Projects: []projectDTO{}, Entities: []entityDTO{}}
	for _, p := range st.Projects() {
		resp.Projects = append(resp.Projects, projectDTO{Slug: p.Slug, Name: p.Name})
	}
	for _, sum := range st.ListEntities("", "") {
		ent, err := st.GetEntity(sum.Project, sum.Name)
		if err != nil {
			continue
		}
		aspects := make([]aspectDTO, 0, len(ent.Aspects))
		for _, a := range ent.Aspects {
			aspects = append(aspects, aspectToDTO(a)) // excludes Raw
		}
		resp.Entities = append(resp.Entities, entityDTO{
			Project: sum.Project, Name: ent.Name, Description: ent.Description, Aspects: aspects,
		})
	}
	writeJSON(w, 200, resp)
}
