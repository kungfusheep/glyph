# ADR 32: Native inline markdown primitive for glyph (Rich().Markdown())

- status: accepted
- date: 2026-06-22 21:58:39
- proposer: Glyph Smith@tui
- parties: code_review, recap, tui
- deliberation: recap proposal show 32 (2 comments)

# Native inline markdown primitive for glyph

## Problem

Glyph apps that show user/agent prose re-implement the same markdown tokeniser, and
render it inconsistently:

- recap hand-rolls `markupSpans` (bold / italic / `code` / `- ` bullets / `Label:`
  lead-ins) and renders it ONLY in review summaries; chat bubbles, comments, toasts use
  a plain `TextBlock(&body)`, so the same markup shows as RAW characters there.
- wed has a full `ParseMarkdown` (block model + inline runs) — a third tokeniser.

Every new consumer that wants styled prose must re-tokenise. There is no native,
bound, theme-safe markdown rendering primitive.

## What glyph already has (and what's missing)

`Rich(&[]Span)` already renders a LIVE span slice every frame, with wrapping + theming.
So the rendering substrate exists. The gap is the **source→spans** step: a bound
markdown string that tokenises to `[]Span`, re-read per frame like other bindings, and
composable inside a `ForEach`/`List` row (the live case: a per-message chat bubble).

## Proposed API (inline-first)

A `.Markdown()` mode on RichText — additive, not a new component family:

```go
Rich(&body).Markdown()          // bound *string, tokenised to inline spans each frame
```

Inline grammar (the 80%): `**bold**`, `*italic*`, `` `code` ``, `~~strike~~`, and `- `
bullets (the one block-ish affordance recap needs). Links optional, later.

Block-level markdown (headings, nested lists, code blocks, quotes) is explicitly OUT of
scope here — that's a later `MarkdownDoc(&str)` built on wed's block parser. Don't block
the 80% on the hard part.

## Consumers

- recap: chat bubbles (DM / group / blocked), comments, toasts, summaries — drops
  `markupSpans`, gets consistent rendering everywhere. Kestrel will fold recap onto it.
- any glyph app rendering agent/user prose.

## Implementation sketch

1. Tokeniser: lift wed's inline run-splitter (bold/italic/code/strike) → `[]Span`,
   carrying theme colours by Style (theme-safe: resolve colours at render, not parse).
2. Bind + cache: store the last-seen source string + its tokenised `[]Span` on the op.
   Each frame, compare the bound string to last; re-tokenise ONLY on change, else reuse
   the cached spans. This keeps steady-state zero-alloc (glyph's cardinal rule) — the
   parse cost is paid on edit, not per frame.
3. ForEach composition (the crux): the bound `*string` gets the offset-resolved
   per-item treatment (same as `Text` in a ForEach), so each row tokenises its OWN
   value. The per-item span cache is keyed by item identity (elemBase), not shared —
   the frozen-placeholder failure mode the binding day-one rule exists to prevent.
4. Render: reuse the existing RichText span path (wrapping, PreserveBG, CharWrap all
   come free).

## Risks

- **ForEach shared-cache bug** — if the span cache is on the single compiled op it'll be
  shared across items. Must key per item (elemBase). Day-one regression test: a two-item
  ForEach with different markdown bodies asserts each row renders its own styling.
- **Per-frame parse** — naive impl tokenises every frame (allocates). The change-detect
  cache is mandatory, with a benchmark proving steady-state zero-alloc.
- **Grammar scope creep** — keep it inline-only; resist headings/tables here. A `Markdown`
  that silently half-supports block syntax is worse than a clear inline primitive.
- **Theme re-read** — colours must resolve per frame (bound theme), so a theme switch
  restyles cached spans without re-tokenising. Tokenise to semantic styles, resolve to
  colours at render.

## Lineage

Pete wants native markdown; Kestrel (recap) consolidating off its hand-rolled markup
(m1105 thread). Prior art: wed's ParseMarkdown. Discussion converged on inline-first +
parse-on-change-cache + ForEach-composable.
