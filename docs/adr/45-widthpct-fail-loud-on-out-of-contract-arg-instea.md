# ADR 45: WidthPct: fail loud on out-of-contract arg instead of silent no-op

- status: accepted
- date: 2026-06-25 17:54:07
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 45 (0 comments)

# WidthPct: silent no-op on int / unmatched arg, and fraction-vs-percent contract

## Context and Problem Statement

A consumer wrote `VBox.WidthPct(42)` as an HBox column and the column collapsed to
zero width — silently, with no build error and no visual hint. Root cause is two
stacked issues:

1. **Silent drop of unmatched arg types.** `WidthPct(pct any)` is a type switch over
   `float32 | float64 | *float32 | conditionNode | tweenNode | OscC`. There is no
   `case int`. `WidthPct(42)` passes an `int`, matches nothing, and falls through —
   `percentWidth` stays 0. The element then has no width, no percent, and no flex, so
   it is treated as implicit-flex and a `Grow` sibling starves it to 0. `Grow` (and
   `Width`/`Height`) DO accept `int`, so this is an inconsistency as well as a footgun.

2. **Fraction vs percent contract.** `WidthPct` stores a `0.0–1.0` fraction
   (`PercentWidth float32 // 0.0-1.0`). The name "Pct" invites `WidthPct(42)` meaning
   "42%", but the working call is `WidthPct(0.42)`. Even if `int` were accepted as the
   stored value, `42` would mean 4200%.

The defect is not wrong output for valid input — `WidthPct(0.42)` lays out correctly.
It is that misuse is swallowed silently and the name actively invites that misuse.

## Considered Options

1. **Status quo** — document `WidthPct` as 0–1 and that ints are ignored. Leaves the
   silent-zero footgun and the name trap.
2. **Accept int as a percentage (0–100), keep float as 0–1.** `WidthPct(42)` → 0.42;
   `WidthPct(0.42)` → 0.42. Intuitive for the name, but a dual-contract is subtle and
   `WidthPct(1)` is ambiguous (1% or 100%?).
3. **Keep the 0–1 fraction contract; make unmatched/likely-misused args fail LOUD.**
   Accept the documented types; for an out-of-contract arg (an int, or a float > 1)
   panic at BUILD time with a clear message ("WidthPct takes a 0.0–1.0 fraction; got
   42"). Build-time panics are acceptable in glyph's compile-once model — they fire
   once at construction, not per frame, and surface developer error immediately.

## Decision Outcome

Proposed: **Option 3** — keep the 0–1 fraction contract (consistent with the internal
field and any other fractional APIs), and replace the silent fall-through with a
build-time panic on an out-of-contract argument. Rationale:

- the real harm is SILENCE; a degenerate layout value that vanishes an element should
  never pass quietly. Loud-at-build matches "performance is a feature" (no per-frame
  cost) and surfaces the mistake at the call site.
- it avoids a dual 0–1/0–100 contract (option 2), which trades one ambiguity for
  another (`WidthPct(1)`).
- a clear panic message that names the fraction contract also fixes the name trap in
  practice — the developer who writes `WidthPct(42)` is told exactly what to write.

Open question for review: whether to ALSO accept `int` 0/1 as a convenience, or reject
all ints as out-of-contract (recommended — an int percent is the misuse we are
guarding). Default to reject-with-message unless there is a real 0/1 use case.

## Amendment (post-review)

The build-time panic is on the wrong TYPE only — NOT on a float's range. A percent
width outside `[0,1]` is legitimate: a static `WidthPct(1.2)` is a deliberate 120%
over-width, and animations routinely overshoot past 100% or below 0% (spring/overshoot
easing). The original range guard would have rejected valid values. (Animated/dynamic
percents were never range-checked — they flow through the `*float32`/condition/tween/osc
arms — so only static literals were affected, but static over/under-percent is valid
too.) The guard that remains is the one that catches the actual silent-drop footgun: a
wrong-typed argument (an `int`, or any unmatched type) panics; floats of any magnitude
are accepted.

## Technical

- `WidthPct` (and the `HBoxFn`/`VBoxFn` variants) gain a `default:` arm in the type
  switch that panics with a fraction-contract message (catching `int` and any other
  unmatched type — the silent-drop bug). `float32`/`float64` are accepted at ANY value
  (no range clamp); `*float32`/condition/tween/osc cases are unchanged.
- This is construction-time only; no change to the layout hot path.

## Risks

- A panic is a behaviour change for any caller currently passing an out-of-contract
  value and getting silent-zero. That is exactly the buggy case being surfaced; a
  caller relying on silent-zero is already broken. Worth a one-line note in release
  notes.

## Migration

Existing valid callers (`WidthPct(0.42)`, `WidthPct(&p)`) are unaffected. Callers
passing an int or >1 float were silently broken and now get a clear build-time error.
