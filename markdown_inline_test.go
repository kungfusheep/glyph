package glyph

import (
	"testing"
	"unsafe"
)

func TestParseInlineMarkdownSpans(t *testing.T) {
	spans := parseInlineMarkdownSpans("a **b** `c` *d* ~~e~~")
	var text string
	attr := map[string]Attribute{}
	for _, s := range spans {
		text += s.Text
		attr[s.Text] = s.Style.Attr
	}
	if text != "a b c d e" {
		t.Fatalf("reassembled text = %q, want %q", text, "a b c d e")
	}
	if attr["b"]&AttrBold == 0 {
		t.Error("**b** should be bold")
	}
	if attr["c"]&AttrInverse == 0 {
		t.Error("`c` should render as code (AttrInverse)")
	}
	if attr["d"]&AttrItalic == 0 {
		t.Error("*d* should be italic")
	}
	if attr["e"]&AttrStrikethrough == 0 {
		t.Error("~~e~~ should be strikethrough")
	}

	// bold nested INSIDE bold/strike composes (those recurse); nested inside italic
	// is a documented v1 limitation, but must never corrupt the visible text.
	var nestedText string
	for _, s := range parseInlineMarkdownSpans("*x **y** z*") {
		nestedText += s.Text
	}
	if nestedText != "x y z" {
		t.Errorf("nested markdown must preserve visible text, got %q", nestedText)
	}
	// bold-inside-strike DOES compose (strike recurses)
	for _, s := range parseInlineMarkdownSpans("~~a **b** c~~") {
		if s.Text == "b" && (s.Style.Attr&AttrBold == 0 || s.Style.Attr&AttrStrikethrough == 0) {
			t.Errorf("**b** inside ~~strike~~ should be bold+strike, got %b", s.Style.Attr)
		}
	}

	// leading bullet
	if b := parseInlineMarkdownSpans("- item"); b[0].Text != "• " {
		t.Errorf("'- ' should become a bullet glyph, got %q", b[0].Text)
	}
}

func TestMarkdownSpansUsesInlineMarkdownParser(t *testing.T) {
	spans := MarkdownSpans("a **b** `c`")
	var text string
	attr := map[string]Attribute{}
	for _, s := range spans {
		text += s.Text
		attr[s.Text] = s.Style.Attr
	}
	if text != "a b c" {
		t.Fatalf("reassembled text = %q, want %q", text, "a b c")
	}
	if attr["b"]&AttrBold == 0 {
		t.Error("**b** should be bold")
	}
	if attr["c"]&AttrInverse == 0 {
		t.Error("`c` should render as code")
	}
}

// the cache must re-tokenise ONLY when the source changes: a cache hit returns the
// SAME backing slice (no realloc/re-parse), which is what keeps steady-state render
// free of tokenisation cost.
func TestMarkdownCacheParsesOnChange(t *testing.T) {
	rt := &opRichText{markdown: true}
	a1 := rt.mdSpansFor(nil, "**hello**")
	a2 := rt.mdSpansFor(nil, "**hello**") // unchanged → cache hit
	if &a1[0] != &a2[0] {
		t.Fatal("same source should return the cached spans (no re-tokenise)")
	}
	b := rt.mdSpansFor(nil, "**world**") // changed → re-tokenise
	if len(b) > 0 && len(a1) > 0 && &b[0] == &a1[0] {
		t.Fatal("changed source should re-tokenise to a fresh slice")
	}
}

func TestMarkdownRendersInlineStyles(t *testing.T) {
	body := "**bold** plain"
	tmpl := Build(VBox(Rich(&body).Markdown()))
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)

	if got := buf.Get(0, 0).Style.Attr; got&AttrBold == 0 {
		t.Errorf("first cell ('b' of bold) should be bold, attr=%b", got)
	}
	// the word "plain" (after "bold ") must NOT be bold
	if got := buf.Get(5, 0).Style.Attr; got&AttrBold != 0 {
		t.Errorf("cell at the plain run should not be bold, attr=%b", got)
	}

	// re-read per frame: mutate the source, re-render, styling follows
	body = "plain **now**"
	buf.Clear()
	tmpl.Execute(buf, 20, 3)
	if got := buf.Get(0, 0).Style.Attr; got&AttrBold != 0 {
		t.Errorf("after change, first cell ('p') should not be bold, attr=%b", got)
	}
}

// day-one ForEach regression (the binding rule): a per-item bound markdown string must
// tokenise its OWN value — two items render their own text and styling, not a shared one.
func TestMarkdownForEachPerItem(t *testing.T) {
	type msg struct{ Body string }
	msgs := []msg{{Body: "**alpha**"}, {Body: "*beta*"}}

	tmpl := Build(VBox(
		ForEach(&msgs, func(m *msg) Component {
			return Rich(&m.Body).Markdown()
		}),
	))
	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)

	// row 0 = "alpha" bold, row 1 = "beta" italic — each its own value
	line0 := extractLine(buf, 0, 10)
	line1 := extractLine(buf, 1, 10)
	if line0[:5] != "alpha" {
		t.Fatalf("row 0 = %q, want alpha", line0)
	}
	if line1[:4] != "beta" {
		t.Fatalf("row 1 = %q, want beta", line1)
	}
	if buf.Get(0, 0).Style.Attr&AttrBold == 0 {
		t.Error("row 0 should be bold (its own **alpha**)")
	}
	if buf.Get(0, 1).Style.Attr&AttrItalic == 0 {
		t.Error("row 1 should be italic (its own *beta*) — not the first item's bold")
	}
	if buf.Get(0, 1).Style.Attr&AttrBold != 0 {
		t.Error("row 1 must NOT inherit row 0's bold — frozen-placeholder bug")
	}
}

// steady-state render of unchanged markdown re-uses the cache (no re-tokenise); this
// guards against a regression to per-frame tokenisation.
func BenchmarkMarkdownSteadyState(b *testing.B) {
	body := "the **quick** brown `fox` jumps *over* the ~~lazy~~ dog"
	tmpl := Build(VBox(Rich(&body).Markdown()))
	buf := NewBuffer(60, 3)
	tmpl.Execute(buf, 60, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 60, 3)
	}
}

// the per-item ForEach cache must stay bounded: a slice that grows by append
// reallocates, orphaning every elemBase key. Simulate heavy key churn and assert the
// map doesn't grow without bound.
func TestMarkdownCacheEvictsOrphanedKeys(t *testing.T) {
	rt := &opRichText{markdown: true}
	keys := make([]int, mdCacheEvict*3) // distinct real addresses as fake elemBases
	for i := range keys {
		rt.mdSpansFor(unsafe.Pointer(&keys[i]), "**x**")
	}
	if len(rt.mdCacheMap) > mdCacheEvict {
		t.Fatalf("mdCacheMap unbounded: %d entries (cap %d) — orphaned keys not evicted", len(rt.mdCacheMap), mdCacheEvict)
	}
}

// MarkdownWidth must report the RENDERED width (markers consumed), matching what
// Rich(&s).Markdown() draws — not the raw string length. Content-hug sizing needs this.
func TestMarkdownWidth(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"plain", 5},
		{"**bold**", 4},  // renders "bold"
		{"a *b* `c`", 5}, // "a b c"
		{"~~no~~", 2},    // "no"
		{"- item", 6},    // "• item" (bullet + space + item = 2+? -> "• "=2, "item"=4)
		{"", 0},
	}
	for _, c := range cases {
		if got := MarkdownWidth(c.src); got != c.want {
			// rebuild rendered text for the failure message
			var rendered string
			for _, s := range parseInlineMarkdownSpans(c.src) {
				rendered += s.Text
			}
			t.Errorf("MarkdownWidth(%q) = %d, want %d (renders %q)", c.src, got, c.want, rendered)
		}
	}
}

// MarkdownSpansInto with a reused buffer must tokenise WITHOUT allocating — spans
// reference substrings of the input (no copy), the caller owns the slice growth.
// This is the point of the storage-passing variant: per-frame measurement, zero-alloc.
func TestMarkdownSpansIntoZeroAlloc(t *testing.T) {
	buf := make([]Span, 0, 32)
	allocs := testing.AllocsPerRun(200, func() {
		buf = MarkdownSpansInto(buf[:0], "the **quick** brown `fox` jumps *over* the ~~lazy~~ dog")
	})
	if allocs != 0 {
		t.Fatalf("MarkdownSpansInto with a reused buffer should be zero-alloc, got %.1f allocs/op", allocs)
	}
}
