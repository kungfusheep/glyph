# ADR 47: TruncateWidth: a display-width-aware string clip primitive

- status: accepted
- date: 2026-06-26 16:25:02
- proposer: Glyph Smith@tui
- parties: tui, recap
- deliberation: recap proposal show 47 (2 comments)

# TruncateWidth: a display-width-aware string clip primitive

## Context and Problem Statement

Consumers routinely clip a string to a column budget — a pane width, a fixed label
slot, a status field. glyph exposes the measurement primitives (`RuneWidth`,
`StringWidth`) and a buffer-write clip (`Buffer.WriteStringClipped`), but there is NO
standalone function that returns a string truncated to N display columns. So every
consumer hand-rolls the loop:

    cols := 0
    var b strings.Builder
    for _, r := range s {
        w := RuneWidth(r)
        if cols+w > max { break }
        cols += w
        b.WriteRune(r)
    }

The intuitive shortcut — `s[:max]` or `string([]rune(s)[:max])` — is the trap: it clips
by BYTES or RUNES, not display columns. For pure ASCII the two agree, so the bug hides;
the moment the text contains a wide rune (CJK, emoji) the rune-count clip produces a
string WIDER than the budget, which overflows its pane and corrupts at render. This has
already bitten a consumer (a diff pane clipping by rune count overflowed once the content
held wide runes). It is the same silent-width family as the `WidthPct` fraction trap:
the wrong version looks right and passes naive tests.

## Considered Options

1. **Status quo** — document `RuneWidth`/`StringWidth` and let consumers hand-roll. Keeps
   the footgun; every clip site re-implements the loop and any one can reach for the
   rune-count shortcut.
2. **A `TruncateWidth` primitive** (+ an ellipsis variant) — one shared, tested function
   that clips by display width, so consumer clip sites become a one-liner and the
   rune-count trap has no reason to exist.

## Decision Outcome

Chosen option: **2** — add display-width-aware truncation primitives:

    // TruncateWidth returns s truncated to at most maxCols display columns, measured by
    // RuneWidth. It never splits a wide rune across the boundary (a rune that would cross
    // maxCols is excluded). maxCols <= 0 returns "".
    func TruncateWidth(s string, maxCols int) string

    // TruncateWidthEllipsis is TruncateWidth, but when s is actually truncated the result
    // ends with "…" and still fits within maxCols (the ellipsis costs one column). If s
    // already fits, it is returned unchanged.
    func TruncateWidthEllipsis(s string, maxCols int) string

Because:
- it kills the len/`[]rune`-count trap at the source for every consumer — the safe path
  becomes the easy path, and the dangerous shortcut becomes unnecessary.
- it is built on the `RuneWidth` glyph already owns, so glyph's clip and glyph's render
  agree on width by construction.
- the ellipsis variant covers the other ubiquitous case (a label that shows "…" when it
  doesn't fit) so consumers don't hand-roll that boundary math either (reserving a column
  for the ellipsis is its own off-by-one trap).

This earns its surface: there is a proven multi-site need (a consumer clips to a column
budget in several places and hand-rolls the loop each time), not a speculative one. It is
NOT framed as a fix for any specific rendering bug — it is a primitive that removes a
recurring footgun.

## Technical

- Pure functions in the width control point (`width.go`), alongside `RuneWidth`/
  `StringWidth`. No state, no allocation beyond the result string.
- `TruncateWidth`: accumulate `RuneWidth` over runes, stop before exceeding `maxCols`,
  exclude a wide rune that would cross the boundary (never emit a half rune). When a wide
  rune is excluded, the result is left SHORT (e.g. width `maxCols-1`) — it is NOT padded
  to hit `maxCols` exactly. Padding is the container's job; a caller composing fields
  wants the true clipped string.
- `TruncateWidthEllipsis`: if the full string fits, return it; otherwise truncate to
  `maxCols - RuneWidth('…')` and append "…". Reserve by `RuneWidth('…')`, NOT a hardcoded
  1 — the ellipsis `…` is U+2026, itself East-Asian AMBIGUOUS width, so its reserved
  columns must come from the same table everything else uses. For `maxCols < 1`, return ""
  (no room even for the ellipsis); `maxCols == 1` returns ellipsis-only (keeps the contract
  "truncated ⇒ ends with …" unconditional, and one rune of unknown width is no more
  informative than "…").
- Combining marks / zero-width runes follow `RuneWidth`'s existing rule (reported as 1),
  so behaviour is consistent with the rest of glyph's measurement.

**Correctness is defined relative to glyph's `RuneWidth`.** This helper applies glyph's
width table; it does not reconcile it with a display layer that measures ambiguous-width
runes differently. A consumer whose terminal/multiplexer renders an ambiguous rune (incl.
the ellipsis `…`) at a different width than glyph's `RuneWidth` reports will see the same
off-by-one it would hand-rolling the loop — `TruncateWidth` neither introduces nor cures
that. The cure for that disagreement is width-POLICY agreement between glyph and the
consumer, which is a separate concern (an ambiguous-width policy knob, tracked elsewhere).

## Risks

- Wide-rune-at-boundary semantics (exclude vs split) must be defined and tested; excluding
  is the only correct choice (a split wide rune is the bug we are preventing).
- Ambiguous-width runes inherit whatever `RuneWidth` reports; this primitive does not
  change width policy, it only applies it consistently. (Any ambiguous-width terminal
  policy question is separate and out of scope here.)

## Testing

- ASCII: truncates by count == columns (the easy case still correct).
- wide runes (CJK/emoji): a string of N width-2 runes truncates to `maxCols/2` runes, and
  the result's `StringWidth` is `<= maxCols` (the regression the rune-count version fails).
- wide rune straddling the boundary is excluded, not split.
- ellipsis variant: fits-unchanged when it fits; ends with "…" within budget when it
  doesn't; boundary `maxCols` of 0 and 1.

## Migration

Purely additive. Consumers adopt it by replacing hand-rolled clip loops (and any
rune/byte-count truncations) with `TruncateWidth` / `TruncateWidthEllipsis`.
