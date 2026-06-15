# ADR 12: Render partial rows at the viewport edge (variable-height Lists)

- status: accepted
- date: 2026-06-15 21:09:49
- proposer: Kestrel@recap
- parties: recap, tui
- deliberation: recap proposal show 12 (13 comments)

# Render partial rows at the viewport edge (variable-height Lists)

## Problem

A List of variable-height rows drops the bottom row ENTIRELY when its measured
height doesn't fully fit the remaining viewport rows (renderSelectionList, the
clipMaxY trim ~template.go:8667: `trimEnd = ci; break`). At rest, a row partly
off-screen vanishes with no signal — it reads as the end of the content.

In recap's comments pane this caused a recurring misread: a multi-line comment at
the bottom edge disappeared, so the reviewer repeatedly believed there were no
replies. The row is reachable by navigating to it (the List scrolls it into view
on select), so it is not data loss — it is a missing affordance.

The fix belongs IN the List, not in a consumer workaround (the human's pc91): a
batteries-included List should show the partial edge row itself, not force
consumers to migrate to a Layer and hand-roll selection just to get it.

## Proposed change

At the viewport edge, render the last row PARTIALLY — clipped at the viewport
bottom — instead of dropping it whole. The top of the edge row peeks in, the
universal "there's more below" affordance, no separate indicator needed.

## Fix shape (converged with the owner)

The real work is that `clipMaxY` is not enforced at the WRITE level: only wrapped
text honours it (via clipLines); plain Text rows in a full-pipeline item and the
selection/default FillRect (template.go:8773, fills itemH unconditionally) write
straight through it. So including the partial row requires actually CLIPPING its
overflow, not merely including it.

Cleanest mechanism (Glyph Smith): a buffer-level vertical clip. Buffer has only
InBounds (full-buffer) today; add a clip-max-Y that the write primitives
(Set/SetFast/FillRect/WriteString*) respect, set + restore around the partial
row's draw in renderSelectionList. One predictable compare per write, and it
makes clipMaxY mean ONE thing everywhere instead of being honoured ad hoc in
three places — a clean general primitive, not a special case.

Scope: `trimEnd = ci + 1` (include the edge row); FillRect height clamp; the
buffer vertical clip around the partial row's draw. Also: the visRows scroll
writeback (template.go:8719) counts the partial row's VISIBLE portion
(availableRows - rowsUsed), not its full height, so a ScrollbarDyn reads right at
the boundary.

## Default policy (Pete's verdict)

Converged recommendation: DEFAULT-ON. The at-rest change is a partial row peeking
in where a whole row used to vanish — the correct read, not a regression — which
matches the batteries-included framing (the List just does the right thing).
Glyph Smith initially leaned opt-in (it IS an at-rest clipping change) but moved
to default-on on the same framing. An opt-in `List.PartialEdge()` flag remains
the fallback if Pete prefers existing Lists stay byte-identical. The verdict sets
which.

## Alternatives considered

- Consumer-side overflow cue ("▼ more below"): tried in recap and rejected — it
  occupied a row the pane focus line rendered over, and it's a band-aid for a
  rendering choice that belongs to the List.
- Migrate the consumer to a Layer (blit shows partial rows for free): rejected by
  the human (pc91) — forcing a Layer + hand-rolled selection defeats the point of
  a batteries-included List.

## Notes

Classified semantics-needs-a-proposal by the owner (m418/m429); fix shape +
default-on endorsed (m447). No implementation written ahead of a verdict;
Glyph Smith builds the moment the ADR + todo materialise. recap's comments pane is
a ready repro. General win: every variable-height List (chat, feed, comments).
