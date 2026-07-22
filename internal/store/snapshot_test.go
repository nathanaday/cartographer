package store

import "testing"

func TestProjectsSorted(t *testing.T) {
	s, _ := Open(Config{Dir: t.TempDir()})
	_ = s.RegisterEntity("edge-app", "SensorReading", "")
	_ = s.RegisterEntity("cloud-app", "SensorReading", "")
	projects := s.Projects()
	if len(projects) != 2 || projects[0].Slug != "cloud-app" || projects[1].Slug != "edge-app" {
		t.Fatalf("projects not sorted by slug: %+v", projects)
	}
}
