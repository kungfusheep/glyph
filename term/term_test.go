package term

import (
	"strings"
	"testing"
	"time"

	glyph "github.com/kungfusheep/glyph"
)

func bufContains(buf *glyph.Buffer, w, h int, sub string) bool {
	for y := 0; y < h; y++ {
		if strings.Contains(buf.GetLine(y), sub) {
			return true
		}
	}
	return false
}

// TestTermRendersShellOutput is the end-to-end proof: the component hosts a
// real shell, its output flows through the VT interpreter into the grid, and a
// Render blits that grid into a glyph.Buffer. If pty hosting, the reader loop,
// or the interpreter were broken, the marker never appears.
func TestTermRendersShellOutput(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	updates := make(chan struct{}, 256)
	tc.OnUpdate(func() {
		select {
		case updates <- struct{}{}:
		default:
		}
	})
	defer tc.Close()

	const w, h = 40, 10
	// first render lazily starts the pty at this geometry
	tc.Render(glyph.NewBuffer(w, h), 0, 0, w, h)

	if _, err := tc.Write([]byte("echo HELLO_TERM\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		buf := glyph.NewBuffer(w, h)
		tc.Render(buf, 0, 0, w, h)
		if bufContains(buf, w, h, "HELLO_TERM") {
			return // success
		}
		select {
		case <-updates:
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("HELLO_TERM never appeared in rendered output")
}

// TestTermResizeFromGeometry proves the component sizes the pty from its
// allocated cell box: rendering into a different geometry reshapes the grid.
func TestTermResizeFromGeometry(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	tc.Render(glyph.NewBuffer(20, 5), 0, 0, 20, 5)
	if tc.scr.rows != 5 || tc.scr.cols != 20 {
		t.Fatalf("initial grid %dx%d, want 20x5", tc.scr.cols, tc.scr.rows)
	}

	tc.Render(glyph.NewBuffer(60, 15), 0, 0, 60, 15)
	if tc.scr.rows != 15 || tc.scr.cols != 60 {
		t.Fatalf("resized grid %dx%d, want 60x15", tc.scr.cols, tc.scr.rows)
	}
}

// TestTermCursorInverse proves the focused terminal draws a cursor as an
// inverse cell, and an unfocused one does not.
func TestTermCursorInverse(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 20, 5
	tc.Render(glyph.NewBuffer(w, h), 0, 0, w, h)

	// place the cursor deterministically
	tc.scr.write([]byte("\x1b[2;3H"))

	tc.Focus(true)
	buf := glyph.NewBuffer(w, h)
	tc.Render(buf, 0, 0, w, h)
	if !buf.Get(2, 1).Style.Attr.Has(glyph.AttrInverse) {
		t.Fatal("focused cursor cell should be inverse")
	}

	tc.Focus(false)
	buf2 := glyph.NewBuffer(w, h)
	tc.Render(buf2, 0, 0, w, h)
	if buf2.Get(2, 1).Style.Attr.Has(glyph.AttrInverse) {
		t.Fatal("unfocused terminal should not draw a cursor")
	}
}
