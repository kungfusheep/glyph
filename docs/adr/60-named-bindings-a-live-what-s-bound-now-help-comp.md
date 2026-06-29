# ADR 60: named bindings + a live what's-bound-now help component

- status: accepted
- date: 2026-06-29 13:11:17
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 60 (1 comments)

# Proposal: named bindings + a live "what's bound now" help component

## Problem

There's no way to show a user the keys available **right now**. A static cheatsheet
drifts from the code and can't reflect context (which view/modal/focus is active).
We want a help component that lists the *currently bound* keys, live — and, as a
bonus, the ability to let users rebind keys via a config file.

## What already exists (the happy surprise)

riffkey already has the entire engine; glyph just throws it away. On `riffkey.Router`:

- `HandleNamed(name, defaultPattern, h)` — "registers a handler with a semantic name
  for **introspection and rebinding**."
- `Bindings() []Binding` where `Binding{Name, Pattern, DefaultPattern}` — the live
  list, in registration order, with the current (post-rebind) pattern.
- `Rebind(name, pattern)`, `Reset(name)`, `ResetAll()` — runtime rebinding.
- `ApplyBindings(map)`, `LoadBindings(appName)`, `LoadBindingsFrom(path, appName)`,
  `WriteDefaultBindings(w, appName)` — config-file support, already built.

glyph's `binding{pattern, handler}` and `Key(pattern, handler)` wire through plain
`router.Handle(pattern, h)`, which registers **no name** — so `Bindings()` is empty
and none of the above is reachable. The whole feature is a thin glyph delta over
machinery riffkey already ships.

## The delta (glyph side)

Three small pieces, all additive and backward-compatible:

1. **A name on a binding.** Add an optional `name` to glyph's `binding` struct and a
   builder on the existing leaf:

   ```go
   On(Key("j", scrollDown).Named("scroll-down"))   // named → introspectable + rebindable
   On(Key("q", quit))                              // unnamed → works exactly as today
   ```

   `Key(...).Named(n)` keeps the value-receiver chaining glyph leaves already use.
   (Alternative spelling `NamedKey(name, pattern, handler)` if you prefer a distinct
   constructor — same wiring; I lean on the `.Named()` chain so the pattern stays the
   visible default.)

2. **Wire named bindings through `HandleNamed`.** In `wireBinding`/
   `wireComponentBindings` (app.go), when `b.name != ""` call
   `router.HandleNamed(b.name, b.pattern, h)` instead of `router.Handle(b.pattern, h)`.
   That single switch unlocks `Bindings()`, `Rebind`, and config load — for free.

3. **A live help component.** `KeyHelp()` reads the active router's `Bindings()` each
   frame and renders `pattern → name` rows. Because it reads the live router, it always
   shows what's bound *now* — switch view/modal/focus and it updates with no extra
   wiring. It needs one app accessor:

   ```go
   func (a *App) ActiveBindings() []riffkey.Binding { return a.input.Current().Bindings() }
   ```

   The component is glyph-idiomatic: a `ForEach` over `ActiveBindings()` inside a box,
   re-evaluated per frame (no snapshot, no drift). Names are the human label; a
   `humanise(name)` ("scroll-down" → "Scroll down") keeps it readable, or names can be
   written display-ready.

## Config-file rebinding (falls out for free)

Expose a hook so an app can load user overrides at startup:

```go
func (a *App) LoadKeyBindings(appName string) error // wraps riffkey LoadBindings on the app routers
```

riffkey's `LoadBindings(appName)` reads the standard config path and `ApplyBindings`
maps `name → pattern`; `WriteDefaultBindings` can emit a starter file. Only **named**
bindings participate — naming is the opt-in that makes a key both discoverable in help
and user-rebindable. Nice incentive alignment: you name what you want to expose.

## Open questions for review

- **Scope of "right now":** v1 reads `input.Current()` (the active top frame) — covers
  the common case (current view / modal / focused field). The *full* union across the
  router stack would need a small riffkey accessor to enumerate frames; defer to a
  riffkey follow-up unless you want it in v1.
- **Name vs. description:** is the semantic name (`scroll-down`) enough as the help
  label (humanised), or do we want a separate free-text `desc`? Lean: name only,
  humanised — one field, no redundancy; add `desc` later only if a name can't carry it.
- **Help component API surface:** `KeyHelp()` as a plain component (you place it), vs a
  built-in overlay toggled by a standard key. Lean: ship the component; let apps wire
  the toggle (a `?` overlay is then trivial and stays the app's choice).

## Risks / notes

- Backward-compatible: unnamed `Key(...)` is untouched (still `Handle`), so nothing
  existing changes behaviour. Only newly-named bindings flow through `HandleNamed`.
- No per-frame cost concern: `Bindings()` is read only by the help component when it's
  actually rendered (rare, a help pane), not on the hot path.
- Day-one binding rule applies: the new `name` field on `binding` gets a test asserting
  a named binding round-trips to `ActiveBindings()` with the right pattern, and that an
  unnamed binding still routes.
