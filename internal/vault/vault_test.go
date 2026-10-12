package vault

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Writers in parallel each read a file, add a line, and commit. The lock makes each
// read-modify-write one step, so no line is lost.
func TestParallelWritersLoseNothing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "log.md")
	os.WriteFile(path, []byte("start\n"), 0o644)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for k := 0; k < 5; k++ {
				w := NewWriter(root)
				text, err := w.Read(path)
				if err != nil {
					t.Error(err)
					return
				}
				w.Put(path, text+"line\n")
				if _, err := w.Commit("append"); err != nil {
					t.Error(err)
				}
			}
		}(i)
	}
	wg.Wait()
	b, _ := os.ReadFile(path)
	if n := strings.Count(string(b), "line\n"); n != 40 {
		t.Fatalf("%d of 40 lines survived", n)
	}
}

// A scratch writer never takes the lock, so it cannot block a real writer.
func TestScratchTakesNoLock(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.md")
	os.WriteFile(path, []byte("a"), 0o644)
	s := NewScratch(root)
	s.Read(path)
	w := NewWriter(root)
	if _, err := w.Read(path); err != nil {
		t.Fatalf("a scratch writer blocked a real one: %v", err)
	}
	w.Abort()
}

// A commit records only its own paths, even when the user has other files staged, and
// commits the ignore file of .carto the first time.
func TestCommitTakesOnlyItsPaths(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		out, err := git(root, args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return out
	}
	run("init", "-q")
	run("config", "user.name", "t")
	run("config", "user.email", "t@t")
	os.WriteFile(filepath.Join(root, "a.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(root, "b.md"), []byte("b"), 0o644)
	run("add", "-A")
	run("commit", "-qm", "init")
	os.WriteFile(filepath.Join(root, "b.md"), []byte("staged by the user"), 0o644)
	run("add", "b.md")

	w := NewWriter(root)
	w.Put(filepath.Join(root, "a.md"), "a")
	if _, err := w.Commit("no change"); err != nil {
		t.Fatalf("a commit with nothing of its own to record: %v", err)
	}
	w = NewWriter(root)
	w.Put(filepath.Join(root, "a.md"), "a2")
	if _, err := w.Commit("change a"); err != nil {
		t.Fatal(err)
	}
	files := run("show", "--name-only", "--format=", "HEAD")
	if strings.Contains(files, "b.md") || !strings.Contains(files, "a.md") {
		t.Fatalf("the commit holds:\n%s", files)
	}
	if tree := run("ls-tree", "-r", "--name-only", "HEAD"); !strings.Contains(tree, ".carto/.gitignore") || strings.Contains(run("log", "-p", "--format="), "staged by the user") {
		t.Fatalf("HEAD holds:\n%s", tree)
	}
	if !strings.Contains(run("status", "--porcelain"), "M  b.md") {
		t.Fatal("the user's staged file is no longer staged")
	}
}

// A commit waits while another program holds the index lock for a moment.
func TestCommitWaitsForTheIndexLock(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "t"}, {"config", "user.email", "t@t"}} {
		if out, err := git(root, args...); err != nil {
			t.Fatal(out)
		}
	}
	lock := filepath.Join(root, ".git", "index.lock")
	os.WriteFile(lock, nil, 0o644)
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Remove(lock)
	}()
	w := NewWriter(root)
	w.Put(filepath.Join(root, "a.md"), "a")
	if _, err := w.Commit("a"); err != nil {
		t.Fatalf("the commit did not wait for the lock: %v", err)
	}
}

// A snapshot commits the project's documents edited outside an act, new and removed ones
// too, and leaves canvases, runtime state, and other folders alone.
func TestSnapshotCommitsEditsByHand(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		out, err := git(root, args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return out
	}
	run("init", "-q")
	run("config", "user.name", "t")
	run("config", "user.email", "t@t")
	folder := filepath.Join(root, "Acme")
	os.MkdirAll(filepath.Join(folder, "Services"), 0o755)
	for _, f := range []string{"Acme.md", "Services/api.md", "Services/old.md", "Acme.canvas"} {
		os.WriteFile(filepath.Join(folder, f), []byte("v1"), 0o644)
	}
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("v1"), 0o644)
	run("add", "-A")
	run("commit", "-qm", "init")
	if n, err := Snapshot(root, folder); n != 0 || err != nil {
		t.Fatalf("a snapshot of a clean vault: %d %v", n, err)
	}

	os.WriteFile(filepath.Join(folder, "Acme.md"), []byte("v2"), 0o644)
	os.WriteFile(filepath.Join(folder, "Services", "web.md"), []byte("new"), 0o644)
	os.Remove(filepath.Join(folder, "Services", "old.md"))
	os.WriteFile(filepath.Join(folder, "Acme.canvas"), []byte("v2"), 0o644)
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("v2"), 0o644)
	n, err := Snapshot(root, folder)
	if err != nil || n != 3 {
		t.Fatalf("snapshot: %d %v", n, err)
	}
	if msg := run("log", "-1", "--format=%s"); strings.TrimSpace(msg) != "carto: 3 files edited by hand" {
		t.Fatalf("the message: %s", msg)
	}
	status := run("status", "--porcelain")
	if strings.Contains(status, "Acme/") && !strings.Contains(status, "Acme/Acme.canvas") || !strings.Contains(status, "notes.md") || strings.Count(status, "\n") != 2 {
		t.Fatalf("after the snapshot:\n%s", status)
	}
}
