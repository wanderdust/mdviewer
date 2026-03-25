# mdview — Implementation Plan

## Lessons Learned from md-viewer

The original `md-viewer` repo works, but it grew into something much larger than needed for a markdown viewer. Here's what we're doing differently and why.

### What the original got right

1. **goldmark + GFM** — The right markdown library. Fast, extensible, good GFM support. Keep it.
2. **fsnotify + debounce** — File watching with a 200ms debounce works well. Catches atomic saves. Keep the same pattern.
3. **WebSocket live reload** — Simple hub/broadcast pattern. Clean, works. Keep it.
4. **`go:embed` for static files** — Single binary with embedded assets. Keep it.
5. **`safePath` validation** — Path traversal prevention with symlink resolution. Keep the logic.
6. **GitHub-style CSS** — Clean, familiar. Keep the approach.
7. **Scroll preservation on reload** — Small detail, big UX improvement. Keep it.

### What we're cutting

1. **Copilot integration** (copilot.go, prompts.go, prompts/) — Not needed. This was ~50% of the codebase.
2. **SQLite database** (state.go, state_queries.go) — A viewer doesn't need state. Removes the heaviest dependency (modernc.org/sqlite pulls in ~20 transitive deps).
3. **Backup system** (backup.go) — No editing = no backups needed.
4. **State machine / pipeline** — No workflow, no spec/plan/tasks lifecycle.
5. **`.mdview/` directory** — No hidden directories. No config. No state files.
6. **Feature selector UI** — No features, no selector.
7. **Copilot toolbar, edit bars, pending edits panel** — All Copilot UI elements.
8. **`copilot.js`** — Entire file is Copilot-specific.
9. **`detectFileType()`** — No need to classify files as "spec", "plan", or "tasks".
10. **`reorderArgs()` hack** — Use a proper CLI pattern instead.

### What we're improving

1. **Default to current directory** — The original requires a positional argument. Running `mdview` with no args should just work (serve `.`).
2. **Auto-pick port** — Default to port 0 (random available), not a hardcoded 5173. Avoids "port already in use" when running multiple instances.
3. **Cleaner HTML** — The original index.html has 15+ Copilot-specific elements with `display:none`. Start with a clean template.
4. **Cleaner CSS** — Remove all Copilot-specific styles (~40% of the CSS).
5. **Cleaner JS** — Remove `window.mdview` global state object that exists solely for copilot.js interop. The reload.js file has Copilot guards (`if copilotBusy return`). Start clean.
6. **File listing shows only current dir** — The original does `ReadDir` (non-recursive). This is correct for the viewer. Keep it simple.
7. **Better code organization** — The original server.go is 400+ lines mixing viewer endpoints with Copilot endpoints. Ours will be small.
8. **Syntax highlighting** — The original doesn't have code syntax highlighting. Add highlight.js (embedded, small).

### Dependency comparison

| Original | New |
|----------|-----|
| goldmark | goldmark |
| gorilla/websocket | gorilla/websocket |
| fsnotify | fsnotify |
| copilot-sdk | removed |
| modernc.org/sqlite (+ ~20 transitive) | removed |

**From ~25 total deps to 3.**

---

## File Structure

```
mdviewer/
├── main.go          # CLI entry point, flag parsing, startup/shutdown
├── server.go        # HTTP handlers and routing
├── render.go        # Markdown to HTML conversion
├── watcher.go       # File system watching with debounce
├── websocket.go     # WebSocket hub for live reload
├── security.go      # Path validation (safePath)
├── static/
│   ├── index.html   # Single-page app shell
│   ├── style.css    # GitHub-style theme
│   ├── app.js       # File nav, content loading, WebSocket, scroll
│   └── highlight/   # highlight.js (embedded, for code blocks)
│       ├── highlight.min.js
│       └── github.min.css
├── go.mod
├── go.sum
├── spec.md
└── plan.md
```

**7 Go files. 4 static files (+ highlight.js). Flat structure. No packages.**

---

## Implementation Steps

### Step 1: Project Setup

- `go mod init`
- Add goldmark, gorilla/websocket, fsnotify dependencies
- Create the directory structure

### Step 2: main.go — CLI Entry Point

What it does:
- Parse flags: `--port` (default 0), `--host` (default 127.0.0.1), `--no-open`
- Handle positional arg: file, directory, or none (defaults to ".")
- Resolve to absolute path
- Determine rootDir and initialFile
- Create WebSocket hub
- Start file watcher
- Set up HTTP server
- Bind listener (port 0 = auto-pick)
- Print startup URL
- Open browser (unless --no-open)
- Block on SIGINT/SIGTERM, then graceful shutdown

Key difference from original:
- No `reorderArgs()` hack — use `flag.Parse()` normally, positional arg is `flag.Arg(0)` defaulting to "."
- No Copilot initialization (saves ~40 lines)
- No database init
- No backup manager
- Simpler `setupServer` call (fewer params)

### Step 3: render.go — Markdown Rendering

What it does:
- Configure goldmark with GFM extension and auto heading IDs
- Single function: `renderMarkdown(filePath string) ([]byte, error)` — read file, convert to HTML

Straight from the original. ~35 lines. No changes needed except renaming.

### Step 4: security.go — Path Validation

What it does:
- `safePath(rootDir, requestedPath) (string, error)` — validate path stays within root
- Reject `..` patterns
- Resolve symlinks
- Check prefix

Straight from the original. ~60 lines. Works correctly.

### Step 5: watcher.go — File System Watching

What it does:
- `startWatcher(dir string, onChange func()) (*fsnotify.Watcher, error)`
- Watch root dir + subdirectories
- 200ms debounce
- Filter for markdown extensions only
- Dynamic: watch new directories, remove deleted ones

From the original, simplified:
- Remove `.specs` special-casing in the directory walk
- Skip all hidden directories (starting with `.`)
- Same debounce logic

### Step 6: websocket.go — Live Reload Hub

What it does:
- `Hub` struct with Register/Unregister/Broadcast/CloseAll
- `HandleWebSocket` handler for upgrade
- Thread-safe via mutex

Straight from the original. ~95 lines. No changes needed.

### Step 7: server.go — HTTP Handlers

What it does:
- `setupServer(addr, initialFile, rootDir string, hub *Hub) (*http.Server, net.Listener, error)`
- Route: `GET /` → serve embedded index.html
- Route: `GET /static/*` → serve embedded CSS/JS
- Route: `GET /api/files` → JSON list of markdown files
- Route: `GET /api/render?file=X` → rendered HTML
- Route: `GET /api/info?file=X` → file metadata JSON
- Route: `GET /file/*` → serve raw files (images, assets)
- Route: `GET /ws` → WebSocket upgrade

Key difference from original:
- No Copilot endpoints (removes 8 routes and ~200 lines of handler code)
- No `copilotOpBusy` mutex
- No `BackupManager` or `StateDB` parameters
- Helper: `listMdFiles(dir)` — read dir, filter markdown, sort
- Helper: `resolveRequestedFile(r, rootDir, defaultFile)` — extract ?file= param, validate

### Step 8: static/index.html — Clean HTML

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>mdview</title>
    <link rel="stylesheet" href="/static/style.css">
    <link rel="stylesheet" href="/static/highlight/github.min.css">
</head>
<body>
    <header id="topbar">
        <span class="dirpath" id="dirpath"></span>
        <span class="filename" id="filename"></span>
    </header>
    <nav id="filenav"></nav>
    <main id="content">
        <p>Loading...</p>
    </main>
    <script src="/static/highlight/highlight.min.js"></script>
    <script src="/static/app.js"></script>
</body>
</html>
```

**13 lines** vs original's 49 lines (15+ Copilot elements removed).

### Step 9: static/app.js — Client-Side Logic

What it does:
- On load: fetch file list, load initial content, connect WebSocket
- File navigation: render tabs, handle clicks, update URL (?file=)
- Content loading: fetch /api/render, inject HTML, post-process
- Post-processing: rewrite external links (target=_blank), resolve relative images, make .md links navigate in-app, wrap tables
- WebSocket: connect, reconnect with backoff, reload on "reload" message, preserve scroll position
- Connection banners: warn on disconnect, confirm on reconnect
- File not found overlay: show when 404, hide when file reappears
- Syntax highlighting: call `hljs.highlightAll()` after content load

Key difference from original:
- No `window.mdview` global interop object
- No `copilotBusy` guards
- No Copilot state-driven nav
- Single clean file instead of reload.js + copilot.js
- Add highlight.js integration (one line: `hljs.highlightAll()` after render)

### Step 10: static/style.css — Clean Theme

GitHub-style theme covering:
- Body: system fonts, clean spacing
- Top bar: sticky, light background, path + filename
- File nav: horizontal tabs with active state
- Content: max-width 800px, centered, markdown typography
- Tables: bordered, striped, scrollable wrapper
- Code blocks: background, padding, highlight.js colors
- Task lists: checkbox styling
- Connection banners: warn (yellow), ok (green)
- Not-found overlay

Remove all Copilot styles: feature selector, toolbar, floating input bars, pending edits panel, locked banner, progress area, copilot buttons, copilot spinner.

### Step 11: Install & Test

- `go build -o mdview .`
- Test: `mdview` (current dir)
- Test: `mdview README.md` (single file)
- Test: `mdview docs/` (directory)
- Test: `mdview --port 3000 .` (custom port)
- Test: `mdview --no-open .` (no browser)
- Test: edit a file → verify live reload
- Test: add/remove a file → verify nav updates
- Test: verify images render
- Test: verify code syntax highlighting
- Test: verify table scrolling
- Test: run two instances simultaneously (auto port should prevent conflicts)

---

## Code Style Guidelines

Since you're learning Go through this project, the code follows these principles:

1. **No clever abstractions** — Every function does one obvious thing. If you can't tell what a function does from its name and first 3 lines, it's too clever.

2. **Comments explain why, not what** — The code should be readable enough to see *what* it does. Comments explain *why* a decision was made.

3. **Flat structure** — Everything in `package main`. No internal packages, no interfaces (unless Go requires them). When you have 7 files, a flat structure is the right choice.

4. **Named return values only when they help** — Don't use them just because Go allows it. Use them when the meaning isn't obvious from the type.

5. **Error handling is explicit** — Check every error. No `_` for errors. Log and exit in main, return errors from everything else.

6. **Standard library first** — Use `net/http`, `flag`, `log`, `os`, `path/filepath` from the standard library. Only add a dependency when the standard library genuinely can't do the job.

7. **Small files** — Each file should be under 150 lines. If it's getting longer, it's probably doing too much.
