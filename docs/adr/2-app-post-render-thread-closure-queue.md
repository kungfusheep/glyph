# ADR 2: app.Post: render-thread closure queue

- status: accepted
- date: 2026-06-12 23:07:04
- proposer: Glyph Smith@tui
- parties: calendar, mail, recap, tui
- deliberation: recap proposal show 2 (20 comments)

# app.Apply — render-thread closure queue

consumers: recap (three staged pairs delete), calendar (enqueue/drainPending
deletes, ~20 lines), mail (three pending flags delete; OnBeforeRender ends
EMPTY) — all three formally endorsed with concrete migration walks
related: the no-per-frame-work ruling (#239 thread), render generations
(a3068bf); name decided unanimously in deliberation (pc36-pc43)

## problem

glyph's documented mutation model — "mutate the pointed-to value, then
RequestRender from goroutines" — is unsound for anything bigger than a word:
OnBeforeRender is the only user code that runs under the render lock, so a
goroutine writing bound slices/structs races an in-flight Execute. Every
fleet app hand-rolled the same answer: stage the result, drain the staging at
the frame seam, swap under the lock. The staging layer exists only because
nobody has a safe place to run the apply.

## proposal

    app.Apply(fn func())

Enqueue a closure; the render loop drains the queue inside render(), under
renderMu, BEFORE OnBeforeRender and before any reads. Applying coalesces with
RequestRender (one frame per batch). Closures run in apply order, exactly
once, on the render thread; they mutate bound state and return — never
render, never block. An applied closure MAY spawn goroutines (the
chained-apply pattern: adopt -> kick async build -> terminal Apply); Apply
names the intent, not a pure-setter restriction.

Naming note (deliberated): Post was rejected for the in-API collision with
PostContext (same prefix, opposite end of the frame); Apply matches the
vocabulary the fleet already converged on (applyInbox/applyDetail/
applyResize, the skill's staged-apply seam).

Loader pattern becomes:

    go func() {
        result := fetch(...)
        app.Apply(func() {
            if key == current { apply(result) }
        })
    }()

No staging structs, no take/swap pairs, no frame-hook residue.

## semantics (deliberation-hardened)

- a closure applied DURING the drain runs NEXT frame, not this one: the drain
  swaps the batch out at frame top and runs only that batch. This is the
  anti-livelock invariant (a self-applying chain cannot wedge a frame) and
  calendar's reload chains depend on it. Do not optimise into drain-until-empty.
- closures must not call RenderNow (renderMu deadlock) — same documented
  constraint as OnResize callbacks; doc + guard test.
- unbounded queue growth under producer flood is accepted (any channel has
  the same property); Apply is for applies, not streams.

## implementation sketch

slice + mutex on App (applyMu, applied []func()); render() swaps the slice
out under the lock at frame top, runs each; Apply appends + RequestRender.
Zero allocation steady-state via double-buffered slices. Guard test shape:
calendar's TestQueuedReloadsDrainBeforeRender — goroutines hammering Apply
against concurrent RenderNow, asserting order and exactly-once.

## migration

recap deletes stage/take pairs (OnBeforeRender ends empty but for the seam);
calendar's enqueue/drainPending becomes Apply (pure deletion); mail's three
pending flags dissolve (OnBeforeRender ends EMPTY — no seam call at all).
Skill's staged-swap section updates to Apply as the primary apply point with
staging as the pre-Apply historical note.
