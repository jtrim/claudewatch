package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writePromptFile creates dir (and parents) and writes a .claudewatchprompt
// file inside it, returning the file's path.
func writePromptFile(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	path := filepath.Join(dir, ".claudewatchprompt")
	if err := os.WriteFile(path, []byte("prompt body"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

func TestFindPromptFileInStartDirectory(t *testing.T) {
	root := t.TempDir()
	want := writePromptFile(t, root)

	got := findPromptFile(root)

	if got != want {
		t.Errorf("findPromptFile(%q) = %q, want %q", root, got, want)
	}
}

func TestFindPromptFileWalksUpToAncestor(t *testing.T) {
	root := t.TempDir()
	want := writePromptFile(t, root)
	start := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", start, err)
	}

	got := findPromptFile(start)

	if got != want {
		t.Errorf("findPromptFile(%q) = %q, want ancestor prompt %q", start, got, want)
	}
}

func TestFindPromptFileNearestWinsOverAncestor(t *testing.T) {
	root := t.TempDir()
	writePromptFile(t, root) // ancestor prompt that should be shadowed
	start := filepath.Join(root, "a", "b")
	want := writePromptFile(t, start)

	got := findPromptFile(start)

	if got != want {
		t.Errorf("findPromptFile(%q) = %q, want nearest prompt %q", start, got, want)
	}
}

func TestFindPromptFileNoneFound(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", start, err)
	}

	got := findPromptFile(start)

	if got != "" {
		t.Errorf("findPromptFile(%q) = %q, want empty string", start, got)
	}
}

func TestFindPromptFileSkipsDirectoryNamedPrompt(t *testing.T) {
	root := t.TempDir()
	// A directory (not a file) named .claudewatchprompt must be skipped so the
	// search continues up to a real prompt file.
	decoy := filepath.Join(root, "a", ".claudewatchprompt")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", decoy, err)
	}
	want := writePromptFile(t, root)
	start := filepath.Join(root, "a")

	got := findPromptFile(start)

	if got != want {
		t.Errorf("findPromptFile(%q) = %q, want %q (directory decoy should be skipped)", start, got, want)
	}
}
