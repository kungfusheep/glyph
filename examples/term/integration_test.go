package main

import (
	"strings"
	"testing"
	"time"

	. "github.com/kungfusheep/glyph"
	termpkg "github.com/kungfusheep/glyph/term"
)

func shellPane(name string) *pane {
	p := &pane{name: name}
	p.term = termpkg.New().
		Shell("/bin/sh").
		Env("PS1=", "TERM=dumb").
		Grow(1).
		OnUpdate(func() {})
	return p
}

// rowIndexOf returns the column where sub first appears in any row, or -1.
func rowIndexOf(buf *Buffer, h int, sub string) int {
	for y := 0; y < h; y++ {
		if i := strings.Index(buf.GetLine(y), sub); i >= 0 {
			return i
		}
	}
	return -1
}

// TestViewRendersAndRoutesAcrossSplit drives the real usage path: build the
// pane tree into a view, render it to a buffer, run a command in the focused
// shell, split, and confirm both shells render — the second one in the right
// half, proving a real side-by-side layout rather than overlap.
func TestViewRendersAndRoutesAcrossSplit(t *testing.T) {
	const W, H = 60, 16

	p0 := shellPane("0")
	tr := newTree(p0)
	defer func() {
		for _, p := range tr.leaves() {
			p.term.Close()
		}
	}()

	render := func() *Buffer {
		buf := NewBuffer(W, H)
		Build(buildView(tr)).Execute(buf, W, H)
		return buf
	}
	render() // first render lazily starts pane 0's pty

	p0.term.Write([]byte("echo PANE_ZERO\n"))
	if !waitFor(t, render, H, "PANE_ZERO", 3*time.Second) {
		t.Fatal("pane 0 output never rendered")
	}

	// split side by side — focus moves to the new pane (tmux %)
	p1 := shellPane("1")
	tr.splitFocused(true, p1)
	render() // lazily starts pane 1 at its new (narrower) geometry

	p1.term.Write([]byte("echo PANE_ONE\n"))
	if !waitFor(t, render, H, "PANE_ONE", 3*time.Second) {
		t.Fatal("pane 1 output never rendered after split")
	}

	buf := render()
	if rowIndexOf(buf, H, "PANE_ZERO") < 0 {
		t.Fatal("pane 0 vanished after split")
	}
	// pane 1 lives in the right half, so its marker starts at/after the divider
	if col := rowIndexOf(buf, H, "PANE_ONE"); col < W/2-2 {
		t.Fatalf("pane 1 marker at col %d, want right half (>= %d) — layout not side by side", col, W/2-2)
	}
}

// TestStatusLineNamesFocusedPane checks the status line lists panes and the
// focused pane's name is present (the naming requirement in the scope doc).
func TestStatusLineNamesFocusedPane(t *testing.T) {
	const W, H = 50, 8
	p0 := shellPane("0")
	tr := newTree(p0)
	defer p0.term.Close()

	buf := NewBuffer(W, H)
	Build(buildView(tr)).Execute(buf, W, H)

	// status line is the last row
	last := buf.GetLine(H - 1)
	if !strings.Contains(last, "glyph-term") {
		t.Fatalf("status line missing title: %q", last)
	}
	if !strings.Contains(last, "0") {
		t.Fatalf("status line missing pane name: %q", last)
	}
}

func waitFor(t *testing.T, render func() *Buffer, h int, sub string, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if rowIndexOf(render(), h, sub) >= 0 {
			return true
		}
		time.Sleep(30 * time.Millisecond)
	}
	return false
}
