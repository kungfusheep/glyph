# ADR 18: Fixed-Height containers should contain their content

- status: accepted
- date: 2026-06-17 14:32:30
- proposer: Glyph Smith@tui
- parties: recap, tui
- deliberation: recap proposal show 18 (9 comments)

# Fixed-Height containers should contain their content

## Problem

A container given an explicit `Height(n)` neither clips its children to that
height nor contains them within its box. Children lay out at their natural
cumulative Y, the container reports height `n` to its parent, and any overflow
renders **outside the declared box, colliding with following siblings**.

Probe — `VBox.Height(2).Border(BorderSingle)` holding 5 `Text` lines, with a
sibling `Text("AFTER")` after it:

```
row 0: ┌───────────
row 1: └L1─────────     <- bottom border merged onto content (height 2 box)
row 2: AFTER            <- sibling placed at declared height 2 (correct)
row 3:  L3              <- overflow: children escape the box...
row 4:  L4
row 5:  L5              <- ...and render over the siblings' region
```

So the parent lays the *sibling* out using the declared height (good), but the
*children* are drawn unclipped at their natural positions (bad) — the two
disagree and content bleeds. Without a border the same happens: `AFTER` lands on
row 2 where L3 wanted to be, and L4/L5 spill to rows 3-4.

This is the clipping line, so it comes as a proposal rather than a direct fix —
but note the current output is also arguably spec-violating (content leaves its
container's box), so the "do nothing" option isn't really on the table.

## Consumers

- recap comment open/close and log views (Kestrel, recap m607): a fixed-height
  row whose content can exceed the budget. Today the consumer pre-measures and
  trims in Go because the container won't bound its own content.
- any fixed-height panel/pane in a dashboard (a log tail, a detail pane) that
  should show "the first N rows and no more" without the rest leaking downward.

## Proposed change (behaviour — owner's call on spelling)

When a container has an explicit `Height(n)` (or HeightPct), clip its children's
rendering to the inner content box (height minus border/padding/margin). Content
beyond the box is not drawn; the box owns exactly its declared rows. The existing
clip machinery (`clipMaxY`, `PushClip`/`PopClip`) already does row/rect clipping
for other paths — this wires the declared-height box into it during child render.

Open questions for the thread:
- clip silently, or expose an explicit `.Clip()`/`.Overflow(hidden|visible)` so
  the default stays today's behaviour and clip is opt-in? (I lean: fixed Height
  should clip by default — an explicit height that doesn't bound is surprising.)
- scroll is a separate, larger capability (offset + viewport); out of scope here.
  This proposal is clip-only. A `Scroll()` modifier can follow.

## Implementation sketch

- During `layoutContainer`/child render for a container with a resolved explicit
  height, set the render clip rect to the inner content box and restore after.
- Reuse `PushClip`/`PopClip` (buffer.go) — no new clip primitive.
- Cost falls only on fixed-height containers; flexible containers unchanged.
- Benchmark: prove no per-frame cost added to the common (no fixed height) path,
  and the clip path stays allocation-free.

## Risks

- Content that currently shows (by overflowing) will stop showing — that's the
  point, but it's an observable change for anyone who relied on the leak. Mitigate
  with the opt-in question above if we want a softer landing.
- Border + height interaction (the merged-border row in the probe) needs an
  explicit test: a bordered fixed-height box must reserve both border rows and
  clip content between them.

## Binding note

No new pointer-binding surface (height already binds via the existing dyn-height
path). If a dynamic `Height` pointer drives the clip, it rides the existing
height binding — a ForEach regression test (two items, different heights, each
clips to its own) lands with the change per the day-one rule.
