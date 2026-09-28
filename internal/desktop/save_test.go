package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestWriteReaderAtomicReplacesAndPreservesOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteReaderAtomic(path, strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("got %q", got)
	}
	if err := WriteReaderAtomic(path, failingReader{}); err == nil {
		t.Fatal("expected failure")
	}
	got, _ = os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("failure changed destination: %q", got)
	}
}

func TestSafeSuggestedName(t *testing.T) {
	if got := SafeSuggestedName(`../bad:name?.txt`, "export.txt"); got != "bad_name_.txt" {
		t.Fatalf("got %q", got)
	}
}
