# ADR 137: Layer scroll-offset binding goes public: bare LayerView panes can glide

- status: accepted
- date: 2026-07-23 17:57:58

## Context and Problem Statement

The diff and proposal-document panes teleport on scroll. Their Layer already carries the full bound-offset ease machinery — blit interpolates toward `ease.target` and self-schedules frames — but the ARMING is private: only ScrollView wires `ease.target/dur/fn`. Both consumer panes are bare LayerViews, so they cannot opt in. This is the cheap half split out of the List-easing deliberation at the reader's request; it lands independently and first.

## Considered Options

1. Wait and land it with the List row-offset work.
2. Expose the arming now as a chainable `LayerView(l).ScrollOffset(offset any)` — the same declarative form, name and argument as ScrollView's `ScrollOffset`, on the node that mounts the layer.

## Decision Outcome

Chosen option: 2. The surface is declarative, because that is where the consumer already lives: the panes hold a `*Layer` handle but MOUNT it in the template, so the ease policy is declared at the mount, chained exactly like ScrollView's headline form. Usage, the whole change for a consumer pane:

```go
// before
LayerView(propUI.Layer).Grow(1)
// after — the pane glides; ScrollTo/ScrollToTop etc. now ease to their target
LayerView(propUI.Layer).ScrollOffset(Animate(ScrollState())).Grow(1)
```

The argument is exactly what ScrollView's `ScrollOffset` takes (same name, same `any`, one surface to learn): a `*int` binds an instant offset, an `Animate(...)` binds eased, dispatch internal — rather than a hand-assembled triple (`tweenNode` is unexported, so `any` is also the only workable public form). An earlier draft named this `Layer.SetScrollEase`, then `SetScrollOffset`, on the handle; the chainable form replaced it when the reader asked what it looks like to USE — `Set` is the idiom of the imperative handle, but arming is declaration, not mutation. The method's doc states the pointer contract explicitly, at its full width: the bound value is the LIVE position, written by the framework on every scroll — all scroll methods route through the same locked path, which assigns the target in place — with clamping as content grows or shrinks just one case of that. A consumer arming with its own `&offset` hands glyph authorship of it. Verified ready by both reviewing seats before this split: `scrollToLocked` already routes every scroll method through `ease.target` when armed and falls back to the legacy path otherwise, so existing callers need no migration; consumer panes are bare LayerViews.

Review then found that arming splits position three ways — `ease.target` (what `ScrollY()` returns), the displayed offset blit draws from (unexported), and the legacy scroll field, which arming freezes and `ScreenCursor()` still reads. Both instances share one root: arming added a second position representation without migrating its readers. So the real scope is not "expose the arming plus patch two readers" — it is GIVE LAYER ONE POSITION, held as the target/displayed pair, with every reader AND writer derived from it — the writer side is real too: the content-reset paths zero the legacy field and never touch the ease target, so an armed pane opens each new document at the previous one's offset, and page navigation is what these panes do. The audit ledger is closed: readers are ScrollY (target), ScreenCursor (frozen legacy) and blit (displayed); writers scrollToLocked and updateMaxScroll are already migrated — updateMaxScroll's shape (branch on the armed target, touch the legacy field only when unarmed) is the pattern the two unmigrated writers, SetContent and SetBuffer, need:

* `ScrollY()` returns the DESTINATION once armed — deliberate, documented at the method; right for "am I near the bottom".
* A companion accessor exposes the DISPLAYED offset (what blit last drew). Without it, "which rows are visible" is publicly uncomputable mid-ease, and both consumer panes need exactly that for jump-mode label ranges.
* `ScreenCursor()` (and anything else reading the legacy field) must track the displayed offset when armed — an armed layer with a visible cursor must not place it as if never scrolled. This is not only a going-public risk: ScrollView hits it TODAY — it arms itself for any `ScrollOffset` and then feeds the frozen legacy field into its jump viewport, so jump targets land offset by the whole scroll (repro: after `ScrollTo(10)`, target 10, displayed 10, legacy field 0). The one-position redesign fixes a live defect, not a hypothetical.

The verification bar is the surface's own: a pin that arming changes nothing for unarmed Layers, the ease-through-scroll test at the Layer level, plus mid-ease pins for the two seams above — visible-range from the displayed accessor moves with the glide, and cursor placement on an armed, scrolled layer is correct (the ForEach rebind pin belongs to the List seam, which stays in its own proposal).

## Agreed todos

- [ ] `LayerView(l).ScrollOffset(offset any)` exists, pinned so unarmed Layers behave exactly as before.
- [ ] Layer holds ONE position (the target/displayed pair) with every reader derived from it: a public displayed-offset accessor exists, `ScreenCursor()` tracks displayed when armed, the content-reset writers reset the pair (a new document opens at the top on an armed pane), and no reader or writer is left on the legacy field — the derived paths pinned mid-ease and across a content swap.

## End-goal state

A bare-Layer consumer arms easing in one line; the diff and document panes glide the day their side opts in. The List row-offset work proceeds independently in its own deliberation.

## Non-goals

* The List/ForEach row-offset seam — stays in the original deliberation.
* Opting recap's panes in — that is the consumer's one-liner, landed from recap once this exists.
