# ADR 89: frame cost: paint-only animation frames + memoised text measurement

- status: accepted
- date: 2026-07-07 16:34:04
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 89 (10 comments)

## Implementation amendments (as built)

- **Slice 1 verifies instead of classifying.** The sketch below labels tweens
  paint-only vs geometry at compile. As built, the gate is stronger and simpler: it
  OBSERVES the geometry inputs directly after phase 0 — dynamic width/height/flex/
  percent/gap values compared against a snapshot from the last full layout, If/Switch
  checked for branch flips or running exits, ForEach lengths compared to laid-out
  counts, recursively through active branches. Anything moved (or anything per-item
  animated, which advances inside the passes) takes the full frame. Same gate
  semantics, no per-tween plumbing, and it catches every geometry channel by
  construction.
- **Opacity compositing was made arena-backed** (per-frame reuse) so animated-opacity
  frames are steady-state zero-alloc — the snapshot-per-frame allocation would have
  broken the zero-alloc guarantee for exactly the frames this ADR targets.
- **Slice 2 exposed a per-item cache defect, fixed here:** the elemBase-keyed cache
  aged entries against the fixed cap, so any list with more live items than the cap
  falsely evicted every entry every round (evict→miss→recreate churn). Eviction now
  ages against the live round size (map length + a cap of grace); orphan sweeps are
  eventual (within bounded rounds) and also run periodically so a stable list still
  sheds realloc orphans. This defect affected every per-item state user on >cap
  lists, not just the width memo.
- **Measured outcome:** the 2000-row grapheme-heavy list frame fell from ~4.9ms to
  ~290µs full / ~210µs paint-only (0 allocs); the realistic big view from ~525µs to
  ~90µs. The 60%-of-a-core idle-oscillator scenario lands at ~1–2%.

# Proposal: frame cost — paint-only animation frames + memoised text measurement

## Problem

Two related costs make big views expensive to animate, measured (all zero-alloc):

- A typical view Executes in ~4.75µs (0.03% of a core at 60fps) and a heavy 3-pane
  view in ~525µs (~3–4% at 60fps) — fine. But a ForEach bound to a large backlog
  (2000 grapheme-heavy rows, ~60 visible) costs **~5.3ms/frame ≈ 30%+ of a core at
  60fps**, because layout measures **every row in the tree** each frame, dominated by
  `StringWidth`/grapheme work (matches the consumer profile from the live burn).
- Any active animation — including a one-cell opacity oscillator — forces those full
  frames at ~60fps, so a breathing dot over a big backlog burns a core doing string
  measurement for strings that didn't change.

Consumers bind full slices to List (inbox ~680 rows, explorer trees, kanban) and would
otherwise need hand-rolled windowing in every view. The engine can make both costs
disappear structurally.

## Design

**Slice 1 — paint-only animation frames.** Extend the existing effect-only skip gate
(cache-effects) to template animations: when a frame is driven *only* by animations
whose bound targets cannot affect geometry (Opacity, colour/style tweens, Osc on
those), skip width-distribution/layout/flex entirely and re-run only the render phase
over the cached geometry. Width/height/percent tweens still take full layout. The gate
classifies by what the active tweens bind — conservative: any geometry-capable tween,
any dirty bound data, resize, or input-driven frame ⇒ full Execute, exactly as today.

**Slice 2 — memoised text measurement.** Cache the measured cell-width of text ops:
on measure, compare the op's current string to the last-measured one (cheap: length +
pointer/prefix check, then bytes); unchanged ⇒ reuse the cached width, skipping the
grapheme walk. Per-op cache slots live beside the existing per-op state (ForEach
per-item cache included, with the established orphan-eviction pattern), so steady-state
frames over an unchanged backlog do no measurement at all. Changed strings pay one
re-measure — the cost moves from per-frame to per-edit.

## Performance targets (mechanically checkable)

- `BenchmarkUnboundedListFrame` (landed): steady-state frame drops from ~5.3ms to well
  under 1ms with memoisation; with the paint-only gate an opacity-only animation frame
  over the same view drops to the render-only cost.
- Consumer verification: the live app that hit the burn re-enables its breathing-dot
  oscillators and profiles at idle — **zero layout samples** on animation-only frames.
- Off-path discipline (the feather rule): views with no animation and changing text pay
  nothing new — `BenchmarkBigViewFrame`/`BenchmarkV2Execute*` must match pre-change ns.

## Agreed todos

- [ ] slice 1: animation classification — active tweens report geometry-affecting or
      paint-only; the frame gate (animation-only + all-paint + nothing dirty ⇒ skip
      layout, render over cached geometry), wired alongside the cache-effects gate
- [ ] slice 1: regression tests — opacity-osc frame skips layout (assert via a layout
      counter/hook), width-tween frame does not, dirty-data frame does not; bench
      proving the animation-only frame cost
- [ ] slice 2: per-op width memo — unchanged-string fast path in text measurement,
      incl. ForEach per-item slots with orphan eviction; changed-string invalidation
- [ ] slice 2: benches — BenchmarkUnboundedListFrame steady-state target + off-path
      parity for the existing Execute benches
- [ ] roadmap (after both land; needs the consumer's live profile session): consumer
      re-enables idle oscillators and verifies zero layout samples at idle via pprof

## End-goal state

An opacity/colour animation over any view — including a several-thousand-row bound
backlog — runs at 60fps for a render-only cost (no layout, no re-measurement of
unchanged strings), verified by the landed frame benches (unbounded list well under
1ms steady-state, big-view/typical benches byte-identical off-path) and a consumer
pprof showing zero layout samples on animation-only idle frames.

## Non-goals

- No API changes: no new public surface; both gates are internal engine behaviour.
- No windowing/virtualisation of ForEach in this proposal (memoisation makes the
  common case cheap; true virtualisation is a separate, bigger conversation).
- No change to when full layout runs for real changes — data edits, geometry tweens,
  resize, and input frames behave exactly as today.
- No animation cadence change (the 16ms ticker stays; a gentler idle tick can ride a
  later slice if still wanted once frames are cheap).
