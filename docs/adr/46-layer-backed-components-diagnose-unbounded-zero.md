# ADR 46: Layer-backed components: diagnose unbounded (zero) viewport height instead of blank

- status: accepted
- date: 2026-06-25 18:42:41
- proposer: Glyph Smith@tui
- parties: tui, recap-timeline, Glyph
- deliberation: recap proposal show 46 (8 comments)

# Layer-backed components render blank (silently) with an unbounded viewport height

## Context and Problem Statement

A consumer placed `List(&rows).Selection(...)` in a content-sized column (a VBox
column inside an HBox, with no bounded height) and it rendered NOTHING — silently. It
passed every count/selection assertion; only a render-paint test caught the blank
column. The consumer swapped to `ForEach` for the fixed-row column.

Root cause: `List` (and the other layer-backed components — `Log`, `TextView`,
`ScrollView`) render into an offscreen layer that is blitted using a viewport height
set during layout. When the component sits in a content-sized context with no height
source (no `Grow`, no `.Height`, no bounded parent), the viewport height resolves to
0, so the layer blits zero rows. The component is "there" (state, selection, counts
all correct) but paints nothing.

The harm is the SILENCE: a layer-backed component with no bounded height is almost
always a mistake (the developer wanted to see rows), but it looks like a working tree
until something paints the buffer and finds it empty.

## Considered Options

1. **Status quo + docs** — document that layer-backed components need a bounded height
   (Grow/Height/bounded parent). Cheapest; leaves the silent-blank trap.
2. **Self-size to content when unbounded** — when no viewport height is given, fall
   back to the component's content height (like a non-layer container). Removes the
   trap, but defeats the point of a layer (a 10k-row log would render all 10k rows
   into the layer every change) and changes scroll semantics — there is nothing to
   scroll if the viewport always equals content.
3. **Fail loud when a layer-backed component gets a 0 viewport height at render** —
   a clear panic/diagnostic ("List has no bounded height — give it Grow, .Height, or a
   bounded parent") the first time it would blit zero rows.

## Decision Outcome

Proposed: **Option 3** for the diagnostic, scoped carefully. Rationale:

- the silent-zero is the whole problem; a layer-backed component that would paint
  nothing because it has no height should say so, not vanish.
- option 2 (self-size) quietly defeats the layer's purpose and muddies scroll
  semantics — a large log self-sizing to content is a worse surprise than a clear
  error, and "performance is a feature" argues against rendering unbounded content.

Trigger (resolved): diagnose only when **content is present, the resolved viewport
height is 0, AND the component has no explicit height source** — i.e.
`rows > 0 && viewportHeight == 0 && !hasHeightSource`.

The first two conditions separate the mistake from every content-EMPTY transient (first
frame before layout, an empty If branch, a drill pane that is legitimately empty until
something is selected) — those are 0 rows AND 0 height, nothing to show, so they stay
silent.

The third condition — no explicit height source — handles INTENTIONAL zero height. A
height that is animated, bound (`*int`), or set explicitly (including a deliberate
`Height(0)`) is intentional by definition; we never warn on it. So an animated
`Height` collapse (tweening to 0 and back), a deliberately collapsed accordion section,
and a mount-collapsed expander are all exempt by construction — they HAVE a height
source. The warning is reserved for the one case where the layout never sized the
component at all: content present, resolves to 0, and no `Grow` / `.Height` / `*int` /
height tween was ever given — which is exactly the reported bug (a `List` with no `Grow`
in a content-sized column). This is a static property of how the component was
configured; no animation-state tracking is needed.

Severity: a **single `os.Stderr` line, fired at most once per component instance,
behaviourally inert** — no panic, no change to layout or render. The component still
renders blank; the line only tells the developer why. This matches glyph's existing
diagnostic convention (screen.go and template.go already write debug detail to
`os.Stderr`). Nuance: in a live fullscreen TUI, stderr shares the terminal with the
alternate-screen render, so the warning surfaces cleanly when stderr is redirected to a
file (the standard dev setup, and how glyph's other stderr diagnostics are read); it
never corrupts the model or render, but an un-redirected fullscreen app would see it
bleed onto the screen. A glyph in-app diagnostic buffer (visible without redirect) is a
possible later follow-up; the one-time stderr line is the minimal first cut and is fully
non-behavioural. (A consumer hit this exactly: a 5-row List with 0 height in a
content-sized column rendered blank while three state assertions — len==5, the open-key
dispatch, the view-switch — all passed green; only a render-paint test caught it. Counts
prove the math, not that pixels landed. That is the silence this warning ends.)

## Technical

- A guard at the layer blit / size-resolution point for `List`/`Log`/`TextView`/
  `ScrollView`: when `rows > 0 && viewportHeight == 0 && !hasHeightSource`, emit a
  one-time logged warning naming the component and the fix (give it `Grow`, `.Height`,
  or a bounded parent).
- `hasHeightSource` is a static config flag set when the component is given any of
  `Grow`, `.Height`/`HeightPtr`, or a height tween/binding. It is the signal that a 0
  height is intentional (animated/collapsed/explicit) rather than the layout never
  having sized the component — so an animated `Height`→0 never trips the warning.
- One-time per component instance; must not add per-frame cost (the check is a cheap
  comparison, the warning fires at most once).

## Risks

- False positives on legitimate transient/hidden 0-height — addressed by the
  `rows > 0 && viewportHeight == 0` discriminator (benign cases are content-empty) plus
  the warning-not-panic severity, so an edge that slips the discriminator is non-fatal.

## Migration

Additive diagnostic; no change for components that already have a bounded height. The
fix for an affected consumer is the existing one — give the component `Grow`/`.Height`
or use `ForEach` for a fixed-count list.
