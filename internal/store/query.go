package store

import "sort"

// EntitySummary is the list view of an entity: identity plus which kinds it has.
type EntitySummary struct {
	Project     string   `json:"project"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Kinds       []string `json:"kinds"`
}

// ListEntities returns summaries, optionally filtered to one project slug and/or
// to entities that have an aspect of a given kind. Empty filters match all.
// Results are sorted by project then name for determinism.
func (s *Store) ListEntities(projectFilter, kindFilter string) []EntitySummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	pfilter := ""
	if projectFilter != "" {
		pfilter = Slugify(projectFilter)
	}
	out := []EntitySummary{}
	for pslug, ents := range s.entities {
		if pfilter != "" && pslug != pfilter {
			continue
		}
		for _, e := range ents {
			if kindFilter != "" {
				if _, ok := e.Aspect(kindFilter); !ok {
					continue
				}
			}
			out = append(out, EntitySummary{
				Project:     pslug,
				Name:        e.Name,
				Description: e.Description,
				Kinds:       e.Kinds(),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})
	return out
}
