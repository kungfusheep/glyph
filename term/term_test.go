package term

import (
	"strings"
	"testing"
	"time"

	glyph "github.com/kungfusheep/glyph"
)

func bufContains(buf *glyph.Buffer, h int, sub string) bool {
	for y := 0; y < h; y++ {
		if strings.Contains(buf.GetLine(y), sub) {
			return true
		}
	}
	return false
}

func renderTerm(tc *TermC, w, h int) *glyph.Buffer {
	buf := glyph.NewBuffer(w, h)
	// a terminal fills its parent box; give it one (VBox) the way a real layout
	// does, so grow has a container to distribute
	glyph.Build(glyph.VBox(tc)).Execute(buf, int16(w), int16(h))
	return buf
}

// TestTermRendersShellOutput is the end-to-end proof: the component hosts a
// real shell, its output flows through the VT interpreter into the grid, and a
// render blits that grid into the buffer via the Layer path.
func TestTermRendersShellOutput(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 40, 10
	renderTerm(tc, w, h) // first render starts the pty at this geometry

	if _, err := tc.Write([]byte("echo HELLO_TERM\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bufContains(renderTerm(tc, w, h), h, "HELLO_TERM") {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("HELLO_TERM never appeared in rendered output")
}

// TestTermResizeFromGeometry proves the component sizes the pty from its
// allocated cell box: rendering into a different geometry reshapes the grid.
func TestTermResizeFromGeometry(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	renderTerm(tc, 20, 5)
	if tc.scr.rows != 5 || tc.scr.cols != 20 {
		t.Fatalf("initial grid %dx%d, want 20x5", tc.scr.cols, tc.scr.rows)
	}

	renderTerm(tc, 60, 15)
	if tc.scr.rows != 15 || tc.scr.cols != 60 {
		t.Fatalf("resized grid %dx%d, want 60x15", tc.scr.cols, tc.scr.rows)
	}
}

// TestTermCursorFollowsFocus proves the focused terminal shows the pty cursor
// on its layer and an unfocused one hides it.
func TestTermCursorFollowsFocus(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 20, 5
	renderTerm(tc, w, h)
	tc.scr.write([]byte("\x1b[2;3H")) // place cursor deterministically

	tc.Focus(true)
	renderTerm(tc, w, h)
	if cur := tc.layer.Cursor(); !cur.Visible {
		t.Fatal("focused terminal should show its cursor")
	}

	tc.Focus(false)
	renderTerm(tc, w, h)
	if cur := tc.layer.Cursor(); cur.Visible {
		t.Fatal("unfocused terminal should hide its cursor")
	}
}

// TestBlitDoesNotAllocatePerFrame pins the steady-state render path to zero
// allocations. A fresh Buffer per painted frame is garbage at pty output rate,
// which breaks glyph's zero-alloc-per-render contract.
func TestBlitDoesNotAllocatePerFrame(t *testing.T) {
	const w, h = 80, 24
	tc := blitFixture(w, h)
	tc.blitToLayer(w, h) // first frame legitimately allocates the buffer

	if got := testing.AllocsPerRun(50, func() { tc.blitToLayer(w, h) }); got != 0 {
		t.Fatalf("blitToLayer allocates %v times per frame, want 0", got)
	}
}

// TestBlitReallocatesOnResize proves the reuse is size-aware: a resized viewport
// must get a correctly sized buffer, not a stale one.
func TestBlitReallocatesOnResize(t *testing.T) {
	tc := blitFixture(80, 24)
	tc.blitToLayer(80, 24)
	tc.scr.resize(10, 40)
	tc.blitToLayer(40, 10)

	if tc.buf.Width() != 40 || tc.buf.Height() != 10 {
		t.Fatalf("buffer is %dx%d after resize, want 40x10", tc.buf.Width(), tc.buf.Height())
	}
}

// TestOnTitleNotCalledUnderScreenLock guards the lock order. The host's title
// callback runs on the pty goroutine; if it fires while write() holds the screen
// lock, a host that takes its own mutex there deadlocks against the render
// goroutine, which takes the locks the other way round in blitToLayer.
func TestOnTitleNotCalledUnderScreenLock(t *testing.T) {
	s := newScreen(24, 80)
	var held bool
	var got string
	s.onTitle = func(title string) {
		got = title
		if s.mu.TryLock() {
			s.mu.Unlock() // free to take it: the callback is outside the lock
		} else {
			held = true
		}
	}

	s.write([]byte("\x1b]0;my-title\x07"))

	if held {
		t.Fatal("onTitle fired while write() held the screen lock — a host mutex here deadlocks the renderer")
	}
	if got != "my-title" {
		t.Fatalf("onTitle got %q, want my-title", got)
	}
}
