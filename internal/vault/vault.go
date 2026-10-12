// Package vault is the single writer of a vault: every file Cartographer writes goes
// through it, under a lock, and lands in one commit for each act when the vault is a git
// repository.
package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Root returns the nearest folder at or above path that holds .obsidian, or the parent of
// path when there is none.
func Root(path string) string {
	abs, _ := filepath.Abs(path)
	for d := abs; ; d = filepath.Dir(d) {
		if st, err := os.Stat(filepath.Join(d, ".obsidian")); err == nil && st.IsDir() {
			return d
		}
		if filepath.Dir(d) == d {
			return filepath.Dir(abs)
		}
	}
}

// Writer writes files of one vault. It takes the vault's lock at its first read or write
// and holds it until Commit or Abort, so that a read, a change, and its write are one step
// that no other process can come between.
type Writer struct {
	Root    string
	files   map[string]string // absolute path -> new text
	removed map[string]bool   // absolute path -> remove the file
	order   []string
	scratch bool
	unlock  func()
	err     error
}

// NewWriter starts a set of writes to the vault at root.
func NewWriter(root string) *Writer {
	return &Writer{Root: root, files: map[string]string{}, removed: map[string]bool{}}
}

// NewScratch starts a set of writes that is never committed, to check operations. It
// takes no lock.
func NewScratch(root string) *Writer {
	w := NewWriter(root)
	w.scratch = true
	return w
}

// Hold takes the vault's lock now. Reads and writes take it too.
func (w *Writer) Hold() error {
	if w.scratch || w.unlock != nil || w.err != nil {
		return w.err
	}
	u, err := Lock(w.Root)
	if err != nil {
		w.err = err
		return err
	}
	w.unlock = u
	return nil
}

// Abort releases the lock without writing anything that is not flushed. It does nothing
// after a commit.
func (w *Writer) Abort() {
	if w.unlock != nil {
		w.unlock()
		w.unlock = nil
	}
}

// Remove stages the removal of a file.
func (w *Writer) Remove(path string) {
	w.Hold()
	abs, _ := filepath.Abs(path)
	if _, ok := w.files[abs]; !ok && !w.removed[abs] {
		w.order = append(w.order, abs)
	}
	delete(w.files, abs)
	w.removed[abs] = true
}

// Put stages the text of a file.
func (w *Writer) Put(path, text string) {
	w.Hold()
	abs, _ := filepath.Abs(path)
	if _, ok := w.files[abs]; !ok && !w.removed[abs] {
		w.order = append(w.order, abs)
	}
	delete(w.removed, abs)
	w.files[abs] = text
}

// Read returns the staged text of a file, or its text on disk.
func (w *Writer) Read(path string) (string, error) {
	if err := w.Hold(); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(path)
	if t, ok := w.files[abs]; ok {
		return t, nil
	}
	b, err := os.ReadFile(abs)
	return string(b), err
}

// Paths returns the staged paths, relative to the vault root, sorted.
func (w *Writer) Paths() []string {
	var out []string
	for _, p := range w.order {
		rel, _ := filepath.Rel(w.Root, p)
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

// Flush writes the staged files to disk, under the vault lock, and keeps them staged for
// the commit. A later read of the documents sees them.
func (w *Writer) Flush() error {
	if err := w.Hold(); err != nil {
		return err
	}
	return w.write()
}

func (w *Writer) write() error {
	for _, p := range w.order {
		if w.removed[p] {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		tmp := p + ".carto-tmp"
		if err := os.WriteFile(tmp, []byte(w.files[p]), 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, p); err != nil {
			return err
		}
	}
	return nil
}

// Commit writes the staged files under the vault lock and commits them with message, when
// the vault is a git repository. It returns the commit, or "" when there is no git.
func (w *Writer) Commit(message string) (string, error) {
	defer w.Abort()
	if w.scratch {
		return "", errors.New("a scratch writer does not commit")
	}
	if len(w.order) == 0 {
		return "", w.err
	}
	if err := w.Hold(); err != nil {
		return "", err
	}
	if err := w.write(); err != nil {
		return "", err
	}
	if !IsRepo(w.Root) {
		return "", nil
	}
	var paths []string
	for _, rel := range w.Paths() {
		abs := filepath.Join(w.Root, filepath.FromSlash(rel))
		if w.removed[abs] {
			if _, err := git(w.Root, "ls-files", "--error-unmatch", "--", rel); err != nil {
				continue // never tracked: nothing to record
			}
		}
		paths = append(paths, rel)
	}
	if len(paths) == 0 {
		return Head(w.Root), nil
	}
	if _, err := git(w.Root, "cat-file", "-e", "HEAD:.carto/.gitignore"); err != nil {
		if _, err := os.Stat(filepath.Join(w.Root, ".carto", ".gitignore")); err == nil {
			paths = append(paths, ".carto/.gitignore")
		}
	}
	args := append([]string{"add", "-A", "--"}, paths...)
	if out, err := git(w.Root, args...); err != nil {
		return "", fmt.Errorf("git add: %v: %s", err, out)
	}
	if _, err := git(w.Root, append([]string{"diff", "--cached", "--quiet", "--"}, paths...)...); err == nil {
		return Head(w.Root), nil
	}
	if out, err := git(w.Root, append([]string{"commit", "-q", "-m", "carto: " + message, "--"}, paths...)...); err != nil {
		return "", fmt.Errorf("git commit: %v: %s", err, out)
	}
	return Head(w.Root), nil
}

// IsRepo reports whether the vault is a git repository.
func IsRepo(root string) bool {
	_, err := git(root, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Head returns the vault's current commit.
func Head(root string) string {
	out, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// CommitDirty commits the files among rels (relative to the vault) whose text on disk is
// not the text at HEAD, as they are on disk. It keeps a person's edits apart from the act
// that follows them, so that undoing the act keeps the edits. The caller holds the lock.
func CommitDirty(root string, rels []string, message string) error {
	if !IsRepo(root) || len(rels) == 0 {
		return nil
	}
	var dirty []string
	for _, rel := range rels {
		_, onDisk := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		head, inHead := At(root, "HEAD", rel)
		switch {
		case onDisk != nil && !inHead:
		case onDisk != nil || !inHead:
			dirty = append(dirty, rel)
		default:
			if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil && string(b) != head {
				dirty = append(dirty, rel)
			}
		}
	}
	if len(dirty) == 0 {
		return nil
	}
	if out, err := git(root, append([]string{"add", "-A", "--"}, dirty...)...); err != nil {
		return fmt.Errorf("git add: %v: %s", err, out)
	}
	if out, err := git(root, append([]string{"commit", "-q", "-m", "carto: " + message, "--"}, dirty...)...); err != nil {
		return fmt.Errorf("git commit: %v: %s", err, out)
	}
	return nil
}

// Snapshot commits the Markdown files under folder that changed outside an act: a person's
// edits in Obsidian, or an agent's edits of prose. It returns how many it committed.
func Snapshot(root, folder string) (int, error) {
	if !IsRepo(root) {
		return 0, nil
	}
	unlock, err := Lock(root)
	if err != nil {
		return 0, err
	}
	defer unlock()
	rel, err := filepath.Rel(root, folder)
	if err != nil {
		return 0, err
	}
	out, err := git(root, "--no-optional-locks", "status", "--porcelain", "-z", "--untracked-files=all", "--", filepath.ToSlash(rel))
	if err != nil {
		return 0, fmt.Errorf("git status: %v: %s", err, out)
	}
	var files []string
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		paths := []string{e[3:]}
		if (e[0] == 'R' || e[0] == 'C') && i+1 < len(entries) {
			i++
			paths = append(paths, entries[i]) // the old path of a rename
		}
		for _, path := range paths {
			if strings.HasSuffix(path, ".md") && !strings.HasPrefix(path, ".carto/") {
				files = append(files, path)
			}
		}
	}
	if len(files) == 0 {
		return 0, nil
	}
	n := len(files)
	if _, ok := At(root, "HEAD", ".carto/.gitignore"); !ok {
		files = append(files, ".carto/.gitignore")
	}
	msg := fmt.Sprintf("%d files edited by hand", n)
	if n == 1 {
		msg = "1 file edited by hand"
	}
	return n, CommitDirty(root, files, msg)
}

// IsAncestor reports whether commit a is an ancestor of commit b.
func IsAncestor(root, a, b string) bool {
	_, err := git(root, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// At returns the text of a file at a commit, and false when the file did not exist there.
func At(root, sha, rel string) (string, bool) {
	cmd := exec.Command("git", "-C", root, "show", sha+":./"+rel)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return out.String(), true
}

// git runs git in the vault. Another program's git (an editor, a sync plugin) can hold the
// index lock for a moment; git waits for it to go, for up to five seconds.
func git(dir string, args ...string) (string, error) {
	deadline := time.Now().Add(5 * time.Second)
	for wait := 20 * time.Millisecond; ; wait *= 2 {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		err := cmd.Run()
		if err == nil || !strings.Contains(buf.String(), "index.lock") || time.Now().After(deadline) {
			return buf.String(), err
		}
		time.Sleep(min(wait, 500*time.Millisecond))
	}
}

// Lock takes the vault's write lock, waiting up to ten seconds.
func Lock(root string) (func(), error) {
	dir := filepath.Join(root, ".carto")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ignore := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, os.ErrNotExist) {
		os.WriteFile(ignore, []byte("# Runtime state of this machine. Claims, seen.json, and runner.json are versioned.\nwrite.lock\nprompts/\nsessions/\nworktrees/\nrunner.log\n"), 0o644)
	}
	f, err := os.OpenFile(filepath.Join(dir, "write.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, errors.New("another Cartographer write holds the vault lock")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
