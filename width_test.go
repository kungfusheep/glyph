package glyph

import (
	"strings"
	"testing"
)

func TestTruncateWidth(t *testing.T) {
	cases := []struct {
		s       string
		maxCols int
		want    string
	}{
		{"hello world", 5, "hello"},
		{"hi", 5, "hi"}, // fits, unchanged
		{"hello", 0, ""},
		{"hello", -3, ""},
		{"hello", 5, "hello"}, // exact fit
		{strings.Repeat("世", 5), 4, "世世"},  // 5 wide runes, budget 4 -> 2 runes (width 4)
		{"a世", 2, "a"},                      // 'a'(1)+世(2)=3>2 -> exclude 世, result short (width 1)
		{"世a", 2, "世"},                      // 世(2) fits exactly at 2; 'a' would be 3 -> excluded
	}
	for _, c := range cases {
		got := TruncateWidth(c.s, c.maxCols)
		if got != c.want {
			t.Errorf("TruncateWidth(%q, %d) = %q, want %q", c.s, c.maxCols, got, c.want)
		}
		// the invariant the rune-count version fails: result never exceeds the budget.
		if StringWidth(got) > c.maxCols && c.maxCols > 0 {
			t.Errorf("TruncateWidth(%q, %d) = %q has width %d > %d", c.s, c.maxCols, got, StringWidth(got), c.maxCols)
		}
	}
}

func TestTruncateWidthEllipsis(t *testing.T) {
	if got := TruncateWidthEllipsis("hi", 5); got != "hi" {
		t.Errorf("fits: got %q, want hi", got)
	}
	if got := TruncateWidthEllipsis("hello world", 8); got != "hello w…" {
		t.Errorf("truncate: got %q, want 'hello w…'", got)
	}
	if got := TruncateWidthEllipsis("hello", 1); got != "…" {
		t.Errorf("maxCols 1: got %q, want '…' (ellipsis-only)", got)
	}
	if got := TruncateWidthEllipsis("hello", 0); got != "" {
		t.Errorf("maxCols 0: got %q, want ''", got)
	}
	// when truncated, the result always fits the budget.
	for _, mc := range []int{2, 3, 5, 8} {
		got := TruncateWidthEllipsis("the quick brown fox", mc)
		if StringWidth(got) > mc {
			t.Errorf("TruncateWidthEllipsis(.., %d) = %q width %d > %d", mc, got, StringWidth(got), mc)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("TruncateWidthEllipsis(.., %d) = %q, want it to end with … (truncated)", mc, got)
		}
	}
}
