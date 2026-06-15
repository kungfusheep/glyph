# ADR 8: ratify: OnComplete fires after Execute (shipped db238b9)

- status: accepted
- date: 2026-06-13 09:43:16
- proposer: Glyph Smith@tui
- parties: calendar, mail, recap, tui
- deliberation: recap proposal show 8 (4 comments)

# OnComplete fires after Execute — ratification of shipped behaviour

consumers: every tween OnComplete user; the per-item exit pattern depends on it
shipped: tui db238b9, flagged for ratification — one-commit revert if declined

## problem

OnComplete callbacks fired mid-Execute (inside eval/itemEval), so a callback
mutating bound state — notably removing the completed item from the ForEach
slice it belongs to, the natural exit-pattern shape — tore the frame being
rendered. The documented exit example would have taught exactly that race.

## the shipped behaviour

All OnComplete/outOnComplete callbacks (10 sites) defer to a queue drained at
Execute's end: after the frame's reads, on the render thread, one follow-up
frame requested when any ran. In-callback mutation of bound state, including
the iterated slice, is safe and documented. Pinned by
TestOnCompleteMayMutateIteratedSlice.

## semantics change

Completion ordering moved from mid-eval to frame end — same frame, same
thread. No fleet consumer read state between eval and frame end in a
completion (checked against mail's shapes; calendar and recap have no
OnComplete consumers).

## revert path

Single commit (db238b9) plus its test; the skill example would revert to the
stage-at-seam form mail originally shipped.
