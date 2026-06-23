package glyph

import "strings"

// MarkdownSpansInto tokenises a single line/body of inline markdown into styled Spans,
// APPENDING them to dst and returning the extended slice. Pass a reused buffer (e.g.
// buf[:0]) to tokenise without allocating — markers are ASCII and each span's Text
// references a substring of the input (no copy), so the only growth is dst itself. Pass
// nil for a fresh owned slice.
//
// Grammar: **bold** / __bold__, *italic* / _italic_, ~~strike~~, `code`, and a leading
// "- " bullet. Styling is ATTRIBUTE-only (no colour) so it stays theme-safe — the render
// path merges each span's Style through the inherited/theme cascade, so a bold span
// inherits the surrounding colour and a theme switch restyles without re-tokenising.
// `code` renders AttrInverse (no spare Attribute bit for a code colour; a richer code
// style is a fast-follow).
//
// v1 limitation: bold/strike recurse so they nest, but bold nested inside *italic*
// (*x **y** z*) does not compose to both attrs (the italic delimiter match stops at the
// inner **). Visible text is always preserved. Common chat/prose bodies don't nest;
// richer nesting can follow if needed.
//
// Do NOT retain a slice produced into caller-supplied storage past the next reuse — for
// a long-lived/cached result pass nil so the slice is freshly owned.
func MarkdownSpansInto(dst []Span, text string) []Span {
	if strings.HasPrefix(text, "- ") {
		dst = append(dst, Span{Text: "• "})
		text = text[2:]
	}
	return inlineSpansInto(dst, text, Style{})
}

// inlineSpansInto scans bytes (markers are ASCII), appending spans to dst. Plain runs
// are emitted as substrings of text (zero-copy); nested bold/strike/italic recurse on
// the inner substring with the accumulated base style.
func inlineSpansInto(dst []Span, text string, base Style) []Span {
	start := len(dst)
	runStart := 0
	flush := func(end int) {
		if end > runStart {
			dst = append(dst, Span{Text: text[runStart:end], Style: base})
		}
	}

	i := 0
	for i < len(text) {
		switch c := text[i]; c {
		case '`': // code — literal contents, no nested parsing
			if rel := strings.IndexByte(text[i+1:], '`'); rel >= 0 {
				end := i + 1 + rel
				flush(i)
				dst = append(dst, Span{Text: text[i+1 : end], Style: withAttr(base, AttrInverse)})
				i = end + 1
				runStart = i
				continue
			}
		case '*', '_':
			if i+1 < len(text) && text[i+1] == c { // **bold** / __bold__
				marker := text[i : i+2]
				if rel := strings.Index(text[i+2:], marker); rel >= 0 {
					end := i + 2 + rel
					flush(i)
					dst = inlineSpansInto(dst, text[i+2:end], withAttr(base, AttrBold))
					i = end + 2
					runStart = i
					continue
				}
			} else if rel := strings.IndexByte(text[i+1:], c); rel >= 0 { // *italic* / _italic_
				end := i + 1 + rel
				flush(i)
				dst = inlineSpansInto(dst, text[i+1:end], withAttr(base, AttrItalic))
				i = end + 1
				runStart = i
				continue
			}
		case '~': // ~~strike~~
			if i+1 < len(text) && text[i+1] == '~' {
				if rel := strings.Index(text[i+2:], "~~"); rel >= 0 {
					end := i + 2 + rel
					flush(i)
					dst = inlineSpansInto(dst, text[i+2:end], withAttr(base, AttrStrikethrough))
					i = end + 2
					runStart = i
					continue
				}
			}
		}
		i++
	}
	flush(i)
	if len(dst) == start { // empty input/inner → one empty span (preserve prior contract)
		dst = append(dst, Span{Text: "", Style: base})
	}
	return dst
}

// parseInlineMarkdownSpans tokenises inline markdown into a freshly-owned span slice.
// Internal callers that retain/cache the result use this (nil storage).
func parseInlineMarkdownSpans(text string) []Span {
	return MarkdownSpansInto(nil, text)
}

// MarkdownSpans tokenises inline markdown into styled spans using the same parser as
// Rich(&s).Markdown(). For repeated calls on a hot path, MarkdownSpansInto with a reused
// buffer avoids the slice allocation.
func MarkdownSpans(text string) []Span {
	return MarkdownSpansInto(nil, text)
}

func withAttr(s Style, a Attribute) Style {
	s.Attr = s.Attr.With(a)
	return s
}

// MarkdownWidth returns the rendered cell-width of an inline-markdown string — what
// Rich(&s).Markdown() actually draws, with the markers (**, *, ~~, `, leading "- ")
// consumed. Use it to size content to markdown output: measuring the raw string
// over-counts the markers (a "**bold**" line measures 8 but renders 4). For a hot path
// that measures per frame, tokenise with MarkdownSpansInto(buf[:0], s) and sum the
// widths yourself to avoid the per-call slice allocation.
func MarkdownWidth(s string) int {
	w := 0
	for _, span := range MarkdownSpansInto(nil, s) {
		w += StringWidth(span.Text)
	}
	return w
}
