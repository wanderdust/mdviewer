package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// listMdFiles returns sorted markdown filenames in a directory (non-recursive).
func listMdFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isMdFile(e.Name()) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// resolveRequestedFile extracts the ?file= query param, validates it, and
// returns the absolute path and basename. Falls back to defaultFile if the
// param is missing.
func resolveRequestedFile(r *http.Request, rootDir, defaultFile string) (absPath, baseName string, err error) {
	name := r.URL.Query().Get("file")
	if name == "" {
		name = defaultFile
	}
	if name == "" {
		return "", "", fmt.Errorf("no file specified")
	}
	if !isMdFile(name) {
		return "", "", fmt.Errorf("not a markdown file")
	}
	abs, err := safePath(rootDir, name)
	if err != nil {
		return "", "", fmt.Errorf("invalid path")
	}
	return abs, filepath.Base(name), nil
}

// setupServer creates the HTTP server with all routes and returns both the
// server and the bound listener. The listener is separate so main.go can read
// the actual address (important when port is 0 for auto-pick).
func setupServer(addr, initialFile, rootDir string, hub *Hub) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()

	// Serve the single-page app shell.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// Serve embedded static assets (CSS, JS, highlight.js).
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create static sub-fs: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// API: list markdown files.
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		files, err := listMdFiles(rootDir)
		if err != nil {
			http.Error(w, "failed to list files", http.StatusInternalServerError)
			return
		}
		if files == nil {
			files = []string{} // return [] not null
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	// API: render a markdown file to HTML.
	mux.HandleFunc("/api/render", func(w http.ResponseWriter, r *http.Request) {
		absPath, _, err := resolveRequestedFile(r, rootDir, initialFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		html, err := renderMarkdown(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
	})

	// API: file info (name and path for the top bar).
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		absPath, baseName, err := resolveRequestedFile(r, rootDir, initialFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"fileName": baseName,
			"filePath": absPath,
		})
	})

	// Serve raw files from rootDir (for images and other linked assets).
	mux.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, "/file/")
		absPath, err := safePath(rootDir, relPath)
		if err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, absPath)
	})

	// WebSocket for live reload.
	mux.HandleFunc("/ws", HandleWebSocket(hub))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to bind %s: %w", addr, err)
	}

	server := &http.Server{Handler: mux}
	return server, listener, nil
}
