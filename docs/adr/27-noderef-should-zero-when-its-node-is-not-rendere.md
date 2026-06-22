# ADR 27: NodeRef should zero when its node is not rendered this frame

- status: accepted
- date: 2026-06-22 11:58:56
- proposer: Glyph Smith@tui
- parties: code_review, recap, tui, Glyph, Pete
- deliberation: recap proposal show 27 (8 comments)

# NodeRef should zero when its node is not rendered this frame

## Problem

A `NodeRef` is written only during the render walk (`template.go:7982-7988`, and the
parallel site ~8577). When a node is gated out — `If(false)`, an unselected
`Switch`/`Match` branch, or otherwise not laid out — the walk never visits it, so its
ref **retains the previous frame's** X/Y/W/H and Opacity.

This violates the documented contract (`template.go:95`): *"NodeRef holds a node's
rendered screen bounds, populated each frame after layout."* A node with no rendered
bounds this frame reports stale bounds and `Opacity 1.0`.

Verified repro (FAILS on master) — an `If(&show)`-gated bordered VBox:

```go
func TestNodeRefStaleWhenGatedOut(t *testing.T) {
	var ref NodeRef
	show := true
	view := VBox(If(&show).Then(
		VBox.Border(BorderRounded).NodeRef(&ref)(Text("overlay content")),
	))
	tmpl := Build(view)
	buf := NewBuffer(40, 6)
	tmpl.Execute(buf, 40, 6)          // show=true:  ref = X0 Y0 W40 H3 Op1.00
	show = false
	buf.Clear()
	tmpl.Execute(buf, 40, 6)          // show=false: ref = X0 Y0 W40 H3 Op1.00  <-- STALE
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("STALE: gated-out ref kept geometry W%d H%d", ref.W, ref.H)
	}
}
```

## Consumers affected (systemic — every ref reader)

- **Screen-effect dodge/coverage** (`SETint`/`SEVignette`/FocusShade `.Dodge(&ref)`):
  a closed overlay leaves a phantom rect the effect still exempts → a bright,
  un-dimmed hole. This is the root of the #379 FocusShade saga — recap hit it on all
  three dodged overlays (? help / prompt / omnibox).
- **Jump scope** (`EnterJumpScope` / `ScopeRects []*NodeRef`): a gated-out scope rect
  still constrains jump targets to where hidden content used to be.
- **Hit-testing / anchored placement** reading a now-hidden node's geometry.

Today each consumer must clear the ref at the gated node's hide event (recap did N
close-site clears, c938/c943) — bookkeeping that every future ref-consumer must
remember, forever.

## Proposed contract

A `NodeRef` reflects **this frame's** render. A node not laid out this frame has zero
bounds: `W=H=0`, `Opacity=0`, `opacitySet=false`. Coverage / dodge / hit checks then
fail naturally on a not-rendered ref — no app-side clears, and every future consumer is
immunised by default.

## Implementation sketch (low risk, perf-safe)

- At compile, collect the ops that carry a `NodeRef` (a `t.refOps` slice).
- At the start of each `Execute` render phase, zero every attached ref.
- The render walk repopulates the refs of visited (rendered) ops, exactly as today.
- Net cost O(#refs) per frame — negligible. Add a benchmark proving no per-frame
  regression (repo policy: measure, don't reason).

## Design fork for Pete

- **(a) Zero the rect — recommended.** Simplest; immunises all consumers; deletes
  recap's N close-site clears. Matches the documented "this frame" contract. Cost: a
  consumer wanting *last-known* geometry of a now-hidden node loses it (can cache its
  own — and depending on stale geometry is the footgun we're removing).
- **(b) Add a `rendered-this-frame` bit on NodeRef.** Preserves last geometry but every
  consumer must check the bit; more API surface, more per-consumer burden, and it
  re-introduces the same footgun for anyone who forgets to check.

Recommendation: **(a)** — it removes bookkeeping rather than adding it.

## Risks

- A consumer currently (perhaps unknowingly) relying on stale geometry of a hidden node
  changes behaviour. Surveyed glyph's own ref consumers (effect dodge, jump scope,
  selection/anchor refs) — none want stale; all want "not rendered = absent."
- **ForEach / retained branches:** confirm per-instance refs zero correctly when their
  item or retained branch drops. Tie to the day-one binding rule + a ForEach regression
  test (two items, one dropped, asserts the dropped item's ref zeroes while the kept
  one stays live).
- **Perf:** benchmark the per-frame ref-reset to prove no regression.

## Lineage

Surfaced by Komorebi (@code_review) reviewing recap #436; mechanism verified here with
the repro above. Resolves the #379 FocusShade root cause at the framework level, letting
recap delete its app-side close-site clears.
