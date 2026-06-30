package glyph

import (
	"strings"
	"testing"
)

// inputWrapLines word-wraps at spaces, hard-breaks long words, respects '\n', and the
// displayed text of each line is runes[start:end].
func TestInputWrapLines(t *testing.T) {
	disp := func(runes []rune, l inLine) string { return string(runes[l.start:l.end]) }

	// word wrap at the last fitting space
	r := []rune("the quick brown fox")
	got := inputWrapLines(r, 10)
	var lines []string
	for _, l := range got {
		lines = append(lines, disp(r, l))
	}
	// width 10: "the quick" (9) fits, "brown" would overflow → break
	if lines[0] != "the quick" {
		t.Fatalf("line0 = %q, want %q", lines[0], "the quick")
	}

	// a word longer than width hard-breaks
	r2 := []rune("supercalifragilistic")
	g2 := inputWrapLines(r2, 8)
	if len(g2) < 2 || disp(r2, g2[0]) != "supercal" {
		t.Fatalf("hard break: got %d lines, line0=%q", len(g2), disp(r2, g2[0]))
	}

	// explicit newline forces a break
	r3 := []rune("a\nb")
	g3 := inputWrapLines(r3, 80)
	if len(g3) != 2 || disp(r3, g3[0]) != "a" || disp(r3, g3[1]) != "b" {
		t.Fatalf("newline break: %d lines", len(g3))
	}

	// short text → one line
	if n := len(inputWrapLines([]rune("hi"), 80)); n != 1 {
		t.Fatalf("short text should be 1 line, got %d", n)
	}
}

// inputCursorPos maps a rune index to its wrapped (line, col).
func TestInputCursorPos(t *testing.T) {
	r := []rune("the quick brown fox")
	lines := inputWrapLines(r, 10) // ["the quick", "brown fox"] (space dropped)

	// cursor at 0 → line 0 col 0
	if l, c := inputCursorPos(lines, 0); l != 0 || c != 0 {
		t.Fatalf("cursor 0 → (%d,%d), want (0,0)", l, c)
	}
	// cursor at end → last line, col = len within that line
	if l, _ := inputCursorPos(lines, len(r)); l != len(lines)-1 {
		t.Fatalf("end cursor should be on the last line, got line %d", l)
	}
	// cursor just after "brown " start (index 10 = 'b') → line 1 col 0
	if l, c := inputCursorPos(lines, 10); l != 1 || c != 0 {
		t.Fatalf("cursor 10 → (%d,%d), want (1,0)", l, c)
	}
}

// a multiline input renders its value wrapped across several rows (not one scrolled
// line), keeping all the text. Asserted on the rendered buffer.
func TestMultiLineInputRendersWrapped(t *testing.T) {
	val := "the quick brown fox jumps over the lazy dog"
	node := Input(&val).Width(12).MultiLine()
	tmpl := Build(node)
	buf := NewBuffer(12, 8)
	tmpl.Execute(buf, 12, 8)

	nonEmpty := 0
	var all string
	for y := 0; y < 8; y++ {
		line := strings.TrimSpace(buf.GetLine(y))
		if line != "" {
			nonEmpty++
		}
		all += "|" + line
	}
	if nonEmpty < 3 {
		t.Fatalf("multiline input should wrap to several rows, got %d non-empty: %q", nonEmpty, all)
	}
	if !strings.Contains(all, "fox") || !strings.Contains(all, "dog") {
		t.Fatalf("wrapped text lost content: %q", all)
	}
}

// --- ADR 70: per-range styling on the editable Input ---

// styleRangeAt is the merge-walk core: ascending i, forward cursor, first containing
// range wins; degenerate/out-of-bounds ranges never match and never crash.
func TestStyleRangeAt(t *testing.T) {
	base := Style{FG: RGB(10, 10, 10)}
	red := Style{FG: RGB(255, 0, 0)}
	blue := Style{FG: RGB(0, 0, 255)}
	ranges := []StyleRange{{Start: 2, End: 5, Style: red}, {Start: 7, End: 9, Style: blue}}

	cur := 0
	want := []Style{base, base, red, red, red, base, base, blue, blue, base}
	for i, w := range want {
		if got := styleRangeAt(ranges, i, &cur, base); got != w {
			t.Errorf("i=%d: got %+v, want %+v", i, got.FG, w.FG)
		}
	}

	// degenerate (End<=Start) never matches
	cur = 0
	if got := styleRangeAt([]StyleRange{{Start: 3, End: 3, Style: red}}, 3, &cur, base); got != base {
		t.Error("degenerate range should not match")
	}
	// out-of-bounds Start never matches at valid indices; no panic
	cur = 0
	if got := styleRangeAt([]StyleRange{{Start: 100, End: 200, Style: red}}, 0, &cur, base); got != base {
		t.Error("out-of-range range should not match index 0")
	}
	// empty/nil ranges → base
	cur = 0
	if got := styleRangeAt(nil, 5, &cur, base); got != base {
		t.Error("nil ranges should return base")
	}
}

// A bound StyleRange colours its sub-range of the live value; runes outside it keep the
// uniform style. (Cursor sits at end, away from the styled span.)
func TestInputStyleRangesRender(t *testing.T) {
	val := "hello world"
	red := Style{FG: RGB(255, 0, 0)}
	ranges := []StyleRange{{Start: 6, End: 11, Style: red}} // "world"

	tmpl := Build(Input(&val).StyleRanges(&ranges).Width(20))
	buf := NewBuffer(20, 1)
	tmpl.Execute(buf, 20, 1)

	if r := buf.Get(6, 0).Rune; r != 'w' {
		t.Fatalf("cell 6 rune = %q, want 'w' (alignment check)", r)
	}
	if fg := buf.Get(6, 0).Style.FG; fg != (RGB(255, 0, 0)) {
		t.Errorf("styled 'w' FG = %+v, want red", fg)
	}
	if fg := buf.Get(0, 0).Style.FG; fg == (RGB(255, 0, 0)) {
		t.Errorf("unstyled 'h' should not be red, got %+v", fg)
	}
}

// No StyleRanges → uniform style, the existing path: text cells share one style.
// (Compare mid-text cells, never the caret whether the cursor sits at 0 or end.)
func TestInputStyleRangesNilUniform(t *testing.T) {
	val := "abcdef"
	tmpl := Build(Input(&val).Width(20))
	buf := NewBuffer(20, 1)
	tmpl.Execute(buf, 20, 1)
	s := buf.Get(2, 0).Style
	for x := 3; x < 5; x++ {
		if buf.Get(x, 0).Style != s {
			t.Fatalf("nil StyleRanges should render uniform style; cell %d differs from cell 2", x)
		}
	}
}

// Out-of-range entries never crash and never style a valid cell spuriously.
func TestInputStyleRangesOutOfRangeSafe(t *testing.T) {
	val := "short"
	red := Style{FG: RGB(255, 0, 0)}
	ranges := []StyleRange{{Start: 100, End: 200, Style: red}} // entirely past the value
	tmpl := Build(Input(&val).StyleRanges(&ranges).Width(20))
	buf := NewBuffer(20, 1)
	tmpl.Execute(buf, 20, 1) // must not panic
	for x := 0; x < 5; x++ {
		if buf.Get(x, 0).Style.FG == (RGB(255, 0, 0)) {
			t.Errorf("out-of-range range styled cell %d", x)
		}
	}
}

// The cursor cell wins over a styled range at the caret.
func TestInputStyleRangesCursorWins(t *testing.T) {
	state := InputState{Value: "hello", Cursor: 2}
	group := &FocusGroup{Current: 0}
	red := Style{FG: RGB(255, 0, 0)}
	ranges := []StyleRange{{Start: 0, End: 5, Style: red}} // covers the whole word incl. the caret
	cursorSty := Style{Attr: AttrInverse}

	tmpl := Build(Input(&state.Value).Field(&state).FocusGroup(group, 0).CursorStyle(cursorSty).StyleRanges(&ranges).Width(20))
	buf := NewBuffer(20, 1)
	tmpl.Execute(buf, 20, 1)

	// caret at index 2: cursor style wins over the range
	if a := buf.Get(2, 0).Style.Attr; a&AttrInverse == 0 {
		t.Errorf("caret cell should keep cursor style (inverse), got attr %v", a)
	}
	// a non-caret cell in the range keeps the range style
	if fg := buf.Get(0, 0).Style.FG; fg != (RGB(255, 0, 0)) {
		t.Errorf("non-caret range cell FG = %+v, want red", fg)
	}
}

// Off-path guarantee (ADR 70): an Input with no StyleRanges allocates nothing per render.
func BenchmarkInputRenderNilRanges(b *testing.B) {
	val := "the quick brown fox jumps"
	tmpl := Build(Input(&val).Width(25))
	buf := NewBuffer(25, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 25, 1)
	}
}

// On-path: styled render is also zero-alloc per frame (the merge-walk allocates nothing).
func BenchmarkInputRenderStyled(b *testing.B) {
	val := "the quick brown fox jumps"
	ranges := []StyleRange{{Start: 4, End: 9, Style: Style{FG: RGB(255, 0, 0)}}, {Start: 16, End: 19, Style: Style{FG: RGB(0, 0, 255)}}}
	tmpl := Build(Input(&val).StyleRanges(&ranges).Width(25))
	buf := NewBuffer(25, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 25, 1)
	}
}
