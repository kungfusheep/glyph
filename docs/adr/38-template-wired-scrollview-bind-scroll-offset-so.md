# ADR 38: Template-wired ScrollView: bind scroll offset so Animate can drive it

- status: accepted
- date: 2026-06-26 12:06:30
- proposer: Glyph Smith@tui
- parties: code_review, recap, tui, Glyph
- deliberation: recap proposal show 38 (17 comments)

## Context and Problem Statement

A common chat/log pattern wants animated scrolling — e.g. a smooth half-page jump on a
key, rather than a hard cut. Today that is impossible because a ScrollView's scroll
position lives OUTSIDE the template, beyond the reach of the Animate tween system.

Concretely:
- The scroll offset is a private `Layer.scrollY int` field (`layer.go:24`), guarded by
  `scrollMu`, mutated ONLY by imperative methods: `ScrollTo`, `ScrollDown/Up`,
  `PageDown/Up`, `HalfPageDown/Up`, `ScrollToEnd` (`layer.go:201-262`). `blit` reads it
  and offsets the copy (`layer.go:264-273`).
- Scroll keys are imperative handler closures, not declarative: `BindNav`/`BindPageNav`
  on the log/scroll component append bindings like `func() { layer.ScrollDown(1) }`
  (`log.go:162-193`). They sit in `declaredBindings`, not in the view tree.
- The template reads the offset once per render via `ScrollY()` (`scrollview.go`) but
  there is NO binding path: nothing lets the template (or a tween) WRITE the offset each
  frame. The compiler's dynamic-value cases (`compileDyn*`) have no scroll-offset entry.

Meanwhile Animate drives a value declaratively by watching a pointer and writing an
interpolated scalar to storage the layout reads each frame (`compileTweenScalar`,
`template.go:1611`). The proven precedent is `VBox.Height(smooth(&h))`
(`example_test.go:2354`) — an animatable scalar driving layout. Scroll offset has no
equivalent animatable-scalar seam; it is a method-mutated field, so a tween can't reach
it.

Consumers: any scrollable content view (chat, log, diff) that wants smooth or animated
scroll instead of a hard jump.

## Considered Options

1. **Status quo** — scrolling stays imperative; no animation possible.
2. **Bound, animatable scroll offset** — expose the scroll offset as a template-read
   scalar a tween can drive (mirroring `Height(smooth(...))`). Nav keys set a tween
   TARGET instead of calling `ScrollDown` directly; Animate eases the offset toward it.
   The imperative methods stay (additive, back-compatible).
3. **Fully template-wired ScrollView** — offset, scrollbar, and key controls all
   declarative; a larger redesign of the component.

## Decision Outcome

Chosen option: **2, a bound animatable scroll offset** — the minimal seam that makes
scroll animatable and declarative, with a path toward 3.

Concrete shape — what a consumer writes. The scroll target is framework-owned managed
state, declared inline at build time (no user var to wire); `ScrollState()` returns a stable
`*int` the ScrollView tracks. Because glyph builds the tree ONCE (the ForEach body is even
compiled once with a dummy element), this is a one-time setup call — there is no per-frame
re-allocation; the React-hooks "called every render" failure mode does not apply here.

The headline — the whole point of this proposal — is this one line: animated,
framework-managed scroll with NO var and NO pointer to wire:

    sv := ScrollView.Grow(1).ScrollOffset(
        Animate(ScrollState()).Duration(120 * time.Millisecond).Ease(EaseOutCubic),
    )(chatRows...)

    // nav keys drive it THROUGH the ScrollView; the bound offset eases to the target:
    app.Handle("<C-d>", sv.HalfPageDown)  // smooth half-page down — sets the target, Animate eases
    app.Handle("<C-u>", sv.HalfPageUp)

`ScrollOffset(Animate(ScrollState()))` reads as what it does: the scroll offset is a
framework-managed cell (`ScrollState()`, allocated once at build), driven by a tween
(`Animate`), bound as the ScrollView's offset (`ScrollOffset`). The load-bearing tie:
when the offset is BOUND, the ScrollView's existing imperative scroll methods —
`ScrollTo`, `ScrollDown/Up`, `PageDown/Up`, `HalfPageDown/Up`, `ScrollToEnd` — SET THE
TWEEN TARGET instead of writing the raw offset, so every scroll animates. No var, no
pointer, no hand-cut: `sv.HalfPageDown` smooth-scrolls.

Today the same view is `BindVimNav()` + `layer.HalfPageDown()` — a hard cut. The only
change for the consumer is `ScrollOffset(Animate(ScrollState()))` on the constructor;
the same scroll methods they already call now ease instead of jumping.

Secondary form — when a consumer needs the VALUE (render "row N of M", drive a custom
scrollbar, or set an arbitrary target), hold the `*int` `ScrollState()` returns:

    scroll := ScrollState()
    sv := ScrollView.ScrollOffset(Animate(scroll).Duration(120 * time.Millisecond))(rows...)
    app.Handle("<C-d>", func() { *scroll += sv.HalfPage() })  // or read *scroll for "row N of M"

`ScrollOffset` accepts a plain `*int` (instant, no animation) or a tween over that int
(animated) — exactly how `Height` accepts `*int16` or `Height(smooth(&h))` today.
Internally `Layer.ScrollY()`/`blit` read the bound value each frame instead of the
private field. So: `ScrollState()` inline + the ScrollView's methods is the default
(no var); the returned `*int` is the escape hatch when you need to read or re-bind it.
One primitive, two depths of use.

**Per-item via the existing machinery.** A `ScrollState()` marker inside a `ForEach` body is
seen once when the sub-template compiles; the compiler flags it for per-item allocation
resolved at render by `elemBase` — which is exactly the bounded per-item-state store
(`perItemCache`) already in the engine. So `ScrollState()` outside a ForEach is one managed
`*int`; inside, it is one per item keyed by `elemBase`, on the same offset + bounded-eviction
path. No new mechanism — it reuses what is there.

Because:
* it reuses the proven scalar-tween path (`Height(smooth)`), so animated scroll falls out
  of the existing engine rather than a bespoke mechanism;
* it is the smallest change that makes scroll both animatable AND declarative, and it
  keeps the imperative `ScrollTo/ScrollDown` API working (the bound path is additive);
* NOT option 1 — animation is the whole goal; NOT option 3 wholesale yet — the offset
  seam is the load-bearing 80%; a fully-declarative scrollbar and key layer can follow
  once the offset binding proves out.

## Technical

- **Offset source.** Layer holds an optional bound offset (pointer or tween-storage);
  `ScrollY()` and `blit` read it when set, else the private field (back-compat).
- **Clamping — clamp the VALUE, not just the read.** The offset must clamp to
  `[0, maxScroll]`, and `maxScroll` is content-dependent (computed at render). Clamp-on-
  read alone is NOT enough: if a target overshoots, the tween keeps the raw value out of
  range; then if content GROWS (maxScroll rises), the clamp releases and the drawn offset
  JUMPS to the stale raw value. So write back the clamped offset (and/or re-clamp the
  target when maxScroll changes), so the stored value is never sitting out-of-range
  waiting to snap. (The shrink case is benign by comparison; the grow case is the glitch.)
- **Concurrency — reuse the existing lock-free pattern, don't re-derive.** This is the
  same shape as the jump-mode re-entrancy: a consumer's layer Render callback calling into
  the Layer during Execute would re-enter `scrollMu` and deadlock — which is why
  `JumpMode.active` is an `atomic.Bool` (pinned by `TestJumpModeLayerRenderNoDeadlock`).
  The bound offset should be read lock-free on the frame path the same way (atomic, or
  written at eval-time under the no-lock-across-Render discipline), following that proven
  approach rather than a fresh attempt.
- **Pending-vs-manual ownership — dissolved by construction.** Today a stale programmatic
  pending scroll can clobber a later manual scroll: an at-bottom auto-refresh queues
  `ScrollTo(1<<30)` into the deferred `pendingScroll` slot, a manual scroll goes direct to
  the layer (bypassing that slot), and the stale pending re-applies at the next layout —
  yanking the reader back. The bound model removes the race rather than guarding it: there
  is ONE source of truth (the bound `scrollTarget`), read fresh every frame, last-write-
  wins. Both programmatic scrolls and manual nav keys write that same value, so there is
  no separate pending slot left to go stale. The auto-refresh-to-bottom case is then the
  follow-rule below: auto-target `maxScroll` only when the offset is already near the
  bottom, so a reader who scrolled away is never pulled down. Net: consumer-side guards
  that re-register a manual landing as the pending (last-write-wins by hand) delete
  cleanly once a view adopts the bound offset.
- **Keys.** `BindVimNav`/`BindScroll` move from calling `layer.ScrollDown` to setting the
  offset tween target — either a new binding variant or the same keys wired to a target.
- **`ScrollState()` marker + storage.** `ScrollState()` returns a stable `*int`. Outside a
  ForEach it is a single managed cell allocated once at build. Inside a ForEach body (seen
  once at sub-template compile) the compiler flags it for per-item allocation, resolved at
  render by `elemBase` through the existing bounded `perItemCache` — the same store that
  backs the per-item tween/branch state. No new storage mechanism is introduced.

## Out of scope for the first slice

- **Per-item managed-state mutation addressing.** For scroll itself this is moot — the
  offset is per-ScrollView, not per-row, so the consumer holds the instance/pointer and
  mutates freely. If `ScrollState()` later becomes a general per-item primitive, the
  consumer model is mutate-within-the-item's-render-scope (the item and its state are both
  in hand there); cross-item mutation is the rare case and is served by an event carrying
  item identity, NOT a second pointer API. Noted as a deliberate future consideration; it
  does not gate this slice.

## Migration

Additive: the imperative scroll API and existing bindings keep working unchanged;
`.ScrollOffset(...)` + target-setting bindings are opt-in. A consumer adopts animated
scroll by binding an offset and switching its nav keys to target-setters.

## Risks

- Concurrency regression if the offset read/write doesn't keep the no-lock-across-Render
  discipline.
- Overshoot/clamp correctness when content height changes during an animation.
- Stick-to-bottom / auto-follow (a log that tracks the latest line) becomes just another
  target-set: auto-target `maxScroll` on a new line so it animates like everything else.
  The conflict-resolving rule: only auto-follow when the current offset is within a small
  threshold of the bottom; if the user has scrolled away, leave them where they are (don't
  yank them down mid-read). No special path — follow is a conditional target-set.
- Stale pending vs external manual scroll: covered by the single-source-of-truth offset
  (no separate pending slot to go stale) plus the follow-rule; a regression test should
  pin that a manual scroll after an at-bottom auto-refresh is NOT clobbered.
- Perf: the per-frame offset read must stay alloc-free; the scalar-tween path already is,
  but benchmark the scroll case.

## Lineage

Requested to enable animated half-page scrolling in a chat view, which the current
out-of-template scroll model can't express.
