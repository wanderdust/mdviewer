package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 200 * time.Millisecond

// startWatcher watches dir for changes to markdown files and calls onChange
// after a debounce period. It watches the directory (not individual files) to
// catch atomic save patterns used by many editors (write-to-temp then rename).
func startWatcher(dir string, onChange func()) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	// Also watch subdirectories (but skip hidden ones like .git).
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name != "." && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if path != dir {
			watcher.Add(path)
		}
		return nil
	})

	go watchLoop(watcher, onChange)

	return watcher, nil
}

func watchLoop(watcher *fsnotify.Watcher, onChange func()) {
	// Timer starts stopped. Reset it when we see a relevant file change.
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Pick up newly created directories so their files are watched too.
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

			// Clean up watches for removed directories.
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				watcher.Remove(event.Name)
			}

			// Only care about markdown files for the debounce trigger.
			if !isMdFile(event.Name) {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				// Reset the debounce timer. If multiple changes happen within
				// 200ms (e.g. editor save-and-format), we fire only once.
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounceInterval)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("mdview: watcher error: %v", err)

		case <-timer.C:
			onChange()
		}
	}
}
