# 001 — oscillators and self-gated animation

status: proposed — awaiting Pete's approval
consumers: mail (Ada, m108), all fleet apps via skill rule "tick only while animating"
related: recap inbox #270 (oscillator design), a3068bf (render generations)

## problem

The "animation ticks only while something animates" rule currently makes every app
re-implement a gated ticker. glyph already owns the right mechanism — the template
runs a 16ms ticker while `t.animating` and stops it when settled (template.go:5036)
— but only tween ops can set `animating`. Spinners need hand-fed frame counters from
app goroutines, and time-driven ScreenEffects need external tick windows (mail's
compose transition holds a 940ms app-level ticker).

## proposal

Three additions, in value order. All ride the existing `t.animating` gate; no new
scheduling machinery.

### 1. self-animating spinner

`Spinner()` with no frame pointer derives its frame from the frame clock:
`idx = int(elapsed * fps) % len(frames)`, default ~12fps, `.Fps(n)` to tune.
While it renders (branch active, not culled) it marks `animating`; hidden spinners
cost nothing. `Spinner(&frame)` keeps compiling via a variadic parameter (the
ForEach precedent — zero breakage). Deletes the skill's "increment frame yourself"
wart and the main reason apps hand-roll tickers.

### 2. oscillator value nodes

`Osc(hz)` — a value node accepted everywhere the property compiler accepts a
tweenNode (template.go compileDyn cases). Waveforms `.Sine()` (default)
`.Triangle()` `.Saw()` `.Square(duty)` `.Steps(n)`; mappings `.Range(min,max)`
`.Lerp(a,b)` `.Phase(p)`; `.Speed(&hz)` live-bound. Pure derivation from the
frame clock — value = f(now), no counters, no goroutines; marks `animating`
while resolved. Uses: LED breathing (Opacity), alert blink (Square), barber-pole
progress (Phase), marquee (Steps over offset), colour cycling (Lerp).

Phase 1 uses a single app epoch: oscillators are stateless and ForEach-safe by
construction; all spinners phase-lock. Restart-on-activation needs render-identity
keyed state — that is the existing "framework-owned runtime state" backlog item
(the spinGlowPhase shape) and is explicitly phase 2.

### 3. effects request animation

`ctx.RequestAnimation()` on the effect context: a time-driven ScreenEffect
mid-animation marks the template animating for the next frame. Replaces mail's
external 940ms tick window for the compose transition. Effects already receive
ctx.Time (postprocesseffects.go:406); this closes their half of the loop.

### declined (for now): app.HoldAnimation()

App-domain time windows (notification TTL) are events, not animations: schedule a
time.AfterFunc that mutates state and pushes. With 1+2+3 the remaining need is
near zero; adding a manual hold reopens the always-on-ticker door.

## implementation sketch

- one clock: `Template.frameTime` stamped once per Execute from a monotonic epoch;
  injectable (`func() time.Time`) for deterministic tests; shared with effects'
  ctx.Time so template and effect motion phase-lock.
- oscNode compiled like tweenNode into pre-allocated storage; resolving marks a
  frame-active flag that feeds `t.animating` exactly as tweens do today.
- zero allocations per frame; math on int64 nanos.

## risks

- the `animating` gate currently latches per-template; oscillators inside retained
  If branches must not keep it held when the branch exits (same lifecycle tweens
  already handle at template.go:1505-1875 — reuse that path).
- global epoch means a newly-shown spinner starts mid-phase; visible only with
  asymmetric frame sequences; accepted for phase 1.
- touches Execute's tail next to the new render-generation logic (a3068bf);
  the protocol test plus a gated-spinner test must both stay green.

## phase-2 notes (out of scope, recorded)

- restart-phase-on-activation via render-identity keyed state (the spinGlowPhase shape).
- per-item mount/unmount animation inside ForEach (raised by mail): item enter/exit
  is branch-retention territory — likely rides the existing retained-branch tween
  lifecycle once runtime state is keyed by render identity. Adoption detail for the
  notification feed; not a blocker for 001.

## migration

mail deletes its app-level animator and both standing tickers; demos drop frame
counter goroutines; skill examples and pitfall rows updated; Animate unchanged
(one-shot = tween, periodic = oscillator, same clock).
