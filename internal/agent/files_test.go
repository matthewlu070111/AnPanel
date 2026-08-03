package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeUploadName(t *testing.T) {
	ok, err := sanitizeUploadName("hello.txt")
	if err != nil || ok != "hello.txt" {
		t.Fatalf("got %q %v", ok, err)
	}
	// Path components collapse to basename (safe: cannot escape via upload name).
	ok, err = sanitizeUploadName("../etc/passwd")
	if err != nil || ok != "passwd" {
		t.Fatalf("traversal should collapse to basename, got %q %v", ok, err)
	}
	if _, err := sanitizeUploadName(".."); err == nil {
		t.Fatal("expected reject ..")
	}
	if _, err := sanitizeUploadName(""); err == nil {
		t.Fatal("expected reject empty")
	}
}

func TestSaveUploadedFileAndCopy(t *testing.T) {
	root := t.TempDir()
	// Point allowlist at temp root for this test process.
	old := fileRoots
	fileRoots = []string{root}
	t.Cleanup(func() { fileRoots = old })

	dir := filepath.Join(root, "data")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "hello anpanel"
	dst, err := saveUploadedFile(dir, "a.txt", strings.NewReader(content), int64(len(content)), false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || string(b) != content {
		t.Fatalf("read back: %q %v", b, err)
	}
	// No overwrite by default
	if _, err := saveUploadedFile(dir, "a.txt", strings.NewReader("x"), 1, false); err == nil {
		t.Fatal("expected exists error")
	}
	if _, err := saveUploadedFile(dir, "a.txt", strings.NewReader("overwrite"), 9, true); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dir, "a.txt")
	copyTo := filepath.Join(dir, "b.txt")
	if _, err := copyPath(src, copyTo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyTo); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(dir, "c.txt")
	if _, err := movePath(copyTo, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyTo); !os.IsNotExist(err) {
		t.Fatal("source should be gone after move")
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatal(err)
	}
}
