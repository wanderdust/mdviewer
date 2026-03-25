# mdview — Tasks

## Phase 1: Project Scaffold

- [ ] Run `go mod init github.com/pablolopezsantori/mdview`
- [ ] `go get github.com/yuin/goldmark github.com/gorilla/websocket github.com/fsnotify/fsnotify`
- [ ] Create `static/` directory
- [ ] Create empty placeholder files: `main.go`, `server.go`, `render.go`, `watcher.go`, `websocket.go`, `security.go`
- [ ] Verify `go build .` compiles (with empty `func main()`)

## Phase 2: security.go — Path Validation

- [ ] Implement `safePath(rootDir, requestedPath string) (string, error)`
  - Reject empty paths
  - Reject paths containing `..`
  - Join rootDir + requestedPath, resolve to absolute
  - Resolve symlinks (fall back to parent check if file doesn't exist yet)
  - Verify resolved path is within rootDir
- [ ] Implement `isMdFile(name string) bool` — check extension against `.md`, `.markdown`, `.mdown`, `.mkd`
- [ ] Write `security_test.go`:
  - Test: valid relative path returns correct absolute path
  - Test: `..` in path returns error
  - Test: empty path returns error
  - Test: path outside rootDir returns error
  - Test: nested valid path works (`subdir/file.md`)
  - Test: `isMdFile` returns true for `.md`, `.markdown`, `.mdown`, `.mkd`
  - Test: `isMdFile` returns false for `.txt`, `.go`, `.html`, no extension

## Phase 3: render.go — Markdown Rendering

- [ ] Configure goldmark with GFM extension, auto heading IDs, XHTML output
- [ ] Implement `renderMarkdown(filePath string) ([]byte, error)` — read file, convert to HTML bytes
- [ ] Write `render_test.go`:
  - Test: renders `# Hello` to `<h1>` with ID
  - Test: renders GFM table to `<table>`
  - Test: renders task list `- [ ] item` to checkbox HTML
  - Test: renders fenced code block with language class
  - Test: renders strikethrough `~~text~~`
  - Test: returns error for non-existent file
  - Use `testdata/` directory with small `.md` fixture files

## Phase 4: websocket.go — Live Reload Hub

- [ ] Implement `Hub` struct with `clients map[*websocket.Conn]bool` and `sync.Mutex`
- [ ] Implement `NewHub() *Hub`
- [ ] Implement `Register(conn)`, `Unregister(conn)`, `Broadcast(message)`, `CloseAll()`
- [ ] Implement `HandleWebSocket(hub *Hub) http.HandlerFunc` — upgrade connection, register, read loop for keepalive
- [ ] Configure `websocket.Upgrader` with `CheckOrigin: func(r) bool { return true }`
- [ ] Write `websocket_test.go`:
  - Test: Register adds a client, Unregister removes it
  - Test: Broadcast sends message to all registered clients
  - Test: Broadcast removes clients that fail to receive
  - Test: CloseAll closes all connections and empties the map
  - Test: HandleWebSocket upgrades HTTP connection (use `httptest.NewServer` + `websocket.Dial`)

## Phase 5: watcher.go — File System Watching

- [ ] Implement `startWatcher(dir string, onChange func()) (*fsnotify.Watcher, error)`
  - Create fsnotify watcher
  - Add root dir
  - Walk subdirectories, add each (skip hidden dirs starting with `.`)
  - Goroutine: listen for events, filter for markdown files only, debounce 200ms, call `onChange`
  - Dynamically watch new directories (Create events), remove deleted ones (Remove/Rename events)
- [ ] Write `watcher_test.go`:
  - Test: onChange fires when a `.md` file is written (use temp dir, write file, assert callback within 500ms)
  - Test: onChange does NOT fire for `.go` or `.txt` file changes
  - Test: debounce — rapid writes trigger only one onChange call
  - Test: watcher.Close() stops watching cleanly

## Phase 6: server.go — HTTP Handlers & Routing

- [ ] Implement `listMdFiles(dir string) ([]string, error)` — ReadDir, filter markdown, sort alphabetically
- [ ] Implement `resolveRequestedFile(r, rootDir, defaultFile) (absPath, baseName, error)` — extract `?file=` param, validate with safePath
- [ ] Implement `setupServer(addr, initialFile, rootDir string, hub *Hub) (*http.Server, net.Listener, error)`
- [ ] Route `GET /` — serve embedded `static/index.html`
- [ ] Route `GET /static/*` — serve embedded static files (CSS, JS, highlight.js)
- [ ] Route `GET /api/files` — return JSON array of markdown filenames
- [ ] Route `GET /api/render?file=X` — render markdown file to HTML, return as text/html
- [ ] Route `GET /api/info?file=X` — return JSON `{ "fileName": "X.md", "filePath": "/abs/path/X.md" }`
- [ ] Route `GET /file/*` — serve raw files from rootDir (for images, linked assets)
- [ ] Route `GET /ws` — delegate to `HandleWebSocket(hub)`
- [ ] Embed static files with `//go:embed static/*`
- [ ] Write `server_test.go`:
  - Test: `GET /api/files` returns JSON array of `.md` files in test dir
  - Test: `GET /api/files` returns empty array when no markdown files
  - Test: `GET /api/render?file=test.md` returns rendered HTML
  - Test: `GET /api/render` with no file param uses default file
  - Test: `GET /api/render?file=../../etc/passwd` returns 400 (path traversal)
  - Test: `GET /api/render?file=nonexistent.md` returns 404
  - Test: `GET /api/info?file=test.md` returns correct JSON
  - Test: `GET /file/image.png` serves the raw file
  - Test: `GET /file/../secret` returns 400
  - Test: `listMdFiles` returns sorted filenames, excludes non-md
  - Use `httptest.NewServer` with a temp dir containing test fixtures

## Phase 7: main.go — CLI Entry Point

- [ ] Parse flags: `--port` (default 0), `--host` (default 127.0.0.1), `--no-open` (default false)
- [ ] Positional arg: `flag.Arg(0)`, default to `"."` if empty
- [ ] Resolve target path to absolute, check existence with `os.Stat`
- [ ] If directory: set rootDir, find first markdown file as initialFile (exit with error if none found)
- [ ] If file: set rootDir to parent dir, initialFile to basename. Warn if extension isn't recognized markdown.
- [ ] Create WebSocket hub with `NewHub()`
- [ ] Start file watcher with `startWatcher(rootDir, func() { hub.Broadcast("reload") })`
- [ ] Call `setupServer` to create server and listener
- [ ] Print startup info: root dir, initial file, URL (derived from listener address)
- [ ] Open browser with platform-specific command (darwin: `open`, linux: `xdg-open`, windows: `cmd /c start`) unless `--no-open`
- [ ] Start `server.Serve(listener)` in a goroutine
- [ ] Block on `SIGINT`/`SIGTERM`, then: close watcher, close hub, shutdown server with timeout

## Phase 8: static/index.html — Page Shell

- [ ] Minimal HTML: charset, viewport, title "mdview"
- [ ] Link `style.css` and `highlight/github.min.css`
- [ ] `<header id="topbar">` with `<span id="dirpath">` and `<span id="filename">`
- [ ] `<nav id="filenav">` — empty, populated by JS
- [ ] `<main id="content">Loading...</main>`
- [ ] Script tags: `highlight/highlight.min.js`, then `app.js`

## Phase 9: static/style.css — GitHub Theme

- [ ] Base reset: box-sizing, margin, padding
- [ ] Body: system font stack, 16px, line-height 1.6, light background
- [ ] Top bar: sticky, light gray background, border-bottom, flex layout, path + filename
- [ ] File nav: horizontal tab bar, items with padding, active state with border-bottom accent, hover state, hidden when empty
- [ ] Content area: max-width 800px, centered with auto margins, padding
- [ ] Markdown typography: headings (h1–h6) with sizes and spacing, paragraphs, lists
- [ ] Links: blue, underline on hover
- [ ] Code: inline code with background and padding; fenced blocks with background, padding, border-radius, overflow-x scroll
- [ ] Tables: full-width, bordered, header background, striped rows, scrollable wrapper div
- [ ] Task lists: checkbox alignment, no bullet
- [ ] Blockquotes: left border, muted color, padding
- [ ] Images: max-width 100%, display block
- [ ] Horizontal rules: subtle border
- [ ] Connection banners: warn (yellow background), ok (green background), fixed position top
- [ ] Not-found overlay: centered message, semi-transparent background

## Phase 10: static/app.js — Client Logic

- [ ] On `DOMContentLoaded`: read `?file=` from URL, fetch file list, load info, load content, connect WebSocket
- [ ] `loadFiles()` — fetch `/api/files`, set `currentFile` to first if not set, call `renderFileNav(files)`
- [ ] `renderFileNav(files)` — create `<a>` tabs in `#filenav`, mark active, hide nav if only 1 file
- [ ] `handleFileNavClick(e)` — update `currentFile`, push URL state, toggle active class, reload info + content
- [ ] `popstate` listener — handle browser back/forward by re-loading the `?file=` param
- [ ] `loadInfo()` — fetch `/api/info?file=X`, update document title and topbar spans
- [ ] `shortenPath(fullPath, fileName)` — show last 3 directory segments before filename
- [ ] `loadContent()` — fetch `/api/render?file=X`, set `#content` innerHTML, call `postProcess(container)`, fade effect
- [ ] `postProcess(container)`:
  - External links: add `target="_blank"` and `rel="noopener noreferrer"`
  - Relative images: prefix `src` with `/file/`
  - Relative non-md links: prefix `href` with `/file/`
  - Markdown links: rewrite `.md` hrefs to `?file=` and add click handler for in-app navigation
  - Tables: wrap each `<table>` in a `.table-wrapper` div
  - Syntax highlighting: call `hljs.highlightAll()`
- [ ] `connectWebSocket()` — connect to `ws://host/ws`, on message "reload" call `reloadContent()`
  - On close: show warning banner, reconnect with backoff (1s initial, 1.5x, max 5s)
  - On open: hide banner (or show "Reconnected" briefly if was previously connected)
  - 5s connect timeout warning if never connects
- [ ] `reloadContent()` — save scrollY, fetch + re-render, restore scrollY. Also re-fetch file list (files may have been added/removed). Handle 404 with not-found overlay.
- [ ] `showNotFoundOverlay()` / `removeNotFoundOverlay()` — overlay when file deleted
- [ ] `showConnectionBanner(type, text)` / `removeConnectionBanner()` — connection status UI

## Phase 11: highlight.js — Syntax Highlighting

- [ ] Download highlight.js minified bundle (common languages: js, ts, python, go, bash, json, yaml, html, css, sql, rust, java, c, cpp, ruby, diff, markdown)
- [ ] Download github.min.css theme
- [ ] Place in `static/highlight/highlight.min.js` and `static/highlight/github.min.css`
- [ ] Verify `go:embed static/*` picks them up (build, check binary size is reasonable)

## Phase 12: Unit Tests — Full Coverage

- [ ] `security_test.go` — covered in Phase 2
- [ ] `render_test.go` — covered in Phase 3
- [ ] `websocket_test.go` — covered in Phase 4
- [ ] `watcher_test.go` — covered in Phase 5
- [ ] `server_test.go` — covered in Phase 6
- [ ] Create `testdata/` directory with fixture files:
  - `testdata/simple.md` — basic headings, paragraphs
  - `testdata/gfm.md` — table, task list, strikethrough
  - `testdata/code.md` — fenced code blocks with language tags
  - `testdata/links.md` — relative links, external links, md links, image refs
- [ ] Run `go test ./... -v` — all tests pass
- [ ] Run `go test ./... -race` — no race conditions
- [ ] Run `go vet ./...` — no issues

## Phase 13: End-to-End Tests

These tests start the full server and verify behavior from a browser/client perspective. Written in Go using `httptest` and a real WebSocket client.

- [ ] `e2e_test.go` — test the full application flow:
  - **Startup with directory**: create temp dir with 3 `.md` files, start server, verify `/api/files` returns all 3 sorted
  - **Startup with single file**: create temp dir, start server pointing at one file, verify that file is the default, verify sibling files appear in `/api/files`
  - **Render cycle**: write a `.md` file, `GET /api/render?file=X`, verify HTML output contains expected elements
  - **File nav**: verify `/api/files` returns correct list, verify switching files via `?file=` works
  - **Live reload via WebSocket**: connect WebSocket, modify a `.md` file on disk, assert "reload" message received within 1 second
  - **Live reload ignores non-md**: connect WebSocket, modify a `.txt` file, assert NO message received within 500ms
  - **New file discovery**: connect WebSocket, create a new `.md` file, assert "reload" fires, then `GET /api/files` includes the new file
  - **File deletion handling**: `GET /api/render?file=deleted.md` after removing the file returns 404
  - **Image serving**: place a `.png` in temp dir, verify `GET /file/image.png` returns it with correct content-type
  - **Path traversal blocked**: `GET /api/render?file=../../etc/passwd` returns 400
  - **Path traversal on raw files**: `GET /file/../../etc/passwd` returns 400
  - **Info endpoint**: `GET /api/info?file=test.md` returns JSON with correct fileName and filePath
  - **Static assets**: `GET /static/style.css` returns CSS, `GET /static/app.js` returns JS
  - **Concurrent clients**: connect 2 WebSocket clients, modify file, both receive "reload"

## Phase 14: Build & Install

- [ ] `go build -o mdview .` — produces single binary
- [ ] Verify binary size is reasonable (should be < 5MB)
- [ ] Test: `./mdview` from a directory with markdown files — browser opens, files render
- [ ] Test: `./mdview spec.md` — single file mode works
- [ ] Test: `./mdview --no-open .` — no browser opened, server running
- [ ] Test: run two instances — both get different ports, no conflict
- [ ] Test: `Ctrl+C` — clean shutdown, no errors
- [ ] Add install instructions to README: `go install github.com/pablolopezsantori/mdview@latest`
