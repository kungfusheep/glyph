# ADR 33: Bounded perItemCache: evict realloc-orphaned ForEach per-item state

- status: accepted
- date: 2026-06-25 16:32:03
- proposer: Glyph Smith@tui
- parties: code_review, recap, tui
- deliberation: recap proposal show 33 (3 comments)

## Context and Problem Statement

Glyph keeps per-ForEach-item runtime state in maps keyed by `elemBase` (the slice
element address). When a ForEach slice grows by `append`, its backing array reallocates,
so every element address changes — the old keys orphan and are never deleted. Over a
session this leaks O(n) entries. The headline trigger is chat/message lists, which grow
by `append` constantly.

The leak class is **any `elemBase`-keyed per-item state inside a ForEach over an
append/realloc slice**. Members today (all unevicted):

- `opIf.itemBranches` (`template.go:2850`) — `*branchSelector` per item. `selector(elemBase)`
  is called UNCONDITIONALLY on every If-in-ForEach render (5715/5973/8305/8898…) and
  lazily creates an entry, animated or not. This is the **headline** case: a chat row has
  several `If`s (selected, unread dot, exit-fade), so each appended message orphans
  several selectors per realloc — independent of markdown, and more entries than the md
  cache (more If nodes than Rich nodes per row).
- `opSwitch.itemBranches` / `opMatch.itemBranches` — same shape.
- per-item tween maps: `perItemFloat64State` (`1740`/`1952`), `perItemColorState` (`2536`),
  `perItemStyleState` (`2734`) — same `elemBase` key, no eviction.
- Only `exit.renderingItems` (`264`/`342`) evicts (it `delete`s).

Surfaced during review of the markdown primitive; the md cache `mdCacheMap` was the first
instance and is now bounded — that fix is the prototype for this generalization.

## Considered Options

1. **Per-site bespoke sweeps** — copy the md seq-stamp eviction into each of the six maps.
2. **One shared bounded `perItemCache[T]`** — extract the prototype into a generic
   per-item store with a single eviction policy; back opIf/opSwitch/opMatch + the three
   tween maps with it.
3. **Accept the tradeoff** — document that elemBase maps don't evict (status quo, matches
   the original itemBranches design).

## Decision Outcome

Chosen option: **2, a bounded `perItemCache[T]` TYPE, instantiated once per existing
map** — a generic store keyed by `unsafe.Pointer` (elemBase) holding `*T`, with the
seq-stamp + cap sweep from the md fix: each access stamps a monotonic seq; when the map
overgrows a cap, entries not touched within the last `cap` accesses are evicted.
opIf/opSwitch/opMatch branch maps and the three tween-state maps each get their OWN
instance; the md cache moves onto it too (dropping its bespoke copy).

**Load-bearing constraint: one instance PER map, never a single shared store.** A merged
store breaks two ways:
* **key collision** — keys are `elemBase` (the item address). Two different ops rendering
  the same item share that address; in a shared map they collide and read each other's
  state. Per-instance keeps them disjoint (as today).
* **the cap invariant** — the "cap > peak single-frame accesses" live-safety only holds at
  ~1 access/item/frame. Each existing map IS 1/item/frame (opIf.selector once per render;
  each tween property is its own map). A merged store would see (items × sites)
  accesses/frame, shrinking the margin and risking mid-frame eviction of live items. State
  the invariant in ACCESSES/frame and keep it 1/item/frame by staying per-site.

Because:
* the leak is one shared responsibility (per-item state, elemBase-keyed, survive frames
  but evict realloc-orphans) — not six coincidentally-similar shapes; one eviction policy
  proven once beats six that drift.
* per-site sweeps (option 1) duplicate subtle logic six times — divergent bugs, six tests
  of the same property.
* doing nothing (option 3) leaks in the headline chat case, against "performance is a
  feature"; the md instance already showed it's real.

This is a justified abstraction: there's a proven six-site use case today, not a
speculative one.

## Technical

- `perItemCache[T any]` with `get(key) (*T, bool)`, `put(key, *T)`, and internal
  seq-stamp eviction (cap default 1024, set above any realistic single-frame item count
  so live entries never evict mid-frame).
- Migrate: `opIf/opSwitch/opMatch.itemBranches`, `perItem{Float64,Color,Style}State` maps,
  and `mdCacheMap` (drop its inline copy).
- Eviction is orphan-only: live and exit-animating (retained) items are touched every
  frame, so they survive until their branch genuinely drops — consistent with
  `exit.renderingItems` and the NodeRef-zeroing retention rule (ADR 27).

## Risks

- **Mid-animation eviction** — must never evict an item whose tween/exit is still
  rendering. Safe by construction (those items are touched every frame → recent seq), but
  every migrated site gets a churn + exit-animation regression test. "Safe by
  construction" means *never evicts a live item* — NOT "no animation artifacts ever".
- **Out of scope: realloc-during-animation continuity.** If a slice reallocates WHILE an
  item is exit-animating, that item's `elemBase` changes mid-fade, so its branchSelector/
  tween state resets at the new key (a possible fade glitch). This is inherent to
  `elemBase` keying — pre-existing, neither introduced nor fixed here — and is a separate
  concern from the leak.
- **Cap vs live set** — cap must exceed the largest single-frame rendered-item count;
  documented + asserted.
- **Hot path** — `get`/`put` must stay alloc-free on the steady-state hit; benchmark.

## Lineage

Review of the markdown primitive traced the family and recommended the consolidation; the
markdown cache fix is the prototype. Generalization owned by tui.
