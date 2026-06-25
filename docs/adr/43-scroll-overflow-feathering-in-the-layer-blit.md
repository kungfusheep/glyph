# ADR 43: Scroll-overflow feathering in the layer blit

- status: accepted
- date: 2026-06-25 18:12:59
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 43 (0 comments)

# Scroll-overflow feathering in the layer blit

## Context and Problem Statement

A scroll region gives no visual signal that content continues past its edge. When
a chat/log/diff view is scrolled, the top and bottom rows simply cut off at the
viewport boundary, so the reader can't tell at a glance whether there is more
above or below — the content looks like it might just end there.

A scrollbar partially addresses this, but it is a separate gutter element that
competes for width and reads as a control, not as "the content continues." The
wanted affordance is lighter: a few rows of fade at an edge ONLY when there is
more content in that direction — so the fade itself encodes scroll state.

Precise behaviour:
- top edge fades only when scrolled down from the top (`scrollY > 0`).
- bottom edge fades only when not yet at the end (`scrollY < maxScroll`).
- at rest with content fitting the viewport (`maxScroll == 0`), no fade at all.

## Considered Options

1. **Status quo** — hard cut at the viewport edge; no overflow affordance.
2. **Scrollbar only** — already available, but it is a gutter control, not an
   in-content "more below" cue, and it costs a column.
3. **Edge feathering in the blit** — fade the first/last N rows of the blitted
   region toward the layer background, gated on scroll position so the fade only
   appears where content actually overflows.

## Decision Outcome

Chosen option: **3 — edge feathering in `Layer.blit`**, opt-in via a depth config.

```go
ScrollView.Grow(1).Feather(2)(chatRows...)   // 2-row fade at each overflowing edge
```

`Feather(n)` sets the fade depth in rows (0 = off, the default). The fade is
applied in `Layer.blit` after the visible region is copied: the top `n` rows are
ramped toward the background when `scrollY > 0`, and the bottom `n` rows when
`scrollY < maxScroll`. The ramp is per-row — row nearest the edge fades most,
fading out to no change `n` rows in.

Because:
- the fade encodes scroll state directly (it is present exactly when there is more
  to see in that direction), so it is self-documenting and costs no layout space.
- it reuses the existing colour-interpolation machinery (`Lerp`/`lerpIfRGB`)
  rather than a bespoke blend, so RGB colours fade smoothly and non-RGB
  (named/256) colours degrade gracefully to no fade — a documented limitation, not
  a crash.
- it is opt-in and additive: `Feather(0)` (default) is byte-for-byte today's blit.

## Technical

- **Where.** In `Layer.blit` (`layer.go:264-273`), after `dst.Blit(...)`. The blit
  already reads `scrollY` under `scrollMu`; it also needs `maxScroll` (same lock)
  to gate the bottom edge. No new concurrency surface — both are existing
  lock-guarded fields read on the frame path.
- **Fade.** For each faded row `r` (0..n-1 from the edge), compute
  `t = (n - r) / (n + 1)` and lerp each cell's foreground (and background, if set)
  toward the layer's `defaultStyle` background using the existing `lerpIfRGB`. The
  edge row gets the strongest `t`; the fade vanishes by row `n`.
- **Gating.** Top rows fade iff `scrollY > 0`; bottom `n` rows fade iff
  `scrollY < maxScroll`. Both can be active at once (mid-scroll); a short viewport
  where the two regions would overlap clamps so a row is faded by the stronger of
  the two.
- **Colour-mode behaviour.** `lerpIfRGB` only interpolates true-colour cells;
  named/ANSI-256 cells are left unchanged. This is the honest degradation — the
  feature documents that feathering needs RGB to render, and is a no-op otherwise
  rather than mis-approximating.
- **Reuse, don't re-derive.** `Lerp(a, b Color, t)` and `lerpIfRGB(c, target, t)`
  already exist; the feather is a thin per-edge loop over them.

## Performance

`Layer.blit` is the per-frame hot path (unlike the layer's `render`, which is
cached and only re-runs on width change / invalidate). The feather adds work
ONLY when active and bounded to `2 * n * width` cell lerps per frame (n is small,
typically 1-2). It must stay alloc-free — the lerp is integer/float math over
existing cells, no allocation. A benchmark is REQUIRED and ships with the change:
`BenchmarkLayerBlit` (feather off, must match baseline) vs
`BenchmarkLayerBlitFeather` (on), proving the off-path is unchanged and the
on-path cost is the bounded edge work, not a per-frame regression across the whole
buffer.

## Interactions

- **Scrollbar.** Independent — feathering fades content rows; the scrollbar gutter
  is unaffected. They compose (a view can have both).
- **Bottom-anchor (the underflow vertical-align option).** No conflict: when
  content underflows, `maxScroll == 0`, so neither edge feathers — anchoring short
  content to the bottom never shows a fade.
- **The scroll-offset binding rework.** That work also touches `Layer.blit`. This
  change edits the same function; whichever lands first, the other rebases so blit
  is not modified in two in-flight branches simultaneously. The feather reads the
  resolved `scrollY`/`maxScroll` regardless of whether the offset is imperative or
  bound, so it composes with either model.

## Risks

- Per-frame cost in blit — mitigated by the off-path being unchanged and the
  on-path bounded to edge rows; gated by the required benchmark.
- Colour-mode degradation (non-RGB no-fade) — documented, not silent.
- Cursor row falling inside a faded edge — the cursor is drawn separately by the
  framework after blit, so it is not dimmed; verify in a test.

## Testing

- top feathers only when `scrollY > 0`; absent at the very top.
- bottom feathers only when `scrollY < maxScroll`; absent when scrolled to end.
- content fits viewport (`maxScroll == 0`): no feather at either edge.
- `Feather(0)` / default: blit output identical to today (regression guard).
- depth respected: `Feather(n)` fades exactly `n` rows with a monotonic ramp.
- short viewport where top and bottom regions overlap: rows clamp, no double-dim
  past full.

## Migration

Additive and opt-in. `Feather(0)` is the default and leaves every existing view
unchanged. A view adopts the affordance with a single `.Feather(n)`.
