package main

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Heading(t *testing.T) {
	html, err := renderMarkdown("testdata/simple.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, "<h1") {
		t.Error("expected <h1> tag in output")
	}
	if !strings.Contains(out, `id="hello-world"`) {
		t.Error("expected auto-generated heading ID")
	}
}

func TestRenderMarkdown_GFMTable(t *testing.T) {
	html, err := renderMarkdown("testdata/gfm.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, "<table>") {
		t.Error("expected <table> in output")
	}
	if !strings.Contains(out, "<td>Alice</td>") {
		t.Error("expected table cell with Alice")
	}
}

func TestRenderMarkdown_TaskList(t *testing.T) {
	html, err := renderMarkdown("testdata/gfm.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, `type="checkbox"`) {
		t.Error("expected checkbox input for task list")
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	html, err := renderMarkdown("testdata/code.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, "<code") {
		t.Error("expected <code> tag in output")
	}
	if !strings.Contains(out, "language-go") {
		t.Error("expected language-go class on code block")
	}
}

func TestRenderMarkdown_Strikethrough(t *testing.T) {
	html, err := renderMarkdown("testdata/gfm.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, "<del>") {
		t.Error("expected <del> tag for strikethrough")
	}
}

func TestRenderMarkdown_NonexistentFile(t *testing.T) {
	_, err := renderMarkdown("testdata/does_not_exist.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
