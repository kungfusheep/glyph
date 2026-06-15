# ADR 4: PreserveBG: transparent rune writes

- status: accepted
- date: 2026-06-13 09:41:29
- proposer: Glyph Smith@tui
- parties: recap, tui
- deliberation: recap proposal show 4 (2 comments)

# PreserveBG — transparent rune writes

consumers: recap (Kestrel, m128: focus underline spanning panes; badges over striped
lists; decorations over banners)
related: SetOpacity compositing path (default-BG source keeps destination bg)

## problem

A glyph drawn over varied backgrounds replaces the whole cell style, so overlay
decorations must hand-maintain colour matching of whatever sits beneath. recap's
focus underline colour-matches the focused pane at the focus event: exact when
settled, approximate mid-tween. The needed compositing rule already exists in the
opacity path; ordinary writes just can't reach it.

## proposal

Per-node opt-in routing that node's cell writes through the bg-keep rule:

    Text("▁▁▁").FG(&accent).PreserveBG()

Writes set rune + FG, keep the destination cell's BG. Applies to the node's own
writes only (not children); available on Text/Textf/Rich first, containers later
if a case appears.

## implementation sketch

a style attr bit (AttrPreserveBG) carried into the cell write path; Set/SetFast
check it and merge dest BG, mirroring SetOpacity's default-BG-source rule. Zero
cost when unset (existing attr-bit check).

## risks

- border-merge and fill-cascade paths must ignore the bit (writes only).
- SetFast's no-merge fast path gains one branch; benchmark before/after
  (buffer_bench_test.go covers the write paths).

## migration

recap deletes the focus-event colour matching; mid-tween underline becomes exact.
