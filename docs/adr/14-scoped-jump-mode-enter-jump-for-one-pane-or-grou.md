# ADR 14: Scoped jump mode: enter jump for one pane or group, not the whole screen

- status: accepted
- date: 2026-06-15 21:12:56
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 14 (4 comments)

# Scoped jump mode: enter jump for a region (NodeRef), not the whole screen

## Problem

`EnterJumpMode` collects EVERY visible jump target across the whole template and
labels them all. In a multi-pane app (the classic three panes, one focused at a
time) entering jump mode floods every pane with labels, even though the user
almost always wants to jump within the pane they are working in.

The only scoping today is spatial and implicit: a jump inside a Layer/scrollview
is bounded by `jumpMinY/jumpMaxY` (template.go:659). That is an internal viewport
clip, not a user-chosen region.

Consequences of labelling everything: visual noise across panes; more
simultaneous targets crosses 27 sooner, so labels go two-char earlier than
needed (the density ADR 11's multi-char feedback exists to soften — scoping
attacks it from the other side by keeping counts low); and ambiguity about which
pane a label belongs to.

## Goal

Let the developer enter jump mode scoped to one or more screen regions, so only
targets inside those regions are labelled and selectable. Whole-screen jump stays
the default and is unchanged.

Pete's framing (verbatim): "enter jump mode just for that pane in the UI."

## Design: NodeRef regions as positive hit-tests (pc103)

A pane already knows its own rectangle, and glyph already captures it.
`VBox.NodeRef(*NodeRef)` / `HBox.NodeRef(*NodeRef)` (components.go:455, 833)
populate a `NodeRef` with the container's rendered screen rect every frame — the
same mechanism recap's diff pane uses via `ViewRef`. So the scope is just: pass
those rects when entering jump, and a target counts only if it falls inside one.

```go
// the pane already hands you its rect — no new capture machinery
VBox.NodeRef(&leftPane)( /* a pane full of Jump/JumpItem targets */ )

// enter jump scoped to one or more regions
app.EnterJumpScope(&leftPane)
app.EnterJumpScope(&leftPane, &rightPane)   // union: inside ANY rect
app.JumpScopeKey("g", &leftPane)            // keybound scoped entry

// unscoped entry is exactly as today (every target)
app.EnterJumpMode()                         // unchanged
app.JumpKey("g")                            // unchanged
```

No per-target tags, no wrapper component, no group hierarchy threaded through
sub-templates. The pane's NodeRef IS the scope.

## Implementation sketch

- `JumpMode` gains a scope-rects slice (set by `EnterJumpScope`, cleared on
  exit). Empty == today's whole-screen behaviour.
- The registration filter is the whole runtime change: in `renderJump`
  (template.go:9012, before `AddJumpTarget`) and in the span-jump path
  (`richSpanJumpFunc`, template.go:9044), skip a target whose (x,y) is not inside
  any scope rect. Targets register in screen space and NodeRefs ARE screen rects,
  so it is a plain point-in-rect test — composes with the existing layer viewport
  clip, no coordinate translation, no new traversal.
- `EnterJumpScope(rects ...*NodeRef)` sets the scope then runs the existing
  `EnterJumpMode` flow unchanged (render collects only in-region targets,
  `AssignLabels` labels only those, the router is built only for those).
- `JumpScopeKey(pattern, rects ...*NodeRef)` is the keybound convenience.

This also covers the span path (`richSpanJumpFunc`), which the group-tag design
could not without a new Span field — point-in-rect needs nothing from the target.

## Why NodeRef over group tags

An earlier draft used a string group tag per target plus a `JumpGroup` wrapper.
NodeRef regions are simpler and reuse machinery that already exists:

- No tagging every target; targets stay vanilla `Jump`/`JumpItem`.
- No wrapper component and no group-context propagation through sub-templates.
- Reuses `VBox/HBox.NodeRef` (already shipped) and the `ViewRef` pattern recap
  already relies on.
- Works for span jumps for free.

Trade-off, stated honestly: NodeRef scoping is spatial — a rectangle, or several.
It nails the pane/region case (the actual driver) and accepts a union of regions.
What it does NOT express is a non-contiguous LOGICAL group (e.g. scattered "all
delete buttons") without one rect each. That is judged YAGNI: the real need is
panes/regions; if an arbitrary logical grouping ever surfaces, `JumpItemRef`
already captures per-item rects as the escape hatch, and a tag could be added
then. Start concrete; do not build the general grouping layer speculatively.

## Open questions for the verdict

1. Naming: `EnterJumpScope` / `JumpScopeKey` (vs partial / modal — recommend
   "scope"; glyph has no modes, so "modal" cuts against the brand).
2. One-frame freshness: a pane's NodeRef is populated during render and the scope
   filter reads it during the same collecting render; for a static pane the rect
   is stable frame-to-frame so it reads correct even a frame behind. Acceptable?
   (Recommend yes — panes don't move mid-keystroke.)
3. Empty/zero scope rect (pane not yet rendered): treat as "no targets in scope
   -> exit", matching `EnterJumpMode`'s existing no-targets behaviour. Agree?

## Edge cases (pc105)

- **Empty / never-populated rect — the one explicit guard.** A NodeRef that did
  not render this frame (a pane behind a false `If`, or not yet drawn) is the zero
  value `{0,0,0,0}`. The point-in-rect must treat any rect with `W<=0 || H<=0` as
  matching NOTHING, using half-open bounds (`x >= X && x < X+W`, the convention
  `renderJump` already uses). Otherwise an inclusive test would let `{0,0,0,0}`
  capture a target at the origin. With the guard, scoping to an unrendered pane
  yields zero in-scope targets and falls through to the existing
  no-targets-exit.
- **Off-screen / partially off-screen rects — no special handling.** The filter
  runs in screen space, and `renderJump` already drops targets at `absY<0 /
  >=Height / absX<0 / >=Width` before `AddJumpTarget` (template.go:9013). So a
  fully off-screen scope rect (a scrolled-away pane) contains zero registered
  targets and exits cleanly; a partially off-screen rect (e.g. `Y=-5, H=20`)
  includes only its visible portion via the same point-in-rect. Negative and
  overflow coordinates just work.
- **Stale-by-one-frame is self-correcting.** If a pane scrolled away this frame
  but its NodeRef still holds last frame's on-screen rect, the rect matches
  nothing anyway — the pane's targets are now off-screen and unregistered.

## Risk / ratification

Purely additive: new entry methods + a scope slice; `NodeRef` capture already
exists. Absent, behaviour is identical to today (no scope rects == whole-screen
jump). No observable change to existing apps, so no ratification angle.

No implementation written ahead of a verdict.
