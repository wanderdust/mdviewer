package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMdFile(t *testing.T) {
	yes := []string{"readme.md", "NOTES.MD", "doc.markdown", "file.mdown", "spec.mkd"}
	for _, name := range yes {
		if !isMdFile(name) {
			t.Errorf("isMdFile(%q) = false, want true", name)
		}
	}

	no := []string{"main.go", "style.css", "data.txt", "noext", "image.png"}
	for _, name := range no {
		if isMdFile(name) {
			t.Errorf("isMdFile(%q) = true, want false", name)
		}
	}
}

func TestSafePath(t *testing.T) {
	// Create a temp directory to use as root.
	root := t.TempDir()

	// Create a file and a subdirectory inside it.
	os.WriteFile(filepath.Join(root, "file.md"), []byte("# hi"), 0644)
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	os.WriteFile(filepath.Join(root, "sub", "nested.md"), []byte("# nested"), 0644)

	t.Run("valid relative path", func(t *testing.T) {
		got, err := safePath(root, "file.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "file.md" {
			t.Errorf("got %q, want basename file.md", got)
		}
	})

	t.Run("nested valid path", func(t *testing.T) {
		got, err := safePath(root, "sub/nested.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "nested.md" {
			t.Errorf("got %q, want basename nested.md", got)
		}
	})

	t.Run("dotdot rejected", func(t *testing.T) {
		_, err := safePath(root, "../etc/passwd")
		if err == nil {
			t.Fatal("expected error for .. path, got nil")
		}
	})

	t.Run("empty path rejected", func(t *testing.T) {
		_, err := safePath(root, "")
		if err == nil {
			t.Fatal("expected error for empty path, got nil")
		}
	})

	t.Run("nonexistent file in valid dir is ok", func(t *testing.T) {
		// safePath allows files that don't exist yet (checks parent).
		_, err := safePath(root, "newfile.md")
		if err != nil {
			t.Fatalf("unexpected error for nonexistent file: %v", err)
		}
	})

	t.Run("symlink inside root is ok", func(t *testing.T) {
		// Create a symlink inside root pointing to another file inside root.
		target := filepath.Join(root, "file.md")
		link := filepath.Join(root, "link.md")
		if err := os.Symlink(target, link); err != nil {
			t.Skip("symlinks not supported:", err)
		}
		got, err := safePath(root, "link.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should resolve to the real file.
		if filepath.Base(got) != "file.md" {
			t.Errorf("got %q, expected resolved to file.md", got)
		}
	})

	t.Run("symlink escaping root is rejected", func(t *testing.T) {
		// Create a symlink that points outside root.
		outside := t.TempDir()
		os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0644)
		link := filepath.Join(root, "escape.md")
		os.Remove(link) // clean up if exists
		if err := os.Symlink(filepath.Join(outside, "secret.md"), link); err != nil {
			t.Skip("symlinks not supported:", err)
		}
		_, err := safePath(root, "escape.md")
		if err == nil {
			t.Fatal("expected error for symlink escaping root, got nil")
		}
	})

	t.Run("root dir itself is valid", func(t *testing.T) {
		// Requesting "." effectively resolves to the root dir.
		got, err := safePath(root, ".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolved, _ := filepath.EvalSymlinks(root)
		if got != resolved {
			t.Errorf("got %q, want %q", got, resolved)
		}
	})
}
