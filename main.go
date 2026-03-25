package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 1414, "port to serve on")
	host := flag.String("host", "127.0.0.1", "host to bind to")
	noOpen := flag.Bool("no-open", false, "do not open browser automatically")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mdview [flags] [file.md | directory]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Default to current directory if no argument given.
	target := flag.Arg(0)
	if target == "" {
		target = "."
	}

	// Resolve to absolute path.
	targetPath, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mdview: error resolving path: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "mdview: not found: %s\n", targetPath)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mdview: error reading path: %v\n", err)
		os.Exit(1)
	}

	// Determine rootDir and initialFile.
	var rootDir, initialFile string
	if info.IsDir() {
		rootDir = targetPath
		files, err := listMdFiles(rootDir)
		if err != nil || len(files) == 0 {
			fmt.Fprintf(os.Stderr, "mdview: no markdown files found in %s\n", rootDir)
			os.Exit(1)
		}
		initialFile = files[0]
	} else {
		rootDir = filepath.Dir(targetPath)
		initialFile = filepath.Base(targetPath)
		ext := strings.ToLower(filepath.Ext(initialFile))
		if !isMdFile(initialFile) {
			fmt.Fprintf(os.Stderr, "mdview: warning: %q is not a recognized markdown extension, previewing anyway\n", ext)
		}
	}

	hub := NewHub()

	watcher, err := startWatcher(rootDir, func() {
		hub.Broadcast("reload")
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mdview: failed to start file watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	// Try the requested port. If taken, increment until one works.
	var server *http.Server
	var listener net.Listener
	for attempt := 0; attempt < 10; attempt++ {
		addr := fmt.Sprintf("%s:%d", *host, *port+attempt)
		server, listener, err = setupServer(addr, initialFile, rootDir, hub)
		if err == nil {
			break
		}
		if attempt == 9 {
			fmt.Fprintf(os.Stderr, "mdview: could not find an open port (%d–%d)\n", *port, *port+9)
			os.Exit(1)
		}
	}

	actualAddr := listener.Addr().String()
	url := fmt.Sprintf("http://%s", actualAddr)
	fmt.Fprintf(os.Stdout, "mdview: serving %s\n", rootDir)
	fmt.Fprintf(os.Stdout, "mdview: %s\n", url)
	fmt.Fprintf(os.Stdout, "mdview: watching for changes (press Ctrl+C to stop)\n")

	if !*noOpen {
		openBrowser(url)
	}

	// Start serving in a goroutine so we can block on shutdown signals.
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "mdview: server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Fprintf(os.Stdout, "\nmdview: shutting down...\n")
	hub.CloseAll()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mdview: warning: could not open browser: %v\n", err)
	}
}
