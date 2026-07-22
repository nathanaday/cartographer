package store

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/nathanaday/cartographer/internal/model"
)

func projectDir(cfg Config, projectSlug string) string {
	return filepath.Join(cfg.Dir, projectSlug)
}

func entityDir(cfg Config, projectSlug, entitySlug string) string {
	return filepath.Join(projectDir(cfg, projectSlug), entitySlug)
}

// atomicWrite writes data to path via a temp file in the same directory, then
// renames, so an interrupted write cannot leave a partial file.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename to %q: %w", path, err)
	}
	return nil
}

func writeProject(cfg Config, p model.Project) error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}
	return atomicWrite(filepath.Join(projectDir(cfg, p.Slug), "project.yaml"), data)
}

func readProject(cfg Config, projectSlug string) (model.Project, error) {
	data, err := os.ReadFile(filepath.Join(projectDir(cfg, projectSlug), "project.yaml"))
	if err != nil {
		return model.Project{}, fmt.Errorf("read project.yaml: %w", err)
	}
	var p model.Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return model.Project{}, fmt.Errorf("unmarshal project: %w", err)
	}
	return p, nil
}

// writeEntity writes entity.yaml (metadata + fields, no raw) plus one sidecar
// file per aspect holding its raw text.
func writeEntity(cfg Config, projectSlug string, e model.Entity) error {
	dir := entityDir(cfg, projectSlug, Slugify(e.Name))
	data, err := yaml.Marshal(e) // Aspect.Raw has yaml:"-", so raw is excluded
	if err != nil {
		return fmt.Errorf("marshal entity: %w", err)
	}
	if err := atomicWrite(filepath.Join(dir, "entity.yaml"), data); err != nil {
		return err
	}
	for _, a := range e.Aspects {
		if err := atomicWrite(filepath.Join(dir, RawFilename(a.Kind)), []byte(a.Raw)); err != nil {
			return err
		}
	}
	return nil
}

func readEntity(cfg Config, projectSlug, entitySlug string) (model.Entity, error) {
	dir := entityDir(cfg, projectSlug, entitySlug)
	data, err := os.ReadFile(filepath.Join(dir, "entity.yaml"))
	if err != nil {
		return model.Entity{}, fmt.Errorf("read entity.yaml: %w", err)
	}
	var e model.Entity
	if err := yaml.Unmarshal(data, &e); err != nil {
		return model.Entity{}, fmt.Errorf("unmarshal entity: %w", err)
	}
	for i := range e.Aspects {
		raw, err := os.ReadFile(filepath.Join(dir, RawFilename(e.Aspects[i].Kind)))
		if err != nil {
			if os.IsNotExist(err) {
				continue // aspect registered without raw text
			}
			return model.Entity{}, fmt.Errorf("read raw for %q: %w", e.Aspects[i].Kind, err)
		}
		e.Aspects[i].Raw = string(raw)
	}
	return e, nil
}
