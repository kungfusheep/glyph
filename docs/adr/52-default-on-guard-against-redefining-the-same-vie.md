# ADR 52: default-on guard against redefining the same view instead of using control flow

- status: accepted
- date: 2026-06-26 18:23:40
- proposer: Glyph Smith@tui
- parties: tui
- deliberation: recap proposal show 52 (3 comments)

# Proposal: a default-on guard against redefining the same view instead of using control flow

## Problem

The most common way newcomers (and agents) misuse glyph is to **redefine the same
view** — duplicating a whole view, or rebuilding it, every time state changes —
instead of registering it **once** and letting render-time control flow
(`If().Then().Else()`, `Switch().Case()`) and pointer reactivity drive the changes.

This silently defeats the architecture:

- Compiling a view runs reflection over the whole tree and allocates a fresh
  `Template` (ops, geom, scratch buffers, bindings). It is the expensive compile
  step, meant to run **once** per view.
- Duplicating near-identical views (a `home`, a `homeError`, a `homeLoading` that
  are 90% the same tree) multiplies that cost and the maintenance surface, when one
  view + `If`/`Switch` on state would do.
- It's also a correctness trap: a standard `if` runs at build time, so data mutated
  afterward isn't reflected until a rebuild — which is exactly why the user reaches
  for a second view or a re-`SetView`, compounding the problem.

There's already a blunt, opt-in backstop on the anonymous path: `SetViewLimit(n)`
panics once `setViewCount` exceeds `n`. But it's off by default, it **panics**
rather than teaching, and it counts *every* `SetView` regardless of what was built —
so it can't tell "the same view redefined" (the real defect) from "a genuinely
different view shown."

## Who this is for

Every glyph consumer, but especially:

- **Agents authoring TUIs** — this is the #1 deficiency observed in agent-written
  glyph code. A one-line stderr nudge turns a silent perf/maintenance cliff into an
  immediate "one view + If/Switch" fix.
- **Newcomers** learning the control-flow model, who currently get no feedback that
  they've stepped off the happy path.

## What it does (API shape) — named views first

Lead with the **named-view registry** (`View(name, view)` / `Go` / `PushView`),
because that is where view identity is **explicit**: the name *is* the key, so we
don't have to infer "the same view" by hashing an anonymous tree. Two signals, both
evaluated at `View()` **registration** time (a startup/rare path — **zero per-frame
cost**), both behaviourally inert (one `os.Stderr` line, deduped, never per frame):

1. **Same name re-registered with a different structure.** `View("home", …)` called
   twice is a literal redefinition. `UpdateView(name, …)` is the *sanctioned*
   recompile path and is exempt by construction — only a second `View()` on a name
   already in `viewTemplates` warns:

   ```
   glyph: view "home" registered twice — register each view once and use
   If().Then()/Switch().Case() + pointer state for what changes (UpdateView if you
   truly need to recompile).
   ```

2. **Distinct names, identical structure** *(the high-value signal — it is the
   reported deficiency).* `View("home", …)` and `View("homeError", …)` that compile
   to the same structural fingerprint are the copy-paste-a-whole-view defect:

   ```
   glyph: views "home" and "homeError" are structurally identical — prefer one view
   with If().Then()/Switch().Case() + state over duplicate views.
   ```

Both reuse one cheap **structural fingerprint**: an FNV-1a hash over each op's
`(Kind, Depth, Parent)` — the tree *shape*, independent of the data/pointers it
carries. Signal 2 is low-noise precisely because named views are registered at
startup, not in a render loop.

Optional escape hatch for tests/embedders: `App.SetViewDiagnostic(false)`.

## Later additive layer (not this cut): the anonymous `SetView` path

The single-view `SetView` path has **no name**, so detecting "same view rebuilt in a
loop" there requires the fingerprint as a *heuristic* (count identical shapes,
warn past a threshold with dedup). That's strictly more speculative than the named
case, so it's deferred to a follow-up — named-first covers the reported deficiency
with explicit identity and no guessing. The existing `SetViewLimit` panic remains
the opt-in hard cap on that path in the meantime.

## Implementation sketch

- Add a structural fingerprint: `Template.structHash() uint64`, one pass over
  `tmpl.ops` folding `Kind`, `Depth`, `Parent`. O(ops), runs only at registration.
- In `View()` (app.go ~line 491, after `Build`): before `a.viewTemplates[name] =
  tmpl`, (a) if `name` already present → signal 1; (b) look the new hash up in a
  `map[uint64]string` of hash→firstName; if it maps to a *different* name → signal
  2; else record it. Keep a `seen` set so each message prints at most once.
- `UpdateView()` sets a "recompile expected" flag (or simply doesn't run the
  signal-1 check) so the sanctioned path stays silent.
- ~30 lines in `app.go` plus the hash helper. Only new public surface is the
  optional `SetViewDiagnostic` toggle.

## Risks / edge cases

- **Legitimately similar views.** Two views that share a shell but differ in a few
  ops won't hash-collide, so they won't false-fire; only *byte-for-byte structural*
  twins do, which is exactly the defect. Threshold isn't even needed for signal 2 —
  identical shape across two names is unambiguous.
- **Dynamic views built in a loop and registered under generated names.** If a
  program legitimately registers N structurally-identical views under distinct names
  (rare, e.g. a tiling grid of identical panes), signal 2 would fire once; the
  `SetViewDiagnostic(false)` toggle is the escape hatch, and the line is inert.
- **`UpdateView` vs `View`.** Covered above — `UpdateView` is the sanctioned
  recompile and is exempt; only a duplicate `View()` registration warns.
- **Channel.** stderr, once per finding, zero behavioural change — consistent with
  the existing unsized-component diagnostic and the diagnostics `screen.go`/
  `template.go` already write to stderr. Surfaces in `go test` output and a
  redirected dev terminal; never alters layout, render, or timing.

## Alternative considered

A `go vet` / static analyzer that flags duplicate view registration or `SetView` in
loops. Rejected for the first cut: separate tool, can't see runtime registration
that isn't lexically obvious, and runtime stderr diagnostics are the established
house style. Could be a later additive layer.
