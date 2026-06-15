# ADR 11: Incremental multi-char jump-label feedback

- status: accepted
- date: 2026-06-15 13:17:14
- proposer: Kestrel@recap
- parties: recap, tui
- deliberation: recap proposal show 11 (4 comments)

# Incremental multi-char jump-label feedback

## Problem

When jump mode assigns multi-character labels, the engine already accumulates
the typed prefix (`jumpMode.Input`) and `HasPartialMatch` narrows on it, so the
machinery is correct. What is missing is visual feedback: every label paints
uniformly via `jumpStyle.LabelStyle` regardless of what has been typed, so a
user partway through a two-key label gets no signal that their first keystroke
matched (or which labels are still live). On dense target sets this invites a
wrong first key, which cancels the pick silently.

A recap reviewer asked for exactly this in recap's diff pane (line-range comment
picking), but the affordance belongs to the jump engine, not the consumer:
consumers only call `EnterJumpMode`/`AddJumpTarget` and the paint is the
framework's.

## Proposed change

`paintJumpLabels` splits each label at `len(jumpMode.Input)` and paints the
already-matched prefix in a distinct style from the remaining characters, so
typed progress is visible. Routing and `HasPartialMatch` are unchanged.

## API surface

Additive, no breaking change to the call sites:

- `JumpStyle` gains a `MatchedStyle Style` field for the typed-prefix run.
  `LabelStyle` keeps its current meaning (the not-yet-typed remainder).

## The one decision (Pete's verdict): default policy

The owner (Glyph Smith, pc76) endorsed the shape and isolated the single call
that is Pete's to make — what a zero `MatchedStyle` does:

- **Option A — default-OFF (opt-in).** Zero `MatchedStyle` paints uniformly as
  today; a caller opts into the feedback by setting it. Purest add, zero
  ratification angle: no existing jump UI changes at all.
- **Option B — default-ON (dim-by-default).** Zero `MatchedStyle` derives a dim
  from `LabelStyle`, so every existing jump consumer gets the feedback unasked.
  Technically a behaviour change, but a narrow, transient one: at rest
  `len(Input)==0`, so `label[:0]` is empty and `MatchedStyle` paints nothing —
  pixel-identical to today. The dim only appears DURING multi-char partial
  entry, a mid-keystroke state nothing can meaningfully depend on; single-char
  label apps never see it.

**Bundled sub-question (decided with the above, not split):** whether
`paintJumpLabels` also dims/elides labels whose prefix no longer matches
`jumpMode.Input`. Same transient window, same nature. If B wins, dimming the
now-dead labels is the natural completion; if A wins, gate both behind the
opt-in.

Proposer's recommendation (recap/Kestrel, non-binding): **Option B.** It gives
the better UX for free, the "change" is a flicker that exists only while a key
is mid-sequence, and at rest it is byte-for-byte today's render. But this is a
default-policy call and the verdict is Pete's; the owner will implement whichever
he signs.

## Constraint

The `label[:len(Input)]` split must be rune-safe. Today's labels are ASCII
home-row, so byte-slicing is correct under that assumption — state it explicitly
so a future multi-byte label set does not silently corrupt the split (slice on
rune boundaries, not bytes, if labels ever leave ASCII).

## Alternatives considered

- Consumer-side: not reachable. recap cannot influence the per-cell paint; it
  would have to reimplement the entire jump engine to get at the label
  rendering, which defeats the point of the shared component.
- Leave as-is and rely on short (single-char) labels only: caps the number of
  simultaneous targets and does not help the dense-diff case that surfaced this.

## Notes

Classified FEATURE (not a defect) by the tui owner in m398; raised as a proposal
per that steer, owner-endorsed in pc76. No implementation written ahead of the
verdict.
