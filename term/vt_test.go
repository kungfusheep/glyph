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

func TestUTF8Printable(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "héllo → ✓")
	if got := rowText(s, 0); got != "héllo → ✓" {
		t.Fatalf("row0 = %q, want %q", got, "héllo → ✓")
	}
	if s.cx != 9 {
		t.Fatalf("cx = %d, want 9 (one column per rune)", s.cx)
	}
}

func TestUTF8SplitAcrossWrites(t *testing.T) {
	s := newScreen(2, 20)
	box := []byte("─") // e2 94 80
	for _, b := range box {
		s.write([]byte{b}) // one byte per pty read
	}
	if got := s.cellAt(0, 0).Rune; got != '─' {
		t.Fatalf("cell(0,0) = %q (U+%04X), want ─", got, got)
	}
	if s.cx != 1 {
		t.Fatalf("cx = %d, want 1", s.cx)
	}
}

func TestUTF8InvalidByteIsReplacementChar(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "a\xffb")
	if got := s.cellAt(1, 0).Rune; got != '�' {
		t.Fatalf("cell(1,0) = U+%04X, want U+FFFD", got)
	}
	if got := s.cellAt(2, 0).Rune; got != 'b' {
		t.Fatalf("cell(2,0) = %q, want b — parser must resync", got)
	}
}

func TestDECSpecialGraphicsCharset(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "\x1b(0lqqk\x1b(Bqx")
	if got := rowText(s, 0); got != "┌──┐qx" {
		t.Fatalf("row0 = %q, want %q (graphics on, then ASCII again)", got, "┌──┐qx")
	}
}

func TestWideRuneOccupiesTwoColumns(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "a世b")
	if got := s.cellAt(1, 0).Rune; got != '世' {
		t.Fatalf("cell(1,0) = %q, want 世", got)
	}
	if got := s.cellAt(2, 0).Rune; got != 0 {
		t.Fatalf("cell(2,0) = U+%04X, want 0 (placeholder)", got)
	}
	if got := s.cellAt(3, 0).Rune; got != 'b' {
		t.Fatalf("cell(3,0) = %q, want b", got)
	}
	if s.cx != 4 {
		t.Fatalf("cx = %d, want 4", s.cx)
	}
}

func TestWideRuneWrapsWholeAtRightMargin(t *testing.T) {
	s := newScreen(2, 4)
	feed(s, "abc世")
	if got := s.cellAt(3, 0).Rune; got != ' ' {
		t.Fatalf("cell(3,0) = %q, want blank — a wide rune must not straddle the margin", got)
	}
	if got := s.cellAt(0, 1).Rune; got != '世' {
		t.Fatalf("cell(0,1) = %q, want 世 on the next row", got)
	}
}

func TestOverwritingWideRuneBlanksItsOtherHalf(t *testing.T) {
	s := newScreen(2, 20)
	feed(s, "世\r")      // wide pair at cols 0..1
	feed(s, "\x1b[2Gx") // land on col 1, the placeholder
	if got := s.cellAt(0, 0).Rune; got != ' ' {
		t.Fatalf("cell(0,0) = %q, want blank — the orphaned left half must clear", got)
	}
	if got := s.cellAt(1, 0).Rune; got != 'x' {
		t.Fatalf("cell(1,0) = %q, want x", got)
	}
}

// TestDSRCursorPositionReport pins the query neovim makes at startup. Without a
// reply it reports "did not detect DSR response from terminal" and degrades.
func TestDSRCursorPositionReport(t *testing.T) {
	s := newScreen(24, 80)
	var got []byte
	s.onReply = func(b []byte) { got = append(got, b...) }

	feed(s, "\x1b[3;5H") // move the cursor to row 3, col 5 (1-based)
	feed(s, "\x1b[6n")   // "where is the cursor?"

	if want := "\x1b[3;5R"; string(got) != want {
		t.Fatalf("DSR reply = %q, want %q", got, want)
	}
}

func TestDSRStatusAndDeviceAttributes(t *testing.T) {
	s := newScreen(24, 80)
	var got []byte
	s.onReply = func(b []byte) { got = append(got, b...) }

	feed(s, "\x1b[5n") // "are you ok?"
	feed(s, "\x1b[c")  // "what are you?"

	if want := "\x1b[0n\x1b[?1;2c"; string(got) != want {
		t.Fatalf("status+DA reply = %q, want %q", got, want)
	}
}

// TestReplyNotSentUnderScreenLock guards the same lock order onTitle needed: the
// reply goes back through the pty on the reader goroutine, and a host that takes
// its own lock there would deadlock against the renderer waiting on s.mu.
func TestReplyNotSentUnderScreenLock(t *testing.T) {
	s := newScreen(24, 80)
	var held bool
	s.onReply = func([]byte) {
		if s.mu.TryLock() {
			s.mu.Unlock()
		} else {
			held = true
		}
	}

	feed(s, "\x1b[6n")

	if held {
		t.Fatal("onReply fired while write() held the screen lock")
	}
}

// TestCSIParameterPrefixSwallowed feeds the escape sequences a hosted agent emits
// in its first frame. The '<' '=' '>' prefixes are parameter bytes like '?', so an
// unimplemented one must be consumed, not abort the sequence and leak its
// parameters into the grid as printable text.
func TestCSIParameterPrefixSwallowed(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"alt screen", "\x1b[?1049h"},
		{"kitty keyboard push", "\x1b[>1u"},
		{"kitty keyboard query", "\x1b[?u"},
		{"modifyOtherKeys", "\x1b[>4;2m"},
		{"mouse reporting", "\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h"},
		{"bracketed paste + focus", "\x1b[?2004h\x1b[?1004h"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newScreen(4, 40)
			feed(s, tc.in)
			feed(s, "OK")
			if got := rowText(s, 0); got != "OK" {
				t.Errorf("row0 = %q, want %q — escape leaked into the grid", got, "OK")
			}
		})
	}
}
