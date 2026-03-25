# mdview

A lightweight command-line markdown viewer with live reload. Run it from any directory to preview markdown files in your browser.

## Install

```
go install github.com/pablolopezsantori/mdview@latest
```

Or build from source:

```
git clone https://github.com/pablolopezsantori/mdview.git
cd mdview
go build -ldflags="-s -w" -o mdview .
```

## Usage

```
mdview                     # serve current directory
mdview README.md           # serve a single file
mdview docs/               # serve a directory
mdview --port 8080 .       # custom port
mdview --no-open .         # don't auto-open browser
```

The browser opens automatically. Edit any markdown file and the page reloads instantly.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `0` (auto) | Port to serve on. 0 picks a random available port. |
| `--host` | `127.0.0.1` | Host to bind to |
| `--no-open` | `false` | Don't open browser automatically |

## Features

- GitHub-Flavored Markdown (tables, task lists, strikethrough, autolinks)
- Syntax highlighting for code blocks
- Live reload on file changes
- File navigation when multiple markdown files exist
- Relative images and links resolved correctly
- Single binary, no runtime dependencies
