package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startE2E creates a temp dir, starts a full server (watcher + hub + HTTP),
// and returns the httptest server and the temp dir path.
func startE2E(t *testing.T, files map[string]string) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	hub := NewHub()

	watcher, err := startWatcher(dir, func() {
		hub.Broadcast("reload")
	})
	if err != nil {
		t.Fatalf("startWatcher error: %v", err)
	}
	t.Cleanup(func() { watcher.Close() })

	mdFiles, _ := listMdFiles(dir)
	initial := ""
	if len(mdFiles) > 0 {
		initial = mdFiles[0]
	}

	server, listener, err := setupServer("127.0.0.1:0", initial, dir, hub)
	if err != nil {
		t.Fatalf("setupServer error: %v", err)
	}

	ts := &httptest.Server{
		Listener: listener,
		Config:   server,
	}
	ts.Start()
	t.Cleanup(ts.Close)

	return ts, dir
}

// dialWS connects a WebSocket client to the test server.
func dialWS(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readWSMessage reads one message with a timeout. Returns "" if timeout.
func readWSMessage(conn *websocket.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return ""
	}
	return string(msg)
}

// ── Tests ───────────────────────────────────────────────────────────

func TestE2E_StartupWithDirectory(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{
		"charlie.md": "# C",
		"alpha.md":   "# A",
		"bravo.md":   "# B",
	})

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	var files []string
	json.NewDecoder(resp.Body).Decode(&files)

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
	if files[0] != "alpha.md" || files[1] != "bravo.md" || files[2] != "charlie.md" {
		t.Errorf("expected sorted [alpha bravo charlie], got %v", files)
	}
}

func TestE2E_StartupWithSingleFile(t *testing.T) {
	// Start a server where the initial file is explicitly set to bravo.md.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("# A"), 0644)
	os.WriteFile(filepath.Join(dir, "bravo.md"), []byte("# B"), 0644)

	hub := NewHub()
	server, listener, err := setupServer("127.0.0.1:0", "bravo.md", dir, hub)
	if err != nil {
		t.Fatalf("setupServer error: %v", err)
	}
	ts := &httptest.Server{Listener: listener, Config: server}
	ts.Start()
	defer ts.Close()

	// Default render (no ?file=) should return bravo.
	resp, err := http.Get(ts.URL + "/api/render")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "B") {
		t.Error("expected default to render bravo.md")
	}

	// /api/files should still list both siblings.
	resp2, err2 := http.Get(ts.URL + "/api/files")
	if err2 != nil {
		t.Fatalf("GET /api/files error: %v", err2)
	}
	defer resp2.Body.Close()
	var files []string
	json.NewDecoder(resp2.Body).Decode(&files)
	if len(files) != 2 {
		t.Errorf("expected 2 sibling files, got %d", len(files))
	}
}

func TestE2E_RenderCycle(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{
		"doc.md": "# Title\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n- [x] done\n\n```go\nfmt.Println()\n```",
	})

	resp, err := http.Get(ts.URL + "/api/render?file=doc.md")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	checks := []struct {
		desc, substr string
	}{
		{"heading", "<h1"},
		{"table", "<table>"},
		{"checkbox", `type="checkbox"`},
		{"code block", "language-go"},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.substr) {
			t.Errorf("render missing %s (expected %q in output)", c.desc, c.substr)
		}
	}
}

func TestE2E_FileNav(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{
		"first.md":  "# First",
		"second.md": "# Second",
	})

	// Render first file.
	resp1, _ := http.Get(ts.URL + "/api/render?file=first.md")
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	if !strings.Contains(string(body1), "First") {
		t.Error("expected First in first.md render")
	}

	// Switch to second file.
	resp2, _ := http.Get(ts.URL + "/api/render?file=second.md")
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if !strings.Contains(string(body2), "Second") {
		t.Error("expected Second in second.md render")
	}
}

func TestE2E_LiveReload(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"test.md": "# Original",
	})

	conn := dialWS(t, ts)
	time.Sleep(100 * time.Millisecond) // let watcher settle

	// Modify the file.
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# Modified"), 0644)

	msg := readWSMessage(conn, 2*time.Second)
	if msg != "reload" {
		t.Errorf("expected 'reload' message, got %q", msg)
	}
}

func TestE2E_LiveReloadIgnoresNonMd(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"test.md": "# ok",
	})

	conn := dialWS(t, ts)
	time.Sleep(100 * time.Millisecond)

	// Write a non-markdown file.
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("hello"), 0644)

	msg := readWSMessage(conn, 500*time.Millisecond)
	if msg != "" {
		t.Errorf("expected no message for .txt change, got %q", msg)
	}
}

func TestE2E_NewFileDiscovery(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"existing.md": "# Existing",
	})

	conn := dialWS(t, ts)
	time.Sleep(100 * time.Millisecond)

	// Create a new markdown file.
	os.WriteFile(filepath.Join(dir, "new.md"), []byte("# New"), 0644)

	msg := readWSMessage(conn, 2*time.Second)
	if msg != "reload" {
		t.Errorf("expected 'reload' for new file, got %q", msg)
	}

	// The new file should appear in the file list.
	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("GET /api/files error: %v", err)
	}
	defer resp.Body.Close()
	var files []string
	json.NewDecoder(resp.Body).Decode(&files)

	found := false
	for _, f := range files {
		if f == "new.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("new.md not in file list: %v", files)
	}
}

func TestE2E_FileDeletionHandling(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"ephemeral.md": "# Will be deleted",
	})

	// Verify it renders first.
	resp1, _ := http.Get(ts.URL + "/api/render?file=ephemeral.md")
	resp1.Body.Close()
	if resp1.StatusCode != 200 {
		t.Fatalf("expected 200 before deletion, got %d", resp1.StatusCode)
	}

	// Delete the file.
	os.Remove(filepath.Join(dir, "ephemeral.md"))

	// Should return 404 now.
	resp2, _ := http.Get(ts.URL + "/api/render?file=ephemeral.md")
	resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Errorf("expected 404 after deletion, got %d", resp2.StatusCode)
	}
}

func TestE2E_ImageServing(t *testing.T) {
	// A real PNG starts with the 8-byte magic header.
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	ts, dir := startE2E(t, map[string]string{
		"readme.md": "# ok",
	})

	os.WriteFile(filepath.Join(dir, "logo.png"), pngHeader, 0644)

	resp2, err := http.Get(ts.URL + "/file/logo.png")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Errorf("expected 200 for image, got %d", resp2.StatusCode)
	}

	ct := resp2.Header.Get("Content-Type")
	if !strings.Contains(ct, "png") {
		t.Errorf("expected png content-type, got %q", ct)
	}
}

func TestE2E_PathTraversal_Render(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{
		"test.md": "# ok",
	})

	resp, _ := http.Get(ts.URL + "/api/render?file=../../etc/passwd")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal on render, got %d", resp.StatusCode)
	}
}

func TestE2E_PathTraversal_File(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{
		"test.md": "# ok",
	})

	// Use percent-encoded dots so the HTTP client doesn't normalize them away.
	req, _ := http.NewRequest("GET", ts.URL+"/file/x", nil)
	req.URL.RawPath = "/file/..%2F..%2Fetc%2Fpasswd"
	req.URL.Path = "/file/../../etc/passwd"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		t.Error("expected non-200 for path traversal on /file/")
	}
}

func TestE2E_InfoEndpoint(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"notes.md": "# Notes",
	})

	resp, err := http.Get(ts.URL + "/api/info?file=notes.md")
	if err != nil {
		t.Fatalf("GET /api/info error: %v", err)
	}
	defer resp.Body.Close()

	var info map[string]string
	json.NewDecoder(resp.Body).Decode(&info)

	if info["fileName"] != "notes.md" {
		t.Errorf("expected fileName=notes.md, got %q", info["fileName"])
	}
	expectedPath := filepath.Join(dir, "notes.md")
	if info["filePath"] != expectedPath {
		t.Errorf("expected filePath=%q, got %q", expectedPath, info["filePath"])
	}
}

func TestE2E_StaticAssets(t *testing.T) {
	ts, _ := startE2E(t, map[string]string{"test.md": "# ok"})

	tests := []struct {
		path, contentType string
	}{
		{"/static/style.css", "text/css"},
		{"/static/app.js", "application/javascript"},
	}

	for _, tt := range tests {
		resp, err := http.Get(ts.URL + tt.path)
		if err != nil {
			t.Fatalf("GET %s error: %v", tt.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("GET %s: expected 200, got %d", tt.path, resp.StatusCode)
		}
	}
}

func TestE2E_ConcurrentClients(t *testing.T) {
	ts, dir := startE2E(t, map[string]string{
		"test.md": "# Original",
	})

	conn1 := dialWS(t, ts)
	conn2 := dialWS(t, ts)
	time.Sleep(100 * time.Millisecond)

	// Modify file.
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# Changed"), 0644)

	// Both clients should receive the reload.
	for i, conn := range []*websocket.Conn{conn1, conn2} {
		msg := readWSMessage(conn, 2*time.Second)
		if msg != "reload" {
			t.Errorf("client %d: expected 'reload', got %q", i+1, msg)
		}
	}
}
