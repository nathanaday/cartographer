package store

import (
	"sort"

	"github.com/nathanaday/cartographer/internal/model"
)

// Projects returns all projects, sorted by slug.
func (s *Store) Projects() []model.Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Project, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}
