# ADR 70: per-rune style resolver on the editable Input (gates @-mention type-time highlight)

- status: accepted
- date: 2026-06-30 15:16:45
- proposer: Glyph Smith@tui
- parties: tui, token
- deliberation: recap proposal show 70 (8 comments)

# Proposal: per-rune style resolver on the editable Input

## Problem

A glyph `Input` renders its whole buffer in one uniform style. `renderTextInput`
applies a single `textStyle` to every rune, with the only per-cell deviation being
`cursorStyle` at the caret. So there is no way to style a *sub-range* of the live,
editable text — colour an `@mention` as you type it, highlight a search match in a
filter field, tint a syntax token in a small code input. The capability simply
doesn't exist on the editable path today (it does on static `RichText`, but not on a
buffer the user is editing).

This blocks "@-mention highlight while typing" in a chat/comment composer, and more
generally any live-styled editor field.

## Who this is for

- **A chat/comment composer** — a completed `@Name` lights up as it's typed.
- **Any consumer of a styled editable field** — search-match highlight, inline token
  colouring, validation-state colouring of a specific span. This is a general gap in
  the Input, not a single app's feature.

## API shape — styled ranges (compute-on-change, not per-frame)

A bound list of styled ranges over the editable value, opt-in and zero-cost when unset:

```go
Input(&text).StyleRanges(&ranges)   // ranges []StyleRange, rebuilt on change

type StyleRange struct {
    Start, End int   // rune indices into the value, half-open [Start, End)
    Style      Style
}
```

- `StyleRanges(rs *[]StyleRange) *InputC` — binds a pointer to the consumer's range
  slice. At render, `renderTextInput` walks the (sorted) ranges alongside the runes and
  applies the matching range's style; runes in no range keep the uniform `textStyle`.
- The consumer rebuilds the slice **on value-change**, in the Input's existing
  `OnChange` hook (`onChange func(string)`, already fired by `handleChange` — no new
  machinery). The ranges only change when the text does, so they're computed once per
  edit, not per frame.
- `nil`/empty (the default) keeps the exact current uniform-style path —
  **byte-identical, zero added cost** for every Input that doesn't opt in.
- The caret still wins at the cursor cell (cursor styling unchanged).

### Why ranges, not a per-rune callback

The first draft proposed `StyleFunc(func(i, r) Style)` called per rune. Pete flagged the
perf, correctly: a per-rune callback recomputes the styling decision every frame and
pays a non-inlinable call (params escape) per visible rune — yet the styled ranges only
change when the text changes. Ranges flip it to glyph's compute-once/execute-cheap
shape: recompute on the discrete edit event, then each frame is a cheap merge-walk of a
pre-built slice (O(N+ranges), no allocation, no call, no escape). Strictly less
per-frame work, and it matches the on-change-not-per-frame principle of pointer
reactivity everywhere else in glyph. A new lightweight `StyleRange` is needed because
the existing `Span` carries `Text` — wrong fit for indexing a buffer the user edits.

## Implementation sketch

- Add `StyleRange` type and a `styleRanges *[]StyleRange` field on `opTextInput`
  (+ the `InputC.StyleRanges` builder).
- In `renderTextInput`'s per-rune loop (single-line and multiline branches): keep a
  cursor into the sorted ranges; for rune `i`, advance past ranges ending at/before `i`,
  and if `i` falls in the current range use its `Style`, else `textStyle` — then the
  existing cursor-override. When `styleRanges` is nil, the loop is exactly as today.
- ~20 lines + the builder + the type. No change to layout, scrolling, masking, cursor.

## Performance

**Guarantee: existing text input is unaffected.** An `Input` that does not set
`StyleRanges` — which is every Input today, none opt in — pays *exactly nothing*. The
nil case takes the current `renderTextInput` code path unchanged, gated by a single
nil-check: no per-rune branch, no range walk, no allocation, byte-identical output and
the same ns as today. This is enforced, not asserted: a benchmark comparing the nil-path
against the pre-change path ships with the implementation (same ns, 0 allocs), the same
off-path discipline used for feather (feather==0 = plain `Blit`, proven identical) and
`ActiveBindings`. If the off-path isn't byte-identical and same-cost, it doesn't ship.

- Off-path (no `StyleRanges`): unchanged, one nil-check, zero cost — per the guarantee
  above (the feather-off-path discipline).
- On-path: a merge-walk of runes + ranges per frame — O(visible runes + ranges), no
  allocation, no per-rune call. The consumer's recompute runs only on change (a
  keystroke parse of a short line), off the per-frame path. Honours zero-alloc-per-frame.

### Incremental span maintenance (consumer-side now; an OnChange-delta enhancement later)

The per-frame render cost is O(N+ranges) regardless of how the ranges are maintained —
this section is purely about the consumer's *recompute on edit*, which the simplest
consumer does as a full re-parse but need not.

- **Permitted today, no glyph change.** The consumer owns the `[]StyleRange` slice and
  mutates the bound pointer, so it is free to update incrementally instead of rebuilding.
  On a pure append (the common typing case), every range before the edit is unchanged —
  only the tail near the cursor needs re-scanning (extend the last token or append one).
  That is O(edit), not O(text). The API does not force a full re-parse; "rebuild on
  change" is the simplest consumer, not the required one.
- **Future direction — an edit delta on OnChange.** To make incremental updates trivial,
  the consumer needs to know *what* changed, not just the new value. `OnChange` is
  `func(string)` today, so an incremental consumer must diff old vs new to find the edit
  point. A later enhancement could pass the edit delta (cursor position + inserted/deleted
  runes) so the consumer patches ranges with no diff. That widens the `OnChange` surface,
  so it is a separate follow-up — not this proposal — but it is the natural path to
  first-class incremental span editing, and is recorded here so the door stays open.

## Risks / notes

- **Range/value sync.** The consumer must rebuild ranges when the value changes, or a
  stale range could index past the current text. Mitigated: indices are clamped to the
  value length at render (an out-of-range entry is skipped, never a crash), and the
  `OnChange` recompute is the documented contract.
- **Cursor precedence.** The caret cell keeps `cursorStyle` regardless of ranges, so a
  styled range never hides the cursor. Stated and tested.
- **Day-one binding rule.** Ships with a test asserting a sub-range renders its range
  style while the rest stays `textStyle`, the on-change recompute path, the
  out-of-range-clamp safety, the cursor-still-wins case, and the nil-unchanged case.

## Not in scope

- No mention parsing, no `@`-awareness — that's the consumer's resolver. This is the
  primitive (per-rune style), nothing semantic.
- No styling of the placeholder or mask — only the real editable runes.
