# ADR 48: OnInput: input-timing hook for idle/activity detection

- status: accepted
- date: 2026-06-26 15:44:59
- proposer: Glyph Smith@tui
- parties: tui, recap, Glyph
- deliberation: recap proposal show 48 (4 comments)

# OnInput: an input-timing hook for idle / activity detection

## Context and Problem Statement

A consumer needs to know WHEN the human last interacted, to detect idle: emit an
idle-start after N minutes with no input and an idle-end on the next input, so
"active" spans reflect real interaction rather than a pane left open. There is no
glyph signal for this today.

glyph exposes render hooks — `OnBeforeRender` / `OnAfterRender` — but renders are the
wrong signal: they also fire on background data reloads, animation ticks, and async
`RequestRender`s, so input cannot be inferred from them. And a consumer's key handling
is typically spread across many per-view `On(...)` bindings, with no central place to
timestamp "last keystroke."

glyph DOES have the central chokepoint internally: every key event passes through the
one callback the app registers with the input loop (`app.input.Run(reader, func(handled
bool){...})`, app.go). It just isn't surfaced.

## Considered Options

1. **Status quo** — consumers cannot detect input timing; idle telemetry is impossible
   or relies on hacks (inferring from renders, which is wrong).
2. **`OnInput(func())`** — a hook fired once per key event at the central input
   callback, symmetric with `OnBeforeRender`/`OnAfterRender`.
3. **`LastInputUnixNano() int64`** — the app stamps the last-input time; the consumer
   polls it on a timer.

## Decision Outcome

Proposed: **Option 3, `LastInputUnixNano() int64`** (revised from OnInput after consumer
review). The app stamps the last-input time on the central input read; the consumer polls
it on a short timer for idle detection. Because:

- **Smaller permanent surface.** A single getter vs a registered callback. glyph keeps
  the public API minimal; a getter is the least it can expose to meet the need.
- **Input-path purity.** Nothing consumer-supplied runs on the input goroutine — glyph
  stamps an atomic internally; the consumer's work happens off the input path, on its own
  timer. A registered callback, however cheap, puts consumer code on the key-dispatch
  path; the getter avoids that entirely.
- **The lost precision is the part that doesn't matter.** OnInput's one advantage was a
  precise idle-END on the exact keystroke. But idle-START — the span boundary that
  decides active-vs-away honesty — is precise either way (the consumer's timer fires at
  `lastInput + N`). idle-END just rounds to the short poll interval (a few seconds slack
  on the away→active edge), which is acceptable for the feature.

The consumer's idle logic: a timer checks `now - LastInputUnixNano()`; crossing N is
idle-start; a later poll seeing a newer stamp is idle-end.

Considered and set aside: **`OnInput(func())`** — symmetric with `OnBeforeRender`/
`OnAfterRender`, the general primitive (the getter is derivable from it), and event-driven
(precise idle-end). It loses on surface size and input-path purity, and its one unique
benefit (precise idle-end) isn't needed here. It can be added later if a consumer
genuinely needs per-key events; the getter does not preclude it.

## Technical

- App gains an internal `lastInputNs atomic.Int64` and a getter
  `LastInputUnixNano() int64`.
- The app's input callback (app.go ~1377) stamps `a.lastInputNs.Store(time.Now().UnixNano())`
  once at the top, before the existing render-pacing logic — the one central place every
  key event passes. Atomic, so the getter reads it lock-free from the consumer's timer
  goroutine.
- No consumer code on the input path; the stamp is a single atomic store. (Kept for
  reference — the earlier OnInput shape would have run on the INPUT goroutine and needed
  the cheap-callback discipline of the render hooks; the getter sidesteps that:
  non-issue in practice.)
- Leaf-level, additive, no behaviour change: it stamps a value the input loop already
  reaches; nothing reads it unless a consumer calls the getter.

## Risks

- Idle-END rounds to the consumer's poll interval (a few seconds slack on away→active).
  Accepted: idle-START, the honesty-critical boundary, stays precise (timer at
  `lastInput + N`).
- A torn read of the int64 on a 32-bit platform — avoided by `atomic.Int64`.

## Testing

- the stamp advances on a dispatched key event (drive a synthetic key through the input
  loop; assert `LastInputUnixNano()` increased).
- a render with no input does NOT advance the stamp (it tracks input, not frames —
  the whole point, since renders fire on background reloads).

## Migration

Additive. Existing apps are unaffected; a consumer opts in by polling
`app.LastInputUnixNano()`.
