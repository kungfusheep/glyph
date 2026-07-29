# ADR 147: A shrinking terminal pane keeps its bottom rows, the way every terminal does

- status: accepted
- date: 2026-07-29 10:11:16

## Context and Problem Statement

`TermC` discards the bottom of the emulated screen whenever its pane gets shorter. Every terminal it is compared against discards the top instead. A full-screen program that pins UI to the last rows loses exactly that UI on the first shrink, and what remains is stale content from rows the program has already scrolled past.

`reshapeGrid` in `term/vt.go` copies the top-left overlap of the old grid into the new one. On a row shrink the surviving rows are `0..rows-1`; the rows below are dropped. `screen.resize` then clamps the cursor into the new bounds, so the cursor lands on content it did not write.

The consumer is the recap agents view (`recap/agents_view.go:577-604`), which renders a live Claude Code session inside a `term.Stream` component whose pane width is computed from the window (`agentsSizeCols`) and whose height is the lane's remaining space. Any window resize calls `TermC.resizeIfNeeded` (`term/term.go:363`), which calls `screen.resize`. Claude Code parks its cursor in a bottom-anchored input box, so every shrink hits the worst case.

Measured against tmux at the same geometry, replaying a 73KB capture of real Claude Code output taken from a live agent:

- Write the capture at 100x97 and compare grids: 0 differing lines. The same at 96x40, and byte-at-a-time chunking at both sizes: 0 differing lines. The parser is correct.
- Write the capture at 100x97, then resize to 96x40: 82 differing lines. tmux holds the live view, the spinner and the footer. glyph holds scrollback from the top of the old screen.

The rule real terminals follow, pinned with a controlled 10-row grid shrunk to 6:

- Cursor on row 5: rows 1 to 6 survive, the cursor does not move. It already fits.
- Cursor on row 10: rows 5 to 10 survive, the cursor lands on the last row.
- Growing 6 rows to 10 appends blank rows at the bottom and leaves the cursor alone. glyph already matches this.

So the shrink drops `max(0, cy - (rows-1))` rows from the top. Nothing else about the reshape changes.

## Considered Options

1. Leave the top-left reshape and have consumers avoid shrinking the pane. The recap agents view already shrinks on every window resize, and a component that only renders correctly at a fixed height is not embeddable.
2. Drop rows from the top on a shrink, by exactly the amount needed to keep the cursor on screen. Matches xterm and tmux, and is a change to one function.
3. Add scrollback so the dropped rows are recoverable, and scroll on resize the way a real terminal does. This is the complete behaviour, but scrollback is a separate feature with its own memory bound, API surface and scroll bindings.

## Decision Outcome

Chosen option: 2. `reshapeGrid` grows a row offset, and `screen.resize` computes it:

```go
// reshapeGrid returns a grid of the new geometry holding the overlap starting at
// srcTop in the old grid.
func reshapeGrid(old []glyph.Cell, oldRows, oldCols, rows, cols, srcTop int) []glyph.Cell
```

`screen.resize` computes an offset PER GRID, from that grid's own cursor, in three cases:

- Active grid: `srcTop = max(0, s.cy - (rows-1))` on a shrink and 0 otherwise; the cursor becomes `s.cy - srcTop` before the existing clamp.
- Inactive primary, while the alternate screen is active: the same expression over `altSavedCy`, which is decremented by its own offset so cursor and content stay together.
- Inactive alt, while the primary is active: no offset. `enterAlt` blanks the grid and homes the cursor on every entry (`term/vt.go:116-118`), so no alt cell survives a leave-enter round trip and there is no alt cursor saved anywhere to derive one from.

For that to hold on every path, `enterAlt` records the primary cursor into `altSavedCx/Cy` unconditionally. Today it saves only on the 1049 path (`term/vt.go:108`), so a screen entered through 47 or 1047 holds a stale value there; `saveCursor` continues to gate only the restore in `leaveAlt`.

Because:

* It is the behaviour every terminal a user has already learned, so the pinned UI of a full-screen program survives a resize and the visible content stays the content the program last painted.
* It is cheaper than option 3 and does not commit the component to a scrollback design. Rows above the cursor are still lost on a shrink; that limit is stated, and scrollback remains available later without revisiting this decision.
* The alternative in option 1 leaves the only in-tree consumer rendering a broken pane on every window resize.

## Technical

`reshapeGrid` is called only from `screen.resize`, so the signature change is local to `term/vt.go`.

A single offset shared by both grids is what misaligns the primary, not what prevents it. The offset derives from a cursor, and while the alternate screen is active the primary's cursor lives in `altSavedCy`, not `s.cy`. Scrolling the primary by the alt program's offset while leaving `altSavedCy` untouched leaves cursor and content diverged by exactly that offset the moment the program exits. Hence the per-grid rule above.

The scroll region continues to reset to the full screen on resize, which is what xterm does and what the capture's `CSI 2;91r` / `CSI r` idiom depends on.

## Risks

A program that repaints unconditionally after SIGWINCH sees no difference, since its next frame overwrites the grid. The change is only observable in the window between the resize and that repaint, and for programs that repaint incrementally. That window is precisely where the reported corruption lives.

## Non-goals

Scrollback. Rows above the cursor are still discarded on a shrink; recovering them is a separate feature with its own memory bound and scroll API.

Column reflow. Shrinking the width still truncates on the right rather than rewrapping, which is what tmux does without its reflow option and what the measurements above compared against.

The colon-form CSI parse defect (`ESC[38:2::r:g:b m` aborts and prints its tail), which is a separate parser bug with its own repro and no bearing on resize.

## Agreed todos

- [ ] A terminal pane that shrinks keeps its bottom-anchored content, matching xterm and tmux
- [ ] A tmux-oracle regression test replays a captured session across resizes and asserts the grids agree
- [ ] A program that leaves the alternate screen after a resize finds the primary's cursor still on the content it left

## End-goal state

`TermC` renders the same cells as tmux when its pane shrinks, growing and column changes are unchanged, and a regression test replays a real captured session through a resize and asserts zero divergence from a tmux reference grid at the same geometry. A resize taken while the alternate screen is active leaves the primary grid and its saved cursor consistent, asserted directly rather than left to the replay to reach.
