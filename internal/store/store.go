package store

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/nathanaday/cartographer/internal/model"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrEntityNotFound  = errors.New("entity not found")
	ErrAspectNotFound  = errors.New("aspect not found")
)

// Store is the in-memory index over the on-disk store. It is safe for
// concurrent use; every mutation writes through to disk before returning.
type Store struct {
	cfg      Config
	mu       sync.Mutex
	projects map[string]model.Project           // keyed by project slug
	entities map[string]map[string]model.Entity // projectSlug -> entityName -> Entity
}

// Open loads the entire store directory into memory. A missing or empty
// directory yields an empty store.
func Open(cfg Config) (*Store, error) {
	s := &Store{
		cfg:      cfg,
		projects: map[string]model.Project{},
		entities: map[string]map[string]model.Entity{},
	}
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read store dir: %w", err)
	}
	for _, pe := range entries {
		if !pe.IsDir() {
			continue
		}
		pslug := pe.Name()
		proj, err := readProject(cfg, pslug)
		if err != nil {
			continue // a directory without project.yaml is not a project
		}
		s.projects[pslug] = proj
		s.entities[pslug] = map[string]model.Entity{}
		ents, err := os.ReadDir(projectDir(cfg, pslug))
		if err != nil {
			return nil, fmt.Errorf("read project dir %q: %w", pslug, err)
		}
		for _, ee := range ents {
			if !ee.IsDir() {
				continue
			}
			ent, err := readEntity(cfg, pslug, ee.Name())
			if err != nil {
				continue
			}
			s.entities[pslug][ent.Name] = ent
		}
	}
	return s, nil
}

func (s *Store) projectSlug(project string) string { return Slugify(project) }

// RegisterEntity upserts an entity shell, creating the project on first use.
// Existing aspects are preserved; only the description is updated.
func (s *Store) RegisterEntity(project, name, description string) error {
	if name == "" {
		return errors.New("entity name must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pslug := s.projectSlug(project)
	if _, ok := s.projects[pslug]; !ok {
		p := model.Project{Slug: pslug, Name: project}
		if err := writeProject(s.cfg, p); err != nil {
			return err
		}
		s.projects[pslug] = p
		s.entities[pslug] = map[string]model.Entity{}
	}
	ent, ok := s.entities[pslug][name]
	if !ok {
		ent = model.Entity{Name: name}
	}
	ent.Description = description
	if err := writeEntity(s.cfg, pslug, ent); err != nil {
		return err
	}
	s.entities[pslug][name] = ent
	return nil
}

// GetEntity returns a copy of the named entity.
func (s *Store) GetEntity(project, name string) (model.Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getEntityLocked(project, name)
}

func (s *Store) getEntityLocked(project, name string) (model.Entity, error) {
	pslug := s.projectSlug(project)
	ents, ok := s.entities[pslug]
	if !ok {
		return model.Entity{}, fmt.Errorf("%q: %w", project, ErrProjectNotFound)
	}
	ent, ok := ents[name]
	if !ok {
		return model.Entity{}, fmt.Errorf("%q: %w", name, ErrEntityNotFound)
	}
	return ent, nil
}
