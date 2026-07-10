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
