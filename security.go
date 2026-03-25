package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// isMdFile returns true if the file has a recognized markdown extension.
func isMdFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// safePath validates that requestedPath resolves to a location inside rootDir.
// It rejects path traversal attempts and resolves symlinks to verify the real
// destination. Returns the absolute path or an error.
func safePath(rootDir, requestedPath string) (string, error) {
	if requestedPath == "" {
		return "", fmt.Errorf("empty path")
	}

	// Reject obvious traversal patterns before doing any filesystem work.
	if strings.Contains(requestedPath, "..") {
		return "", fmt.Errorf("path contains ..")
	}

	// Build absolute path and resolve symlinks.
	joined := filepath.Join(rootDir, requestedPath)
	absPath, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	// Try to resolve symlinks on the full path first.
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// File might not exist yet — check the parent directory instead.
		parentDir := filepath.Dir(absPath)
		realParent, parentErr := filepath.EvalSymlinks(parentDir)
		if parentErr != nil {
			return "", fmt.Errorf("cannot resolve parent: %w", parentErr)
		}
		realRoot, rootErr := filepath.EvalSymlinks(rootDir)
		if rootErr != nil {
			return "", fmt.Errorf("cannot resolve root: %w", rootErr)
		}
		if !strings.HasPrefix(realParent, realRoot) {
			return "", fmt.Errorf("path outside root directory")
		}
		return absPath, nil
	}

	realRoot, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve root: %w", err)
	}

	// The resolved path must be inside (or equal to) the root.
	if realPath != realRoot && !strings.HasPrefix(realPath, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside root directory")
	}

	return realPath, nil
}
