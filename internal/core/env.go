package core

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nathanaday/cartographer/internal/md"
)

// Env is what the core needs to know beyond the documents: the clock, the history of
// each repository, the runner's claims, and the user's last visit.
type Env interface {
	Now() time.Time
	LastVisit() time.Time
	NextSweep() time.Time
	Claims() []Claim
	// OnDefault reports whether a result at commit on ref counts for the default branch.
	// landed reports whether a goal's Log records that repo landed from ref.
	OnDefault(repo, commit, ref string, landed func(repo, ref string) bool) bool
	// ChangedSince returns the commits on the default branch after the result's commit
	// (or time) that touch path.
	ChangedSince(repo, path string, r *Result, landed func(repo, ref string) bool) []string
	// BranchChangedSince returns the commits on ref after the result that touch path.
	BranchChangedSince(repo, ref, path string, r *Result) []string
}

// Claim is the runner's record of an agent at work on a task.
type Claim struct {
	Goal    string `yaml:"goal" json:"goal"`
	Task    string `yaml:"task" json:"task"`
	Session string `yaml:"session" json:"session"`
	Repo    string `yaml:"repo" json:"repo"`
	Log     string `yaml:"log" json:"log"`
}

// Commit is one commit of a simulated repository.
type Commit struct {
	SHA     string    `yaml:"sha"`
	DateRaw string    `yaml:"date"`
	Ref     string    `yaml:"ref"`
	Paths   []string  `yaml:"paths"`
	Date    time.Time `yaml:"-"`
}

// Sim is an Env read from a simulation file. It stands in for git, the claims, and the
// clock in tests and in the design example, with the same approximations as the
// prototype: commits are ordered by date, and a commit is on the default branch when it
// has no ref or its branch landed.
type Sim struct {
	NowRaw       string              `yaml:"now"`
	LastVisitRaw string              `yaml:"last_visit"`
	NextSweepRaw string              `yaml:"next_sweep"`
	Commits      map[string][]Commit `yaml:"commits"`
	ClaimList    []Claim             `yaml:"claims"`
	Default      string              `yaml:"-"`

	NowAt, LastVisitAt, NextSweepAt time.Time `yaml:"-"`
}

// LoadSim reads a simulation file.
func LoadSim(path string) (*Sim, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &Sim{Default: "main"}
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, err
	}
	s.NowAt, s.LastVisitAt, s.NextSweepAt = md.Time(s.NowRaw), md.Time(s.LastVisitRaw), md.Time(s.NextSweepRaw)
	for repo, cs := range s.Commits {
		for i := range cs {
			cs[i].Date = md.Time(cs[i].DateRaw)
		}
		s.Commits[repo] = cs
	}
	return s, nil
}

func (s *Sim) Now() time.Time       { return s.NowAt }
func (s *Sim) LastVisit() time.Time { return s.LastVisitAt }
func (s *Sim) NextSweep() time.Time { return s.NextSweepAt }
func (s *Sim) Claims() []Claim      { return s.ClaimList }

func (s *Sim) OnDefault(repo, commit, ref string, landed func(repo, ref string) bool) bool {
	return ref == s.Default || landed(repo, ref)
}

func (s *Sim) ChangedSince(repo, path string, r *Result, landed func(repo, ref string) bool) []string {
	var out []string
	for _, c := range s.Commits[repo] {
		ref := c.Ref
		if ref == "" {
			ref = s.Default
		}
		if (ref == s.Default || landed(repo, ref)) && c.Date.After(r.Date) && touches(c.Paths, path) {
			out = append(out, c.SHA)
		}
	}
	return out
}

func (s *Sim) BranchChangedSince(repo, ref, path string, r *Result) []string {
	var out []string
	for _, c := range s.Commits[repo] {
		if c.Ref == ref && c.Date.After(r.Date) && touches(c.Paths, path) {
			out = append(out, c.SHA)
		}
	}
	return out
}

func touches(paths []string, path string) bool {
	p := strings.TrimRight(path, "/")
	for _, x := range paths {
		if strings.HasPrefix(x, p) {
			return true
		}
	}
	return false
}
