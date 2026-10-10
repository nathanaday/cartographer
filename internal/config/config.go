// Package config holds the machine settings: the projects this machine works with, the
// folders to scan for clones, and the clone of each remote.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Config is ~/.cartographer/config.json.
type Config struct {
	Projects []string          `json:"projects"`        // project folders (each holds a carto-project document)
	Roots    []string          `json:"roots"`           // folders to scan for git clones
	Repos    map[string]string `json:"repos"`           // remote -> local clone
	Agent    []string          `json:"agent,omitempty"` // the runner's agent command; {prompt} and {session} are filled in
}

// Home is the folder of the machine settings. CARTO_HOME overrides it.
func Home() string {
	if h := os.Getenv("CARTO_HOME"); h != "" {
		return h
	}
	d, _ := os.UserHomeDir()
	return filepath.Join(d, ".cartographer")
}

// Load reads the machine settings. A missing file is an empty config.
func Load() (*Config, error) {
	c := &Config{Repos: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(Home(), "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if c.Repos == nil {
		c.Repos = map[string]string{}
	}
	return c, nil
}

// Save writes the machine settings.
func (c *Config) Save() error {
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return err
	}
	sort.Strings(c.Projects)
	sort.Strings(c.Roots)
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(filepath.Join(Home(), "config.json"), append(b, '\n'), 0o644)
}

// AddProject records a project folder, once.
func (c *Config) AddProject(folder string) {
	abs, _ := filepath.Abs(folder)
	for _, p := range c.Projects {
		if p == abs {
			return
		}
	}
	c.Projects = append(c.Projects, abs)
}

// AddRoot records a folder to scan, once.
func (c *Config) AddRoot(folder string) {
	abs, _ := filepath.Abs(expand(folder))
	for _, r := range c.Roots {
		if r == abs {
			return
		}
	}
	c.Roots = append(c.Roots, abs)
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}

// Scan finds the git clones under the roots, two levels deep, and records each remote.
func (c *Config) Scan() int {
	n := 0
	for _, root := range c.Roots {
		var dirs []string
		level1, _ := filepath.Glob(filepath.Join(root, "*"))
		level2, _ := filepath.Glob(filepath.Join(root, "*", "*"))
		dirs = append(append(dirs, root), append(level1, level2...)...)
		for _, d := range dirs {
			if st, err := os.Stat(filepath.Join(d, ".git")); err != nil || st == nil {
				continue
			}
			if remote := RemoteOf(d); remote != "" {
				if c.Repos[remote] != d {
					c.Repos[remote] = d
					n++
				}
			}
		}
	}
	return n
}

// RemoteOf returns the normalized origin remote of a clone: host/owner/name.
func RemoteOf(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return Normalize(strings.TrimSpace(string(out)))
}

// Normalize turns git@host:owner/name.git or https://host/owner/name into host/owner/name.
func Normalize(remote string) string {
	r := strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	for _, p := range []string{"https://", "http://", "ssh://", "git://"} {
		r = strings.TrimPrefix(r, p)
	}
	if i := strings.Index(r, "@"); i >= 0 {
		r = r[i+1:]
	}
	if i := strings.Index(r, ":"); i >= 0 && !strings.Contains(r[:i], "/") {
		r = r[:i] + "/" + r[i+1:]
	}
	return r
}

// Clone returns the local clone of a remote, or "".
func (c *Config) Clone(remote string) string {
	return c.Repos[strings.SplitN(remote, "@", 2)[0]]
}

// CloneOfName returns the local clone of a repository by its short name (the last part
// of its remote), or "".
func (c *Config) CloneOfName(name string) string {
	for remote, dir := range c.Repos {
		if filepath.Base(remote) == name {
			return dir
		}
	}
	return ""
}
