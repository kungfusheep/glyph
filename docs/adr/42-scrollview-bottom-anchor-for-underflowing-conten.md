# ADR 42: ScrollView bottom-anchor for underflowing content

- status: accepted
- date: 2026-06-25 17:27:08
- proposer: Glyph Smith@tui
- parties: tui, recap
- deliberation: recap proposal show 42 (3 comments)

# ScrollView bottom-anchor for underflowing content

## Context and Problem Statement

`ScrollView` always top-anchors its content. The render path measures children
top-down, then trims the layer buffer to `max(contentHeight, viewportHeight)`.
When content is shorter than the viewport, the content occupies the top rows and
the leftover space falls *below* it.

This is wrong for the most common scroll-region shape there is: a chat / message
log with a composer pinned beneath it. A `ScrollView.Grow(1)` lane with only a
few messages leaves a large void between the last message and the composer,
because the messages stick to the top of the grown lane. `ScrollTo(maxScroll)`
cannot fix it — `maxScroll` is 0 when content fits, so there is nothing to scroll
to. There is no vertical-alignment option on `ScrollViewC` or on the underlying
`Layer`.

The desired behaviour: when content is shorter than the viewport, it should hug
the **bottom** (newest line just above the composer) with the empty space at the
**top**; once content overflows the viewport, normal top-to-bottom scrolling
resumes unchanged.

## Considered Options

1. **Consumer-side leading spacer** — wrap children in `VBox(Spacer.Grow(1),
   children...)`. Fails: `ScrollView` measures content into a buffer taller than
   the viewport (it grows to fit all content before trimming), so a flex spacer
   expands to fill the *measurement* buffer, not the viewport, and the measured
   content height becomes meaningless. The grow-to-measure strategy and flex
   spacers do not compose.

2. **Reactive `StickBottom(*bool)`** — a per-frame pointer-bound toggle. Overkill:
   a chat pane always wants bottom-anchoring; it is configuration, not state that
   changes frame to frame. Adds a pointer-binding surface (and its day-one
   regression-test obligation) for a value that never varies.

3. **Static `AnchorBottom()` builder flag** — a configure-then-call flag on
   `ScrollViewFn`, matching the existing `.Scrollbar()` / `.Border()` shape.
   Concrete, intent-revealing, no per-frame state. Affects only the underflow
   case; overflow scrolling is untouched.

## Decision Outcome

Chosen option: **3 — `ScrollView.AnchorBottom()`**.

```go
ScrollView.Grow(1).AnchorBottom()(
    ForEach(&messages, func(m *Message) Component { return Text(&m.Text) }),
)
```

Because:

- it matches glyph's component builder idiom (a static decoration method on
  `ScrollViewFn`, like `Scrollbar`/`Border`/`Title`).
- it stays concrete — no alignment enum until a top/center/bottom second use case
  actually exists. (If one does, `AnchorBottom()` is trivially re-expressible as
  `VAlign(AlignBottom)` later without breaking callers — easy to change, not a
  one-way door.)
- no new pointer-binding surface, so no per-frame state and no `isWithinRange`
  binding obligation.

Scope is deliberately narrow: `AnchorBottom()` changes **only** the
content-shorter-than-viewport case. When content overflows, behaviour is
byte-for-byte identical to today, including tail-follow via
`ScrollTo(maxScroll)`. This is underflow vertical alignment, not tail-following.

## Technical

Add to `ScrollViewC`:

```go
anchorBottom bool
```

and the builder method:

```go
func (f ScrollViewFn) AnchorBottom() ScrollViewFn {
    return func(children ...Component) *ScrollViewC {
        sv := f(children...)
        sv.anchorBottom = true
        return sv
    }
}
```

In `render()`, the content is measured into `buf` (height `h >= viewport`), with
the real content at rows `0..rawContentH-1`. The change applies at trim time:

- Today: when `rawContentH < viewportHeight`, the buffer is resized to
  `viewportHeight` and the content stays in the top rows.
- With `anchorBottom` set and `rawContentH < viewportHeight`: build the trimmed
  `viewportHeight`-tall buffer and blit the measured content rows down by
  `viewportHeight - rawContentH`, so they land in the bottom rows. The slack ends
  up at the top.

When `rawContentH >= viewportHeight` the flag is a no-op — the existing trim and
scroll path runs unchanged.

`render()` already allocates a buffer per invocation, and it runs only on width
change / `Invalidate()` — not per frame (the per-frame path is the layer blit,
which is untouched). So the extra bottom-positioning costs nothing in the hot
path; the perf-is-a-feature constraint holds.

## Risks

- **Interaction with `ScrollTo`/pending scroll.** Bottom-anchoring only triggers
  when content underflows (`maxScroll == 0`), where `ScrollTo` is already a no-op,
  so the two cannot fight. Covered by tests below.
- **Scrollbar gutter.** Bottom-anchor changes vertical content position only;
  width/gutter reservation is unaffected.

## Testing

- underflow + `AnchorBottom`: 2-3 lines in a tall viewport render in the BOTTOM
  rows, slack at top.
- underflow without the flag: unchanged (content at top) — regression guard.
- overflow + `AnchorBottom`: content taller than viewport scrolls normally,
  `ScrollTo(maxScroll)` still reaches the last line (flag is a no-op when tall).
- exact-fit boundary: `contentH == viewportHeight` positions identically with and
  without the flag.
- boundary handoff: assert the `AnchorBottom`/`ScrollTo` seam directly — one row
  below the viewport, `ScrollTo(maxN)` is a no-op and `AnchorBottom` positions;
  at/above the viewport, `AnchorBottom` no-ops and `ScrollTo(maxN)` tail-follows
  to the last line. This is the exact seam both consumer panes ride (they tail-
  follow with `ScrollTo` after every new message), so it gets its own assertion.

## Migration

Additive. New opt-in builder method; default behaviour (top-anchor) is unchanged
for every existing caller.
