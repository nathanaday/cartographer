// Package contract exports an entity as a stable YAML authority document that
// other repositories can consume. Raw aspect text is deliberately omitted — the
// contract is the normalized shape, not a copy of source files.
package contract

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/nathanaday/cartographer/internal/model"
)

type aspectDoc struct {
	Kind      string        `yaml:"kind"`
	SourceRef string        `yaml:"source_ref,omitempty"`
	Fields    []model.Field `yaml:"fields"`
}

type entityDoc struct {
	Entity      string      `yaml:"entity"`
	Description string      `yaml:"description,omitempty"`
	Aspects     []aspectDoc `yaml:"aspects"`
}

// Entity renders one entity's authority contract as YAML.
func Entity(e model.Entity) ([]byte, error) {
	doc := entityDoc{Entity: e.Name, Description: e.Description}
	for _, a := range e.Aspects {
		doc.Aspects = append(doc.Aspects, aspectDoc{
			Kind:      a.Kind,
			SourceRef: a.SourceRef,
			Fields:    a.Fields,
		})
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal contract: %w", err)
	}
	return data, nil
}

type projectDoc struct {
	Project  string      `yaml:"project"`
	Entities []entityDoc `yaml:"entities"`
}

// Project renders an authority contract for a whole project.
func Project(name string, entities []model.Entity) ([]byte, error) {
	doc := projectDoc{Project: name}
	for _, e := range entities {
		ed := entityDoc{Entity: e.Name, Description: e.Description}
		for _, a := range e.Aspects {
			ed.Aspects = append(ed.Aspects, aspectDoc{Kind: a.Kind, SourceRef: a.SourceRef, Fields: a.Fields})
		}
		doc.Entities = append(doc.Entities, ed)
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal project contract: %w", err)
	}
	return data, nil
}
