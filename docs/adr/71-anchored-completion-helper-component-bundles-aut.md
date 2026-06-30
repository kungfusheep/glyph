# ADR 71: anchored-completion helper component (bundles autocomplete-over-a-field mechanics)

- status: accepted
- date: 2026-06-30 15:20:03
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 71 (4 comments)

# Proposal: an anchored-completion helper component

## Problem

An autocomplete dropdown over a focused text field — type a prefix, a list of matches
appears anchored below the caret, ↑/↓ moves the selection, Enter completes, Esc
dismisses, and **every other key keeps editing the field** — is fully *composable*
from primitives glyph already has, but it's fiddly to wire by hand and easy to get
subtly wrong. The motivating case is @-mention autocomplete in a chat composer, and the
generic "pick from a filtered list while typing" pattern (command palettes, path
completion, tag pickers).

The pieces that already exist:

- **Anchoring** — `opOverlay` already carries `anchor *NodeRef` + `AnchorBelow`/
  `AnchorAbove`. Put a `NodeRef` on the field, anchor the dropdown to it. No change.
- **Key precedence without losing the editor** — riffkey `TextInput` is literally
  `HandleUnmatched(textHandler)`, so *explicit* pattern bindings match first and only
  unmatched keys fall through to the buffer. Register ↑/↓/Enter/Esc as explicit
  handlers active while the dropdown is open; everything else still edits. The field
  never loses focus — same router, the nav keys just win the match.
- **"Active only while open" gating** — the modal-router `Enable`/`Disable` + `Push`
  machinery already does exactly this.

So nothing new is *needed*. The cost is the wiring: a `NodeRef`, an anchored overlay,
a conditionally-active nav router with the right precedence, and the filter — composed
correctly and torn down cleanly on close. Hand-wiring the router precedence and the
open/close lifecycle is the part that's easy to get wrong (nav keys leaking into the
text, or the dropdown swallowing keys after it should have closed).

## Who this is for

- **A chat composer** — `@`+prefix opens a name dropdown over the composer; pick to complete.
- **Any "complete-while-typing" field** — command/file/tag completion. A reusable
  helper means each consumer composes it in one call instead of re-deriving the
  router-precedence dance.

## API shape

A component that bundles the field + its completion, owning the lifecycle:

```go
Complete(&text, candidates).         // candidates: a source the helper filters by the live prefix
    Trigger('@').                    // optional: only arm after this rune (else: always, on any prefix)
    OnPick(func(c Candidate) { … }). // how a selection completes into the buffer
    Anchor(AnchorBelow)              // where the dropdown sits relative to the field
```

- It renders the underlying `Input` (wrapped so the dropdown can anchor to its rect via
  a `NodeRef`), and when armed + non-empty matches: renders the dropdown overlay
  anchored below the field. The ↑/↓/Enter/Esc keys are declared as explicit bindings on
  the field's own router and their handlers switch on open-state.
- Closed state is zero overhead — no overlay; the field behaves as a plain Input.
- Filtering: a case-insensitive prefix match over the candidate pool (the mail-style
  To: behaviour); fuzzy ranking is an easy later swap.

## Implementation sketch (as built)

- A `CompletionC` (templateTree/compound component) expanding to `VBox(HBox(Input)`
  with a `NodeRef`, `If(open).Then(Overlay.Below(ref)(dropdown)))`. Open-state is
  recomputed in the field's `OnChange` (token after the last trigger before the caret →
  prefix-filter → `open = matches>0`), so the declarative `If` keeps it in sync with typing.
- **The key mechanism — single router, no push/pop.** riffkey's `Input.Dispatch`
  consults only the TOP stack frame; there is NO fall-through to routers beneath, so a
  pushed/modal nav router would CAPTURE every key and stop text editing. Instead the
  nav keys are explicit bindings on the SAME router as the field's `TextInput`
  (HandleUnmatched). riffkey matches explicit patterns before the unmatched text
  handler, so the nav keys win *while every other key still edits the field* — exactly
  "keep editing while the dropdown is open", with no lifecycle to manage. The handlers
  no-op (or delegate) when closed: Enter→`OnSubmit`, ↑/↓/Esc→nothing.
- Reuses `opOverlay` anchoring (`Overlay.Below`), `Custom` for the dropdown rows, and a
  prefix filter. The new code is the composition + the open-state token logic, not a
  new primitive.

## Risks / notes

- **Router precedence is the load-bearing detail** — verified against riffkey rather
  than assumed: explicit bindings beat the `TextInput` HandleUnmatched on the same
  router, and a compound component contributes both its `bindings()` (nav) and
  `textBinding()` (the field) onto that one router. Day-one tests cover the open→↑/↓
  →Enter-picks path and the closed→Enter-submits path.
- **Focus model** — the field stays the focused editor throughout; the dropdown is not
  a separate focus stop, because there is no separate router.
- **Scope** — pairs with but is independent of the per-rune style resolver (the
  type-time-highlight slice). This needs neither it nor a style change; orthogonal slices.

## Alternative considered

Hand-wire it from the raw primitives in each consumer with glyph-seat guidance (no new
glyph surface). Viable — everything needed exists — but it pushes the error-prone
router-precedence detail into every consumer, and this won't be the last
complete-while-typing field. A small reusable helper pays for itself the second time.
