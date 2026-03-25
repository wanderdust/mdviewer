# mdview — Specification

## What It Is

A lightweight command-line tool that renders markdown files in the browser with live reload. You run `mdview` from any directory and it serves a clean, readable view of your markdown files.

This is a **viewer only** — no editing, no AI, no workflow. Just fast, beautiful markdown rendering you can call from anywhere.

## Use Cases

- You're in a repo and want to preview README.md or docs/
- You're writing a spec and want to see it rendered as you type
- You want to quickly browse all markdown files in a project

## CLI Interface

```
mdview [file.md | directory]
mdview                        # current directory
mdview README.md              # single file
mdview docs/                  # directory of markdown files
mdview --port 8080 .          # custom port
mdview --no-open .            # don't auto-open browser
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `0` (auto) | Port to serve on. `0` picks a random available port. |
| `--host` | `127.0.0.1` | Host to bind to |
| `--no-open` | `false` | Don't open browser automatically |

### Positional Argument

- **File**: Serve that file. Root directory is the file's parent directory. Navigation shows sibling markdown files.
- **Directory**: Serve all markdown files in that directory. Opens the first file alphabetically.
- **None**: Same as passing `.` (current directory).

### Exit

`Ctrl+C` for graceful shutdown.

## Markdown Discovery

Recognized extensions: `.md`, `.markdown`, `.mdown`, `.mkd` (case-insensitive).

Files are listed from the root directory only (not recursive). Sorted alphabetically. Hidden files/directories (starting with `.`) are excluded.

## Rendering

- GitHub-Flavored Markdown (tables, task lists, strikethrough, autolinks)
- Auto-generated heading IDs for anchor links
- GitHub-style CSS theme (clean, readable, familiar)
- Relative image paths resolved correctly (images display inline)
- Relative links to other `.md` files navigate within the viewer
- External links open in a new tab
- Tables wrapped in scrollable containers for wide tables
- Code blocks with syntax highlighting via highlight.js

## Live Reload

When any markdown file in the root directory changes on disk:

1. File system watcher detects the change
2. Server notifies browser via WebSocket
3. Browser re-fetches and re-renders the current file
4. Scroll position is preserved

Debounce: 200ms after last change event (handles atomic save patterns from editors).

The watcher monitors the root directory and its immediate subdirectories. New subdirectories are picked up automatically. Deleted directories are cleaned up.

## Web UI

### Layout

```
┌─────────────────────────────────────┐
│ topbar: dirpath / filename          │
├─────────────────────────────────────┤
│ filenav: file1.md | file2.md | ...  │
├─────────────────────────────────────┤
│                                     │
│           rendered markdown          │
│                                     │
└─────────────────────────────────────┘
```

- **Top bar**: Sticky. Shows abbreviated directory path and current filename.
- **File nav**: Horizontal tab bar showing all markdown files. Click to switch. Active file highlighted. Hidden when only one file exists.
- **Content area**: Rendered HTML. Max-width for readability. Centered.

### URL State

The current file is stored in `?file=filename.md`. Supports browser back/forward navigation. Shareable URLs.

### Connection Status

- If WebSocket disconnects: shows a warning banner "Connection lost — reconnecting..."
- On reconnect: shows "Reconnected" for 2 seconds, then hides
- If file is deleted while viewing: shows "File not found — waiting for file to reappear..."

## HTTP API

All endpoints are internal (localhost only). No authentication needed.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Serve the single-page app (index.html) |
| `/static/*` | GET | Embedded CSS, JS assets |
| `/api/files` | GET | JSON array of markdown filenames in root |
| `/api/render?file=X` | GET | Rendered HTML for the given file |
| `/api/info?file=X` | GET | JSON with fileName, filePath metadata |
| `/file/*` | GET | Serve raw files (for images, assets) |
| `/ws` | GET | WebSocket upgrade for live reload |

## Security

- Path traversal prevention: all file paths validated to stay within root directory
- `..` in paths rejected
- Symlinks resolved and checked against root boundary
- Only binds to localhost by default
- WebSocket allows all origins (localhost only, so acceptable)

## Architecture Constraints

- **Single binary**: `go build` produces one executable, no runtime dependencies
- **Embedded assets**: HTML, CSS, JS are embedded in the binary via `go:embed`
- **Minimal dependencies**: Only what's truly needed (markdown parser, websocket, file watcher)
- **No database**: Pure file-system based. No state files, no config directories
- **No hidden directories**: Doesn't create `.mdview/` or any dot-directories
- **Simple code**: Flat package structure (all in `main`). Short files. Clear names. No abstractions beyond what's needed.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/yuin/goldmark` | Markdown to HTML (with GFM extension) |
| `github.com/gorilla/websocket` | WebSocket for live reload |
| `github.com/fsnotify/fsnotify` | Cross-platform file system watching |

Three dependencies. That's it.

## What This Is NOT

- Not an editor
- Not an AI tool
- Not a workflow system
- Not a note-taking app
- No database, no state, no config files
- No hidden directories created in your project
