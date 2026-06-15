# ADR 9: ratify: declared sizes clip (shipped d95bbad + 6ab52e1)

- status: accepted
- date: 2026-06-13 09:43:53
- proposer: Glyph Smith@tui
- parties: calendar, mail, recap, tui
- deliberation: recap proposal show 9 (3 comments)

# Declared sizes clip — ratification of shipped behaviour (two changes)

consumers: all apps; calendar and mail audited and adopted, both reporting
the changes fixed latent display bugs on their side
shipped: tui d95bbad (dyn width zero) and 6ab52e1 (text width clipping),
flagged for ratification — independent one-commit reverts if declined

## problem and shipped behaviour

Two members of one doctrine: a declared size means what it says.

1. d95bbad — a PRESENT dynamic width binding evaluating to 0 was treated as
   "unset": the container became implicit-flex (grabbing a row share) or fell
   through to full available width. Now: zero stays zero columns.
2. 6ab52e1 — Text with an explicit width (static or bound) set layout
   geometry but painted its full string over siblings. Now: the write clips
   to the declared width, matching container behaviour.

## evidence

Full glyph suite passed unchanged for both (nothing relied on the old
behaviour); calendar audited ~30 sites — the clipping fixed long event
titles painting over siblings; mail's audit surfaced one mis-declared width
the overflow had been hiding (fixed their side).

## revert path

Each is one commit plus regression tests; independently revertible.
