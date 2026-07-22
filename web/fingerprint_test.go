package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFingerprintChangesOnSchemaChangeNotLayout(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "edge-app")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "project.yaml"), []byte("slug: edge-app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fp1, err := fingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}

	// writing a layout dotfile must NOT change the fingerprint
	if err := os.WriteFile(filepath.Join(proj, ".layout.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp2, _ := fingerprint(dir)
	if fp2 != fp1 {
		t.Fatalf("layout write changed fingerprint: %q -> %q", fp1, fp2)
	}

	// changing a schema file MUST change the fingerprint
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(proj, "project.yaml"), []byte("slug: edge-app\nname: Edge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp3, _ := fingerprint(dir)
	if fp3 == fp1 {
		t.Fatalf("schema change did not change fingerprint: %q", fp3)
	}
}

func TestFingerprintStableWhenNothingChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := fingerprint(dir)
	b, _ := fingerprint(dir)
	if a != b {
		t.Fatalf("unstable fingerprint: %q != %q", a, b)
	}
}
