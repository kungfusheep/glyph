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

// TestAltScreenPreservesPrimary is the property that makes the alt screen worth
// having: a full-screen program paints a blank grid, and what the shell had on
// screen is still there when it leaves.
func TestAltScreenPreservesPrimary(t *testing.T) {
	s := newScreen(4, 20)
	feed(s, "shell output")

	feed(s, "\x1b[?1049h") // program takes over
	if got := rowText(s, 0); got != "" {
		t.Fatalf("alt grid row0 = %q, want blank", got)
	}
	feed(s, "full screen ui")
	if got := rowText(s, 0); got != "full screen ui" {
		t.Fatalf("alt grid row0 = %q, want the program's output", got)
	}

	feed(s, "\x1b[?1049l") // program exits
	if got := rowText(s, 0); got != "shell output" {
		t.Fatalf("row0 after leaving alt = %q, want the shell's output back", got)
	}
}

// TestAltScreenRestoresCursor covers the half of 1049 that 47/1047 do not do.
func TestAltScreenRestoresCursor(t *testing.T) {
	s := newScreen(4, 20)
	feed(s, "\x1b[3;7H") // park the cursor at row 3, col 7 (1-based)
	cx, cy := s.cx, s.cy

	feed(s, "\x1b[?1049h")
	if s.cx != 0 || s.cy != 0 {
		t.Fatalf("alt entered at (%d,%d), want home", s.cx, s.cy)
	}
	feed(s, "\x1b[2;2Hmoved")

	feed(s, "\x1b[?1049l")
	if s.cx != cx || s.cy != cy {
		t.Errorf("cursor = (%d,%d) after leaving alt, want (%d,%d)", s.cx, s.cy, cx, cy)
	}
}

// TestAltScreenLegacyForms: 47 and 1047 swap the grid but leave the cursor alone.
func TestAltScreenLegacyForms(t *testing.T) {
	for _, mode := range []string{"47", "1047"} {
		t.Run(mode, func(t *testing.T) {
			s := newScreen(4, 20)
			feed(s, "primary")
			feed(s, "\x1b[?"+mode+"h")
			feed(s, "alt")
			if got := rowText(s, 0); got != "alt" {
				t.Fatalf("alt row0 = %q, want %q", got, "alt")
			}
			feed(s, "\x1b[?"+mode+"l")
			if got := rowText(s, 0); got != "primary" {
				t.Errorf("row0 = %q after leaving alt, want %q", got, "primary")
			}
		})
	}
}

// TestAltScreenResizeReshapesBoth: a resize taken while a program owns the screen
// must not leave the primary at the old geometry to be restored into.
func TestAltScreenResizeReshapesBoth(t *testing.T) {
	s := newScreen(4, 20)
	feed(s, "kept")

	feed(s, "\x1b[?1049h")
	s.resize(6, 30)
	feed(s, "\x1b[?1049l")

	if s.rows != 6 || s.cols != 30 {
		t.Fatalf("geometry = %dx%d, want 6x30", s.rows, s.cols)
	}
	if len(s.cells) != 6*30 {
		t.Fatalf("primary grid is %d cells, want %d — restored into a stale grid", len(s.cells), 6*30)
	}
	if got := rowText(s, 0); got != "kept" {
		t.Errorf("row0 = %q, want %q", got, "kept")
	}
}

// fillRows paints "L01".."L<rows>", one per row, leaving the cursor parked on
// cursorRow (0-based) — the shape the resize rules are stated over.
func fillRows(s *screen, cursorRow int) {
	for y := 0; y < s.rows; y++ {
		feed(s, "\x1b["+itoa(y+1)+";1H")
		n := itoa(y + 1)
		if len(n) < 2 {
			n = "0" + n
		}
		feed(s, "L"+n)
	}
	feed(s, "\x1b["+itoa(cursorRow+1)+";1H")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A shrink drops rows off the TOP, enough to keep the cursor on screen. Dropping
// them off the bottom is what loses a program's bottom-anchored UI. Both cases are
// the behaviour xterm and tmux were measured to produce.
func TestShrinkKeepsTheCursorsRows(t *testing.T) {
	tests := []struct {
		name      string
		cursorRow int
		wantTop   string
		wantBot   string
		wantCy    int
	}{
		// the cursor already fits, so nothing scrolls and the top survives
		{"cursor fits", 4, "L01", "L06", 4},
		// the cursor is below the new bottom, so the top four rows go
		{"cursor at bottom", 9, "L05", "L10", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScreen(10, 20)
			fillRows(s, tt.cursorRow)
			s.resize(6, 20)

			if got := rowText(s, 0); got != tt.wantTop {
				t.Errorf("row0 = %q, want %q", got, tt.wantTop)
			}
			if got := rowText(s, 5); got != tt.wantBot {
				t.Errorf("row5 = %q, want %q — the bottom-anchored rows must survive", got, tt.wantBot)
			}
			if s.cy != tt.wantCy {
				t.Errorf("cy = %d, want %d — the cursor must land on the row it was on", s.cy, tt.wantCy)
			}
		})
	}
}

// Growing appends blank rows at the bottom and leaves the content and cursor alone.
func TestGrowAppendsBlankRowsBelow(t *testing.T) {
	s := newScreen(6, 20)
	fillRows(s, 5)
	s.resize(10, 20)

	if got := rowText(s, 0); got != "L01" {
		t.Errorf("row0 = %q, want L01", got)
	}
	if got := rowText(s, 5); got != "L06" {
		t.Errorf("row5 = %q, want L06", got)
	}
	if got := rowText(s, 6); got != "" {
		t.Errorf("row6 = %q, want blank", got)
	}
	if s.cy != 5 {
		t.Errorf("cy = %d, want 5 — a grow must not move the cursor", s.cy)
	}
}

// A resize taken while the alternate screen is active scrolls the primary by the
// PRIMARY's cursor, not the alt program's, and moves the saved cursor with it. A
// shared offset leaves cursor and content diverged the moment the program exits.
func TestAltResizeScrollsThePrimaryByItsOwnCursor(t *testing.T) {
	for _, enter := range []string{"\x1b[?1049h", "\x1b[?47h"} {
		t.Run(enter, func(t *testing.T) {
			s := newScreen(10, 20)
			fillRows(s, 3) // the primary's cursor sits on L04, which already fits in 6 rows

			feed(s, enter)
			feed(s, "\x1b[10;1Halt") // the alt program parks at the bottom
			s.resize(6, 20)
			feed(s, "\x1b[?1049l\x1b[?47l")

			// the primary's cursor fitted, so its rows must not have scrolled at all
			if got := rowText(s, 0); got != "L01" {
				t.Errorf("primary row0 = %q, want L01 — scrolled by the alt program's cursor", got)
			}
			if got := rowText(s, 3); got != "L04" {
				t.Errorf("primary row3 = %q, want L04", got)
			}
			if s.altSavedCy != 3 {
				t.Errorf("altSavedCy = %d, want 3 — the saved cursor must track its own rows", s.altSavedCy)
			}
		})
	}
}

// The alt grid is blanked and homed on every entry, so a resize that preserved its
// cells would be preserving nothing a program can ever read.
func TestAltGridIsBlankOnEveryEntry(t *testing.T) {
	s := newScreen(10, 20)
	feed(s, "\x1b[?1049h")
	feed(s, "\x1b[10;1Hbottom")
	s.resize(6, 20)
	feed(s, "\x1b[?1049l")
	feed(s, "\x1b[?1049h")

	for y := 0; y < s.rows; y++ {
		if got := rowText(s, y); got != "" {
			t.Errorf("alt row%d = %q, want blank on entry", y, got)
		}
	}
	if s.cx != 0 || s.cy != 0 {
		t.Errorf("alt cursor = %d,%d, want 0,0", s.cx, s.cy)
	}
}

// Leaving the alternate screen after a resize must put the cursor back on the row it
// was on, not on whatever row now holds that number. The primary scrolls by its own
// cursor and the saved cursor moves with it, so the two cannot drift apart.
func TestLeavingAltLandsOnTheRowItLeft(t *testing.T) {
	tests := []struct {
		name       string
		cursorRow  int
		wantRow    string
		wantCursor int
	}{
		// the primary's cursor is below the new bottom, so its rows scroll under it
		{"primary scrolled", 17, "L18", 7},
		// it already fits, so nothing moves
		{"primary untouched", 3, "L04", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScreen(20, 20)
			fillRows(s, tt.cursorRow)

			feed(s, "\x1b[?1049h")
			feed(s, "\x1b[20;1Halt program at the bottom") // its own cursor, not the primary's
			s.resize(8, 20)
			feed(s, "\x1b[?1049l")

			if s.cy != tt.wantCursor {
				t.Errorf("cy = %d, want %d", s.cy, tt.wantCursor)
			}
			if got := rowText(s, s.cy); got != tt.wantRow {
				t.Errorf("the cursor sits on %q, want %q — cursor and content drifted apart", got, tt.wantRow)
			}
		})
	}
}
