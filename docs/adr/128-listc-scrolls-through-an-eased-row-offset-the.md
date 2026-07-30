# ADR 128: ListC scrolls through an eased row offset: the discussion pane stops teleporting

- status: accepted
- date: 2026-07-30 14:47:26

## Context and Problem Statement

List scrolling teleports. The list computes a first-visible item index and paint snaps to it, so every scroll movement lands the eye on a new frame with no motion between. The discussion pane in the review tool is a List, and its owner names scrolling there as the product's poorest interaction: disorienting on every scroll, without fail.

The contrast is measured, not guessed. ScrollView owns a continuous row offset, and `ScrollOffset(Animate(...))` eases it. The chat lanes moved to that seam in one line and the same motion reads well. The discussion pane cannot follow because ListC exposes no offset to ease: `ext.listPtr.offset` is an item index, and the row figures ScrollState derives exist only for scrollbars.

Consumers today, all in the review tool: the discussion pane, the inbox list, the kanban columns, the explorer pager. Every one scrolls by snap.

## Considered Options

1. Leave Lists snapping; ease only ScrollView surfaces.
2. Render Lists inside a ScrollView to borrow its offset. Rejected: the list culls at the data layer so only visible view-models build; a ScrollView cannot know that and the culling contract breaks.
3. A row-offset presentation stage inside ListC: the window computation stays exactly as it is, and paint reads a continuous row offset that an Animate binding eases toward the window's position. The cull covers the tween's in-flight frames. Opt-in, default unchanged.

## Decision Outcome

Chosen option: 3. ListC gains an opt-in eased offset, mirroring ScrollView's seam.

The concrete surface:

* `List(...).ScrollEase(Animate(0))` (name at the glyph seat's discretion) makes paint go through a continuous row offset. The target of that offset is the same row arithmetic the ScrollState writeback already computes from measured item heights. Nothing about window selection, selection state, or data-layer culling changes.
* The cull stays BOUNDED mid-tween: each in-flight frame culls from the CURRENT eased offset (the usual window's worth of items), never the union of the whole traversal — a long jump must not build hundreds of view-models in one frame. Beyond a distance threshold (a screenful or two) the ease SNAPS: easing hundreds of rows is a blur that costs the widened build for nothing.
* The scrollbar follows the EASED position, not the target index: the bar presents what is on screen, and a bar resting at the destination while rows still glide would disagree with the visible window every in-flight frame. It quantises to whole rows by construction (the offset pointer is an integer), so the bar's motion is row-stepped rather than sub-row — expected behaviour, not a bug.
* The one-shot alignment request already under deliberation on the framework side composes: an aligned selection glides to its position instead of appearing there.
* Default remains the current snap. Every existing consumer renders identically until it opts in.

Because:

* the mechanism is proven on this codebase: the chat lanes ease today through exactly this shape of seam, and the missing piece in ListC is representation, not animation,
* option 2 trades the culling contract for an offset; option 3 adds the offset without touching the contract.

## Verification bar

* `ScrollEase` is a new pointer-binding surface, so glyph's binding day-one rule applies on the day it lands: `isWithinRange` treatment at compile, plus a ForEach test proving two items render their own values — without it a per-item ease pointer freezes on item 0.
* Zero-alloc is measured on TWEEN frames specifically, not resting ones: the in-flight frames are where this feature does its work, and a resting frame staying allocation-free proves nothing about them.

## Agreed todos

- [ ] ListC paints through a continuous row offset that an Animate binding eases, opt-in, culled per-frame from the eased position and snapping past a distance threshold.
- [ ] An aligned selection composes with the eased offset: the alignment request glides instead of teleporting.

## End-goal state

The Layer-arming half moved to its own proposal at the reader's request (it is cheap and independent); this deliberation keeps only the List seam. The review tool's discussion pane opts in and scrolling it reads as motion: keys, half-page jumps, and alignment requests glide to their target and settle, with the same rows visible at rest as the snap would have shown, the scrollbar tracking the eased position throughout. Consumers that do not opt in render byte-identical frames to today. Tween frames allocate nothing and build only the current window's items.

## Non-goals

* Changing the window or culling model. The offset is presentation only.
* Easing by default. Every surface opts in deliberately.
* The alignment request itself, which has its own deliberation; this composes with it.
