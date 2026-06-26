package glyph

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// Single control point for text measurement. Wrapping stdlib and third-party
// calls lets us change width semantics in one place (e.g. emoji table version,
// ambiguous-width handling, terminal-specific overrides) without chasing call
// sites. All wrappers are leaf functions the compiler inlines freely.

// RuneWidth returns the display-cell width of a rune. ASCII is width 1 on the
// fast path; anything above U+1100 defers to the runewidth table. Zero-width
// runes (combining marks) are reported as 1 because terminals treat them as
// advancing the cursor by 1 in most cases.
func RuneWidth(r rune) int {
	if r < 0x1100 {
		return 1
	}
	w := runewidth.RuneWidth(r)
	if w == 0 {
		return 1
	}
	return w
}

// StringWidth returns the total display-cell width of s.
func StringWidth(s string) int {
	return runewidth.StringWidth(s)
}

// RuneCount returns the number of runes in s. Use this when you need a
// character count (e.g. indexing into a rune slice), not a display width.
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

// TruncateWidth returns s truncated to at most maxCols display columns, measured by
// RuneWidth. It never splits a wide rune across the boundary: a rune that would cross
// maxCols is excluded, so the result may be SHORT of maxCols (padding to a fixed width
// is the container's job, not this function's). maxCols <= 0 returns "".
//
// This is the safe alternative to s[:n] / string([]rune(s)[:n]), which clip by bytes or
// runes — for ASCII those equal columns, but a wide rune (CJK, emoji) makes a rune-count
// clip wider than the budget and overflow its pane.
func TruncateWidth(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	cols := 0
	for i, r := range s {
		w := RuneWidth(r)
		if cols+w > maxCols {
			return s[:i]
		}
		cols += w
	}
	return s
}

// TruncateWidthEllipsis is TruncateWidth, but when s is actually truncated the result
// ends with "…" and still fits within maxCols (the ellipsis costs RuneWidth('…')
// columns, reserved from the same table everything else uses — "…" is U+2026, itself
// ambiguous-width). If s already fits, it is returned unchanged. maxCols <= 0 returns "";
// maxCols too small for body + ellipsis returns the ellipsis alone (keeps the contract
// "truncated ⇒ ends with …" unconditional).
//
// Correctness is defined relative to glyph's RuneWidth: a terminal that renders an
// ambiguous rune (including "…") at a different width sees the same off-by-one it would
// hand-rolling the loop — the cure is width-policy agreement, a separate concern.
func TruncateWidthEllipsis(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if StringWidth(s) <= maxCols {
		return s
	}
	body := maxCols - RuneWidth('…')
	if body <= 0 {
		return "…"
	}
	return TruncateWidth(s, body) + "…"
}
