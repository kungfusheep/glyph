# ADR 19: Skip Execute on effect-only frames (cache-effects render path)

- status: accepted
- date: 2026-06-17 16:31:07
- proposer: Glyph Smith@tui
- parties: tui, code_review
- deliberation: recap proposal show 19 (17 comments)

# Skip Execute on effect-only frames (cache-effects render path)

- prototype: branch `experiment/cache-effects` (equivalence + skip-safety tests pass)

## Problem

When a screen effect animates (vignette pulse, plasma, tint sweep) over app
output that hasn't changed, glyph re-runs the FULL render every frame: `Execute`
(layout + render the whole template) → `resolveColor16` → effect `Apply`. On an
effect-only frame the template output is identical to last frame — `Execute`
recomputes a buffer we already have. The effect is the only thing that changed.

Beyond the saving, decoupling effect animation from `Execute` unlocks moving
whole visual pipelines into the effects chain (e.g. green-screen / keying
techniques) that run over a cached app frame.

## Measured cost (measure-don't-assume)

Representative scene (120x40, bordered VBox, 30 rows of text + progress bars),
plasma tint effect, per frame:

| phase           | cost      |
|-----------------|-----------|
| Execute         | ~20.7 µs  |
| resolveColor16  | ~3.0 µs   |
| effect Apply    | ~37.8 µs  |
| **full frame**  | **~75 µs**|

Execute+resolve are ~31% of frame cost here (more on heavier templates), all
recomputing unchanged output on an effect-only frame.

## v1: the skip gate

- **Persistent clean buffer** `Execute` renders into. Effects move from
  in-place-before-copy to running on the screen BACK buffer AFTER `copyToScreen`
  (the copy that already happens), so the clean buffer stays pristine for reuse —
  no second copy. Proven byte-identical to today on the prototype branch.
- **appDirty flag** (default true): `RequestRender` sets it — so input, `Apply`,
  resize, view-change AND template animations (oscillators/spinners/tweens, which
  drive frames via `t.requestRender = a.RequestRender`) all force a full frame.
  The effect's own animation re-request uses a SEPARATE non-dirtying path.
- **Skip `Execute` only when** `!appDirty && effectsActive && clean valid &&
  prior-Execute !Animating()`. Conservative — never optimistic; any uncertainty
  falls through to a full render.
- **reuse-last-frame** (free): on a skipped frame the eval pointers retain their
  last-full-frame values, so static / app-event-driven params are correct with
  zero work.

Measured win (prototype): ~75 µs → ~47 µs per effect-only frame (**~37.5%**), and
5 → 2 allocs. The common (non-effect) path is byte-unchanged.

## Effects-system impact (all 16 built-ins + custom contract)

- **BUFFER-ONLY** (Dim, GradientMap, ScreenShake, Quantize, EachCell, most custom
  funcEffects): unaffected — byte-identical buffer, `ctx.Time/Frame/Delta` still
  advance on skipped frames.
- **NODEREF** (FocusDim + focus/dodge rects in Tint/Vignette/Glow/DropShadow/
  SpinGlow/Bloom): safe — NodeRef X/Y/W/H are written during Execute's render
  phase, so stale on a skipped frame, but the skip precondition is
  layout-unchanged, so stale == correct.
- **SELF-ACCUMULATING** (SESpinGlow): advances its phase off `ctx.Delta` in
  `Apply`, independent of Execute — already animates on skipped frames. The model
  for effect-driven animation.
- **DYNAMIC-PARAM** (`.Strength(Animate(...))` etc, built-in AND custom via
  `EffectCompiler.Float64`): the one impact — the value is updated by an eval that
  runs at Execute start, so skipping Execute freezes a clock-driven param.

### v1 handling of dynamic params

reuse-last-frame is correct for static / app-event params. For a clock-driven
effect param, last-frame's value is the previous tick (a freeze), so v1's safe
rule: if a clock-bound effect param is armed, keep `appDirty` true (full frame) —
correct, just forgoes the optimization for that effect. v2 removes that limit.

## v2: effect-driven animation (the green-screen enabler)

Tick effect-param animations in the EFFECT pass, decoupled from Execute:

1. **Seam**: effects compile at one point (`template.go` CompileEffect/
   compileEffect). Snapshot `len(root.evals)` around each effect's compile and
   MOVE the delta into a new `root.effectEvals` list — no churn to the ~20
   eval-append sites.
2. **Run `effectEvals` in the effect pass** (after copyToScreen, before Apply) on
   BOTH full and skipped frames — identical effect animation either way. Set
   `frameTime` first so oscillators/tweens advance.
3. **Continuity**: an animating effect eval sets `root.animating`; after ticking,
   if animating, request another frame via the effect-frame (non-appDirty) path —
   self-sustaining with Execute skipped.
4. **Shared-animation safety (automatic)**: an Osc driving both content and an
   effect param compiles its CONTENT eval into `root.evals` → sets animating →
   full frame. Only PURELY effect-bound animations land in `effectEvals`. Split is
   by where the value is consumed; no manual classification.
5. **Edge cases**: an effect param bound to app state (not a clock) is unchanged
   on a skipped frame (stale == correct). Imperative `AddEffect` gets the same
   snapshot treatment, or conservatively does not skip.

Unlocks plasma backgrounds, vignette pulses, tint sweeps, green-screen keying
animating at full frame rate while the main render is cached.

## Acceptance tests (required)

- **Animation guard (Komorebi)**: an animating template (Spinner / Osc-bound
  value) UNDER an active effect — animated cells ADVANCE frame-to-frame (appDirty
  fired, Execute NOT skipped). [Signal validated: Spinner reports `Animating()`
  and advances ⠋→⠦.]
- **v2 guard**: an oscillator-driven effect param ADVANCES across frames with
  Execute provably NOT called, while template content is byte-identical.
- **Equivalence + skip-safety** (already passing on the prototype branch):
  effects-on-back == effects-in-place (byte-identical), and an effect-only frame
  matches a full re-Execute+effect of the same state.
- **Benchmark**: per-frame cost full vs effect-only; common path unchanged; clip/
  skip paths allocation-free.

## Plan

Build v1 (skip gate + reuse-last-frame + appDirty-if-armed), with `effectEvals`
structured so v2 (move ticks into the effect pass) slots in as a follow-on.

## Risks

- `render()` is the highest-risk function (inline mode, jump, overlays, opacity,
  cursor, layer cursor all flow through it). Land behind the conservative gate so
  the default path is byte-unchanged; the prototype validates the mechanism in
  isolation.
- Missed animation source that doesn't funnel through `RequestRender` → stale
  frame. Mitigated by the conservative gate + the `Animating()` backstop + the
  animation guard test.

## Out of scope

- Multi-effect ordering changes, new effect APIs, the inline-mode flush path. This
  is purely WHEN Execute runs and WHERE effects apply, plus the effectEvals split.
