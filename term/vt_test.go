package term

import (
	"strings"
	"testing"

	glyph "github.com/kungfusheep/glyph"
)

// rowText returns the runes of row y as a string, trailing blanks trimmed.
func rowText(s *screen, y int) string {
	var b strings.Builder
	for x := 0; x < s.cols; x++ {
		b.WriteRune(s.cellAt(x, y).Rune)
	}
	return strings.TrimRight(b.String(), " ")
}

func feed(s *screen, in string) { s.write([]byte(in)) }

func TestPrintAndNewline(t *testing.T) {
	s := newScreen(4, 20)
	feed(s, "hello\r\nworld")
	if got := rowText(s, 0); got != "hello" {
		t.Fatalf("row0 = %q, want hello", got)
	}
	if got := rowText(s, 1); got != "world" {
		t.Fatalf("row1 = %q, want world", got)
	}
	if s.cx != 5 || s.cy != 1 {
		t.Fatalf("cursor = (%d,%d), want (5,1)", s.cx, s.cy)
	}
}

func TestCarriageReturnOverwrite(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "hello\rHI")
	if got := rowText(s, 0); got != "HIllo" {
		t.Fatalf("row0 = %q, want HIllo", got)
	}
}

func TestCursorPositionAndOverwrite(t *testing.T) {
	s := newScreen(5, 20)
	feed(s, "\x1b[3;5Hmark") // row 3, col 5 (1-based)
	if got := rowText(s, 2); got != "    mark" {
		t.Fatalf("row2 = %q, want '    mark'", got)
	}
	if s.cy != 2 || s.cx != 8 {
		t.Fatalf("cursor = (%d,%d), want (8,2)", s.cx, s.cy)
	}
}

func TestSGRSetsPen(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "\x1b[1;31mX\x1b[0mY")
	cx := s.cellAt(0, 0)
	if cx.Rune != 'X' || !cx.Style.Attr.Has(glyph.AttrBold) {
		t.Fatalf("cell X missing bold: %+v", cx.Style)
	}
	if cx.Style.FG != glyph.Ansi16(1) {
		t.Fatalf("cell X fg = %+v, want ansi red", cx.Style.FG)
	}
	cy := s.cellAt(1, 0)
	if cy.Rune != 'Y' || cy.Style.Attr.Has(glyph.AttrBold) {
		t.Fatalf("cell Y should be reset, got %+v", cy.Style)
	}
}

func TestSGR256AndTruecolor(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "\x1b[38;5;200mA\x1b[38;2;10;20;30mB")
	if fg := s.cellAt(0, 0).Style.FG; fg != glyph.Ansi256(200) {
		t.Fatalf("A fg = %+v, want ansi256 200", fg)
	}
	if fg := s.cellAt(1, 0).Style.FG; fg != glyph.RGB(10, 20, 30) {
		t.Fatalf("B fg = %+v, want rgb(10,20,30)", fg)
	}
}

func TestEraseLine(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "abcdef")
	feed(s, "\x1b[1;4H") // cursor to col 4
	feed(s, "\x1b[0K")   // erase cursor to end
	if got := rowText(s, 0); got != "abc" {
		t.Fatalf("row0 = %q, want abc", got)
	}
}

func TestEraseDisplay(t *testing.T) {
	s := newScreen(3, 10)
	feed(s, "aaa\r\nbbb\r\nccc")
	feed(s, "\x1b[2J") // clear whole display
	for y := 0; y < 3; y++ {
		if got := rowText(s, y); got != "" {
			t.Fatalf("row%d = %q, want empty after 2J", y, got)
		}
	}
}

func TestDeferredAutowrap(t *testing.T) {
	s := newScreen(3, 5) // 5 columns
	feed(s, "abcde")     // exactly fills row 0; cursor should defer-wrap, not advance
	if s.cy != 0 {
		t.Fatalf("after filling row, cy = %d, want 0 (deferred)", s.cy)
	}
	feed(s, "f") // now wraps to row 1
	if got := rowText(s, 0); got != "abcde" {
		t.Fatalf("row0 = %q, want abcde", got)
	}
	if got := rowText(s, 1); got != "f" {
		t.Fatalf("row1 = %q, want f", got)
	}
}

func TestScrollOnLineFeed(t *testing.T) {
	s := newScreen(2, 10) // 2 rows
	feed(s, "one\r\ntwo\r\nthree")
	// row0 "one" scrolled off; now "two","three"
	if got := rowText(s, 0); got != "two" {
		t.Fatalf("row0 = %q, want two", got)
	}
	if got := rowText(s, 1); got != "three" {
		t.Fatalf("row1 = %q, want three", got)
	}
}

func TestScrollRegion(t *testing.T) {
	s := newScreen(4, 10)
	feed(s, "\x1b[2;3r") // scroll region rows 2..3 (1-based)
	feed(s, "\x1b[2;1HA\r\nB\r\nC")
	// region is rows idx1..idx2; feeding A\nB\nC scrolls within it, leaving row0 and row3 untouched
	if got := rowText(s, 0); got != "" {
		t.Fatalf("row0 outside region should stay empty, got %q", got)
	}
}

func TestOSCTitle(t *testing.T) {
	s := newScreen(2, 20)
	var title string
	s.onTitle = func(t string) { title = t }
	feed(s, "\x1b]0;my title\x07")
	if title != "my title" {
		t.Fatalf("title = %q, want 'my title'", title)
	}
	feed(s, "\x1b]2;second\x1b\\") // ST-terminated
	if title != "second" {
		t.Fatalf("title = %q, want 'second'", title)
	}
}

func TestResizePreservesContent(t *testing.T) {
	s := newScreen(3, 10)
	feed(s, "keepme")
	s.resize(5, 20)
	if got := rowText(s, 0); got != "keepme" {
		t.Fatalf("after resize row0 = %q, want keepme", got)
	}
	if s.rows != 5 || s.cols != 20 {
		t.Fatalf("dims = %dx%d, want 5x20", s.rows, s.cols)
	}
}

func TestBackspaceAndTab(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "ab\bX") // backspace over b, write X
	if got := rowText(s, 0); got != "aX" {
		t.Fatalf("row0 = %q, want aX", got)
	}
	s2 := newScreen(2, 20)
	feed(s2, "\tX") // tab to col 8, then X
	if got := rowText(s2, 0); got != strings.Repeat(" ", 8)+"X" {
		t.Fatalf("row0 = %q, want 8 spaces then X", got)
	}
}
