// Package model holds Cartographer's pure domain types: a Project contains
// Entities, an Entity has one Aspect per representation kind, and an Aspect
// has normalized Fields. These types carry no persistence or transport logic.
package model

import "sort"

// Field is one normalized property of an aspect, as reported by the registrant.
// Type is preserved as-written (e.g. "VARCHAR(255)", "Optional[int]"), never
// coerced to a universal type system.
type Field struct {
	Name       string   `yaml:"name" json:"name"`
	Type       string   `yaml:"type" json:"type"`
	Nullable   bool     `yaml:"nullable" json:"nullable"`
	Tags       []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	References string   `yaml:"references,omitempty" json:"references,omitempty"`
	Note       string   `yaml:"note,omitempty" json:"note,omitempty"`
}

// Aspect is one representation of an entity in a specific kind (sql, json,
// protobuf, ...). Raw is the verbatim source text; it is persisted to a sidecar
// file, not into entity.yaml, hence the yaml:"-" tag.
type Aspect struct {
	Kind      string  `yaml:"kind" json:"kind"`
	Raw       string  `yaml:"-" json:"raw"`
	SourceRef string  `yaml:"source_ref,omitempty" json:"source_ref,omitempty"`
	Fields    []Field `yaml:"fields" json:"fields"`
}

// Entity is a single object mapped in a project. Aspects is a slice (not a map)
// so entity.yaml serializes deterministically; there is at most one aspect per
// kind, enforced by the store.
type Entity struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Aspects     []Aspect `yaml:"aspects" json:"aspects"`
}

// Project is the scope an entity belongs to (e.g. "edge-app").
type Project struct {
	Slug string `yaml:"slug" json:"slug"`
	Name string `yaml:"name" json:"name"`
}

// Kinds returns the aspect kinds present on the entity, sorted.
func (e Entity) Kinds() []string {
	ks := make([]string, 0, len(e.Aspects))
	for _, a := range e.Aspects {
		ks = append(ks, a.Kind)
	}
	sort.Strings(ks)
	return ks
}

// Aspect returns the aspect of the given kind, or false if absent.
func (e Entity) Aspect(kind string) (Aspect, bool) {
	for _, a := range e.Aspects {
		if a.Kind == kind {
			return a, true
		}
	}
	return Aspect{}, false
}
