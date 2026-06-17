# ADR 13: A max-width primitive for content-that-clamps-to-a-budget

- status: accepted
- date: 2026-06-15 21:11:35
- proposer: Kestrel@recap
- parties: recap, tui
- deliberation: recap proposal show 13 (3 comments)

# A max-width primitive for content-that-clamps-to-a-budget

## Problem

glyph can size a container two ways: FitContent measures the intrinsic/longest
line with NO upper bound and won't wrap; Width/WidthPct force an exact width.
There is no middle: "wrap at N, then size to the longest wrapped line ≤ N" —
content that grows with its text but clamps at a budget.

This surfaced building the chat component (ADR 10): a bubble should be as wide as
its message up to a budget, then wrap and clamp there. WidthPct forces every
bubble to the full budget (a three-word reply gets a budget-wide bubble);
FitContent never wraps; and measuring the wrapped width in consumer Go would
reimplement glyph's text-width + wrap, which pc65 and ADR 9 say not to do.

## Consumers beyond chat

Any element that should hug its content but never exceed a cap:

- chat / speech bubbles (the immediate driver)
- toasts / notifications (clamp long text, hug short)
- tag pills / chips / badges (size to the label, cap runaway)
- min/max-width labels in dashboards and tables

All of them today either force a width (wasting space on short content) or risk
overflow (FitContent on long content).

## Proposed change

A max-width bound on VBox/HBox: the container wraps its wrappable content at the
bound and then sizes to the longest resulting line, never exceeding the bound and
never padding to it.

API shape (owner's call on the exact spelling):

    VBox.MaxWidth(n)        // n columns (static literal)
    VBox.MaxWidthPct(p)     // fraction of parent (dynamic)
    HBox.MaxWidth(n) / .MaxWidthPct(p)

A min-width counterpart (`MinWidth`) is the natural sibling for pills/labels but
can be a separate step; this proposal is scoped to max-width.

## Implementation sketch

Two paths, kept separate so the cost only falls where it's used (Glyph Smith's
perf note — do NOT add a universal measure pass):

1. **Non-wrapping / no wrappable content** — single-pass, unchanged: the bound is
   just an upper clamp on the measured width. No second pass.
2. **Wrap-then-measure** — only when a MaxWidth container actually holds
   wrappable content: wrap that content at the bound using glyph's EXISTING text
   layout (the path TextBlock already uses), then size the container to
   min(bound, longest produced line). No new width/wrap code — it drives the
   layout glyph has, with an upper bound instead of infinity (FitContent) or a
   fixed value (Width).

**Core work:** TextBlock reports its longest produced (wrapped) line, so the
MaxWidth container can size to it. That measurement reuse is the load-bearing
piece — explicitly NOT a consumer/primitive reimplementation of width+wrap
(pc65 / ADR 9).

ADR 9 composition: clean — the resulting declared width clips as usual.

## Binding (day-one rule for the dynamic form)

The static `MaxWidth(n)` literal has no binding and is exempt. But any DYNAMIC
form — `MaxWidthPct(p)` or a bound bound to app state — is a new pointer-bindable
surface and MUST get the sliceBinding/offset (isWithinRange) treatment, with a
ForEach regression test landing the same day: two bubbles in a ForEach must each
measure their OWN content, not share a frozen bound. This is the per-item-binding
class that has bitten repeatedly; non-negotiable for the dynamic variant.

## Alternatives considered

- Consumer measures wrapped width in Go and sets an exact Width per item:
  reimplements glyph's text-width + wrap (pc65/ADR 9 forbid), and risks
  off-by-one / unicode-width drift from glyph's real wrapping.
- Ship budget-width only (WidthPct): what chat v1 does; loses the content-hug
  that makes short bubbles read right. Acceptable stopgap, not the end state.

## Notes

Raised per ADR 10's escape hatch. Confirmed by the owner (Glyph Smith, m426/m438)
that no such primitive exists, the shape + value are endorsed, and this is the
proposal route; he will review/own it. No implementation written ahead of a
verdict. chat ships budget-width bubbles for v1 and adopts MaxWidth when it lands.

## Amendment (2026-06-17): MaxWidth is a pure cap; MaxWidthPct dropped

After review (recap #367), the semantics were simplified on the owner's call:

- **MaxWidth is a pure upper bound**, not a content-sizing mode. The component
  sizes by its normal rule (default fill, FitContent, Grow, WidthPct) and
  MaxWidth only clamps the result: `width = min(computed, MaxWidth)`. It implies
  no sizing of its own — the original "hug the longest wrapped line" behaviour is
  gone, because it baked a scaling rule into what should be just a constraint.
- The content-hug-to-a-budget use case (chat bubbles) is now a **composition**:
  `FitContent().MaxWidth(n)` — FitContent hugs, MaxWidth caps. Wrappable content
  wraps to the (narrower) capped width naturally, as a consequence of the
  container being narrower, not a special pass. `measureMaxWidthContent` (the
  wrap-then-measure helper) was removed.
- **MaxWidthPct dropped** — it overlapped with Grow + MaxWidth and had no
  consumer; revisit only if a concrete need appears.

This makes MaxWidth orthogonal and composable, at the cost of the
hug-to-longest-wrapped-line nicety (a budget-width box no longer shrinks to its
longest produced line; it stays the cap width). Accepted as the better trade.
