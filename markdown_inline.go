package glyph

import "strings"

// parseInlineMarkdownSpans tokenises a single line/body of inline markdown into
// styled Spans: **bold** / __bold__, *italic* / _italic_, ~~strike~~, `code`, and a
// leading "- " bullet. Styling is ATTRIBUTE-only (no colour) so it stays theme-safe —
// the render path merges each span's Style through the inherited/theme cascade, so a
// bold span inherits the surrounding colour and a theme switch restyles without
// re-tokenising. `code` renders AttrInverse (no spare Attribute bit for a code colour;
// a richer code style is a fast-follow). Adapted from wed's parseInlineMarkdown.
//
// It is a pure function: same input → same spans, no shared state. Callers cache the
// result keyed by the source string (parse-on-change) so steady-state render allocates
// nothing.
//
// v1 limitation: bold/strike recurse so they nest, but bold nested inside *italic*
// (`*x **y** z*`) does not compose to both attrs (the italic delimiter match stops at
// the inner `**`). Visible text is always preserved. recap's cases don't nest; richer
// nesting can follow if needed.
func parseInlineMarkdownSpans(text string) []Span {
	// leading "- " bullet: emit a bullet glyph, then parse the remainder inline.
	var prefix []Span
	if strings.HasPrefix(text, "- ") {
		prefix = []Span{{Text: "• "}}
		text = text[2:]
	}
	if text == "" {
		if prefix != nil {
			return prefix
		}
		return []Span{{Text: ""}}
	}
	return append(prefix, inlineSpans(text, Style{})...)
}

// inlineSpans walks runes, splitting on inline markers and merging the carried base
// style into each emitted span. base accumulates nested styles (bold inside italic).
func inlineSpans(text string, base Style) []Span {
	runes := []rune(text)
	var out []Span
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, Span{Text: cur.String(), Style: base})
			cur.Reset()
		}
	}

	i := 0
	for i < len(runes) {
		r := runes[i]

		// `code` — highest priority, contents are literal (no nested parsing)
		if r == '`' {
			end := i + 1
			for end < len(runes) && runes[end] != '`' {
				end++
			}
			if end < len(runes) {
				flush()
				out = append(out, Span{Text: string(runes[i+1 : end]), Style: withAttr(base, AttrInverse)})
				i = end + 1
				continue
			}
		}

		// **bold** / __bold__
		if i+1 < len(runes) && ((r == '*' && runes[i+1] == '*') || (r == '_' && runes[i+1] == '_')) {
			marker := string(runes[i : i+2])
			if end := strings.Index(string(runes[i+2:]), marker); end >= 0 {
				flush()
				out = append(out, inlineSpans(string(runes[i+2:i+2+end]), withAttr(base, AttrBold))...)
				i = i + 2 + end + 2
				continue
			}
		}

		// ~~strike~~
		if i+1 < len(runes) && r == '~' && runes[i+1] == '~' {
			if end := strings.Index(string(runes[i+2:]), "~~"); end >= 0 {
				flush()
				out = append(out, inlineSpans(string(runes[i+2:i+2+end]), withAttr(base, AttrStrikethrough))...)
				i = i + 2 + end + 2
				continue
			}
		}

		// *italic* / _italic_ (single marker; doubles handled above)
		if (r == '*' || r == '_') && !(i+1 < len(runes) && runes[i+1] == r) {
			if end := strings.Index(string(runes[i+1:]), string(r)); end >= 0 {
				flush()
				out = append(out, inlineSpans(string(runes[i+1:i+1+end]), withAttr(base, AttrItalic))...)
				i = i + 1 + end + 1
				continue
			}
		}

		cur.WriteRune(r)
		i++
	}
	flush()
	if len(out) == 0 {
		return []Span{{Text: "", Style: base}}
	}
	return out
}

func withAttr(s Style, a Attribute) Style {
	s.Attr = s.Attr.With(a)
	return s
}
