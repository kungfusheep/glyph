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
