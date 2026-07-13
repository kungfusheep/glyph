package main

import (
	"strings"
	"testing"
	"time"

	. "github.com/kungfusheep/glyph"
	termpkg "github.com/kungfusheep/glyph/term"
)

// testUI builds the app with deterministic shells and a compiled-once template,
// returning the ui and a render func. The template is built ONCE — every render
// re-executes it, which is the property the example is meant to demonstrate.
func testUI(t *testing.T, w, h int) (*ui, func() *Buffer) {
	t.Helper()
	// synchronous apply: these tests render on the test goroutine, so a mutation
	// can run in place. The concurrent case is covered in race_test.go.
	u := newUI(func(fn func()) { fn() }, func() {}, func() {}, func(tc *termpkg.TermC) {
		tc.Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	})
	u.resize(w, h)
	tmpl := Build(u.view())
	t.Cleanup(func() {
		for _, p := range u.tree.leaves() {
			u.slots[p.slot].Close()
		}
	})
	return u, func() *Buffer {
		buf := NewBuffer(w, h)
		tmpl.Execute(buf, int16(w), int16(h))
		return buf
	}
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

// TestViewRendersAndRoutesAcrossSplit drives the real usage path: compile the
// view once, render it, run a command in the focused shell, split, and confirm
// both shells render — the second in the right half, proving a real side-by-side
// layout rather than overlap. The split mutates state only; the template is
// never rebuilt.
func TestViewRendersAndRoutesAcrossSplit(t *testing.T) {
	const W, H = 60, 16

	u, render := testUI(t, W, H)
	render() // first render lazily starts pane 0's pty

	p0 := u.tree.leaves()[0]
	u.slots[p0.slot].Write([]byte("echo PANE_ZERO\n"))
	if !waitFor(t, render, H, "PANE_ZERO", 3*time.Second) {
		t.Fatal("pane 0 output never rendered")
	}

	// split side by side — focus moves to the new pane (tmux %)
	u.split(true)
	render() // lazily starts pane 1 at its new (narrower) geometry

	p1 := u.tree.focused
	u.slots[p1.slot].Write([]byte("echo PANE_ONE\n"))
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

// TestUnusedSlotsNeverStartShells pins the pool's central claim: a slot with no
// pane gets a zero rect, so it never opens a pty. If it were otherwise, merely
// launching the app would spawn maxPanes shells.
func TestUnusedSlotsNeverStartShells(t *testing.T) {
	const W, H = 60, 16
	u, render := testUI(t, W, H)
	render()

	live := u.tree.leaves()
	if len(live) != 1 {
		t.Fatalf("expected 1 pane at startup, got %d", len(live))
	}
	used := live[0].slot
	for slot, tc := range u.slots {
		if slot == used {
			continue
		}
		if _, err := tc.Write([]byte("x")); err == nil {
			t.Fatalf("slot %d has a live pty — unused slots must not start shells", slot)
		}
	}
}

// TestStatusLineTracksFocusAcrossSplit proves the status line is bound state,
// not a rebuilt tree: after a split the new pane's name appears in the SAME
// compiled template, and focus has moved to it.
func TestStatusLineTracksFocusAcrossSplit(t *testing.T) {
	const W, H = 60, 8
	u, render := testUI(t, W, H)

	last := render().GetLine(H - 1)
	if !strings.Contains(last, "glyph-term") {
		t.Fatalf("status line missing title: %q", last)
	}
	if !strings.Contains(last, "0") {
		t.Fatalf("status line missing pane name: %q", last)
	}

	u.split(true)
	last = render().GetLine(H - 1)
	if !strings.Contains(last, "1") {
		t.Fatalf("status line missing the new pane after split: %q", last)
	}
	if u.chips[1].Label != " 1 " || !u.chips[1].Focused {
		t.Fatalf("focus did not move to the new pane: chips = %+v", u.chips)
	}
	if u.chips[0].Focused {
		t.Fatalf("old pane still focused: chips = %+v", u.chips)
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

// TestTemplateSurvivesStructuralChange is the regression guard for the property
// this example exists to demonstrate: one compiled template, executed many times
// across splits, focus moves and pane deaths. If a change reintroduces a rebuild,
// the tree here would have to be recompiled to show the new panes — so a stale
// template that still renders every pane proves the bindings, not a rebuild, are
// carrying the state.
func TestTemplateSurvivesStructuralChange(t *testing.T) {
	const W, H = 60, 16
	u, render := testUI(t, W, H)
	render()

	u.split(true)  // 2 panes
	u.split(false) // 3 panes
	u.focusNext()

	if got := len(u.tree.leaves()); got != 3 {
		t.Fatalf("expected 3 panes, got %d", got)
	}

	// the SAME template, never rebuilt, must now place three distinct panes
	rects := u.layout(nil, W, 0)
	seen := map[Rect]bool{}
	for _, p := range u.tree.leaves() {
		r := rects[p.slot]
		if r.W == 0 || r.H == 0 {
			t.Fatalf("pane in slot %d has an empty rect %+v — not placed", p.slot, r)
		}
		if seen[r] {
			t.Fatalf("two panes share rect %+v — panes are overlapping, not tiled", r)
		}
		seen[r] = true
	}

	last := render().GetLine(H - 1)
	for _, name := range []string{"0", "1", "2"} {
		if !strings.Contains(last, name) {
			t.Fatalf("status line %q missing pane %s after splits", last, name)
		}
	}
}

// TestLayoutDoesNotAllocatePerFrame keeps the layout callback off the allocator.
// It runs on the render path every frame, so a fresh rect slice per call is
// garbage at frame rate — the same defect the terminal's blit had.
func TestLayoutDoesNotAllocatePerFrame(t *testing.T) {
	const W, H = 60, 16
	u, _ := testUI(t, W, H)
	u.split(true)
	u.layout(nil, W, 0) // warm

	if got := testing.AllocsPerRun(50, func() { u.layout(nil, W, 0) }); got != 0 {
		t.Fatalf("layout allocates %v times per frame, want 0", got)
	}
}
