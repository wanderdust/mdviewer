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
)

// newTestServer creates a server backed by a temp directory with the given files.
// Returns the httptest server, root dir, and a cleanup function.
func newTestServer(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	dir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	hub := NewHub()
	// Use first .md file as initial.
	mdFiles, _ := listMdFiles(dir)
	initial := ""
	if len(mdFiles) > 0 {
		initial = mdFiles[0]
	}

	server, listener, err := setupServer("127.0.0.1:0", initial, dir, hub)
	if err != nil {
		t.Fatalf("setupServer error: %v", err)
	}

	// Use httptest to wrap the listener.
	ts := &httptest.Server{
		Listener: listener,
		Config:   server,
	}
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

func TestAPI_Files(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"bravo.md": "# B",
		"alpha.md": "# A",
		"readme.txt": "not markdown",
	})

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("GET /api/files error: %v", err)
	}
	defer resp.Body.Close()

	var files []string
	json.NewDecoder(resp.Body).Decode(&files)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != "alpha.md" || files[1] != "bravo.md" {
		t.Errorf("expected [alpha.md bravo.md], got %v", files)
	}
}

func TestAPI_Files_Empty(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"main.go": "package main",
	})

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("GET /api/files error: %v", err)
	}
	defer resp.Body.Close()

	var files []string
	json.NewDecoder(resp.Body).Decode(&files)

	if len(files) != 0 {
		t.Errorf("expected empty array, got %v", files)
	}
}

func TestAPI_Render(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# Hello\n\nWorld",
	})

	resp, err := http.Get(ts.URL + "/api/render?file=test.md")
	if err != nil {
		t.Fatalf("GET /api/render error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(html, "<h1") {
		t.Error("expected <h1> in rendered output")
	}
}

func TestAPI_Render_DefaultFile(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"alpha.md": "# Alpha",
		"bravo.md": "# Bravo",
	})

	// No ?file= param should use the first file (alpha.md).
	resp, err := http.Get(ts.URL + "/api/render")
	if err != nil {
		t.Fatalf("GET /api/render error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Alpha") {
		t.Error("expected default file (alpha.md) to be rendered")
	}
}

func TestAPI_Render_PathTraversal(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# ok",
	})

	resp, err := http.Get(ts.URL + "/api/render?file=../../etc/passwd")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", resp.StatusCode)
	}
}

func TestAPI_Render_NotFound(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# ok",
	})

	resp, err := http.Get(ts.URL + "/api/render?file=nonexistent.md")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing file, got %d", resp.StatusCode)
	}
}

func TestAPI_Info(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# ok",
	})

	resp, err := http.Get(ts.URL + "/api/info?file=test.md")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	var info map[string]string
	json.NewDecoder(resp.Body).Decode(&info)

	if info["fileName"] != "test.md" {
		t.Errorf("expected fileName=test.md, got %q", info["fileName"])
	}
	if info["filePath"] == "" {
		t.Error("expected filePath to be set")
	}
}

func TestAPI_FileServing(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md":   "# ok",
		"image.png": "fakepng",
	})

	resp, err := http.Get(ts.URL + "/file/image.png")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "fakepng" {
		t.Errorf("expected file content, got %q", string(body))
	}
}

func TestAPI_FileServing_PathTraversal(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# ok",
	})

	// Build the request manually to prevent the HTTP client from
	// normalizing "/../" out of the URL path before sending.
	req, _ := http.NewRequest("GET", ts.URL+"/file/ok", nil)
	req.URL.RawPath = "/file/..%2F..%2F..%2Fetc%2Fpasswd"
	req.URL.Path = "/file/../../../etc/passwd"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	// The server should reject this — either 400 (safePath catches it)
	// or 404 (Go's mux cleans the path). Either way, not 200.
	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 for path traversal on /file/, got 200")
	}
}

func TestListMdFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "charlie.md"), []byte("# C"), 0644)
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("# A"), 0644)
	os.WriteFile(filepath.Join(dir, "bravo.md"), []byte("# B"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	files, err := listMdFiles(dir)
	if err != nil {
		t.Fatalf("listMdFiles error: %v", err)
	}

	expected := []string{"alpha.md", "bravo.md", "charlie.md"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
	for i, name := range expected {
		if files[i] != name {
			t.Errorf("files[%d] = %q, want %q", i, files[i], name)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md": "# ok",
	})

	// The index.html should be served at /.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for /, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "mdview") {
		t.Error("expected index.html to contain 'mdview'")
	}
}

func TestStaticAssets_CSS(t *testing.T) {
	ts := newTestServer(t, map[string]string{"test.md": "# ok"})

	resp, err := http.Get(ts.URL + "/static/style.css")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for style.css, got %d", resp.StatusCode)
	}
}

func TestStaticAssets_JS(t *testing.T) {
	ts := newTestServer(t, map[string]string{"test.md": "# ok"})

	resp, err := http.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for app.js, got %d", resp.StatusCode)
	}
}

func TestStaticAssets_HighlightJS(t *testing.T) {
	ts := newTestServer(t, map[string]string{"test.md": "# ok"})

	resp, err := http.Get(ts.URL + "/static/highlight/highlight.min.js")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for highlight.min.js, got %d", resp.StatusCode)
	}
}

func TestRoot_NonRootPath_Returns404(t *testing.T) {
	ts := newTestServer(t, map[string]string{"test.md": "# ok"})

	resp, err := http.Get(ts.URL + "/nonexistent-page")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}

func TestAPI_Render_NonMdFile(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md":  "# ok",
		"data.txt": "hello",
	})

	resp, err := http.Get(ts.URL + "/api/render?file=data.txt")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-md file, got %d", resp.StatusCode)
	}
}

func TestAPI_Info_NonMdFile(t *testing.T) {
	ts := newTestServer(t, map[string]string{
		"test.md":  "# ok",
		"data.txt": "hello",
	})

	resp, err := http.Get(ts.URL + "/api/info?file=data.txt")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-md file on info, got %d", resp.StatusCode)
	}
}

func TestAPI_Info_NoFile(t *testing.T) {
	// Server with no initial file and no ?file= param.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "only.go"), []byte("package main"), 0644)

	hub := NewHub()
	server, listener, err := setupServer("127.0.0.1:0", "", dir, hub)
	if err != nil {
		t.Fatalf("setupServer error: %v", err)
	}
	ts := &httptest.Server{Listener: listener, Config: server}
	ts.Start()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/info")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when no file specified and no default, got %d", resp.StatusCode)
	}
}

func TestListMdFiles_NonexistentDir(t *testing.T) {
	_, err := listMdFiles("/nonexistent/dir/path")
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestListMdFiles_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.md"), []byte("# ok"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir.md"), 0755) // directory named .md

	files, err := listMdFiles(dir)
	if err != nil {
		t.Fatalf("listMdFiles error: %v", err)
	}

	if len(files) != 1 || files[0] != "file.md" {
		t.Errorf("expected [file.md], got %v", files)
	}
}
