package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcher_MdFileTriggersOnChange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# initial"), 0644)

	var called atomic.Int32
	watcher, err := startWatcher(dir, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("startWatcher error: %v", err)
	}
	defer watcher.Close()

	// Modify the markdown file.
	time.Sleep(50 * time.Millisecond) // let watcher settle
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# changed"), 0644)

	// Wait for debounce + some margin.
	time.Sleep(500 * time.Millisecond)

	if called.Load() == 0 {
		t.Error("expected onChange to fire for .md file change")
	}
}

func TestWatcher_NonMdFileIgnored(t *testing.T) {
	dir := t.TempDir()

	var called atomic.Int32
	watcher, err := startWatcher(dir, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("startWatcher error: %v", err)
	}
	defer watcher.Close()

	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644)

	time.Sleep(500 * time.Millisecond)

	if called.Load() != 0 {
		t.Error("onChange should not fire for non-markdown files")
	}
}

func TestWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# v1"), 0644)

	var called atomic.Int32
	watcher, err := startWatcher(dir, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("startWatcher error: %v", err)
	}
	defer watcher.Close()

	time.Sleep(50 * time.Millisecond)

	// Rapid-fire writes should coalesce into one onChange call.
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, "test.md"), []byte("# v"+string(rune('2'+i))), 0644)
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	count := called.Load()
	if count != 1 {
		t.Errorf("expected 1 debounced onChange call, got %d", count)
	}
}

func TestWatcher_CloseStopsCleanly(t *testing.T) {
	dir := t.TempDir()

	watcher, err := startWatcher(dir, func() {})
	if err != nil {
		t.Fatalf("startWatcher error: %v", err)
	}

	// Should not panic or hang.
	err = watcher.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
