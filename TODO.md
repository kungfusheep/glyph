## Priority Toolkit Features

1. [x] Inline mode - same components work inline (progress bars, menus, spinners)
2. [x] Jump labels - vim-easymotion style quick selection
3. [ ] Pre-composed components - "just works" components (StatusPanel, Gauge, etc.)
4. [ ] Help from metadata - auto-generate help screens from handler metadata
5. [ ] Overlays/modals - popups, dialogs, floating windows
6. [ ] Workbench shell - VSCode-style layout pattern (sidebar, panels, etc.)

we need a nice solution to the pointer/value binding issue

media query / width zoning

post-processing / 

## Other TODOs

- [ ] ForEach render function gotcha: Go-level if/switch is evaluated once at compile time against a dummy element — users can silently get frozen branches. Two mitigations:
  1. Docs: warn clearly in concepts/getting started, show the anti-pattern vs declarative If()/Switch()
  2. Runtime detection: call the render function twice against different dummies, compare resulting template structure. If they differ, panic with a clear message ("ForEach render function returned different structures — use If()/Switch() for per-item conditions")
- [ ] Optimise flex code to only run when we have flex children
- [ ] Locale-aware number formatting for AutoTable (e.g. `1.234,56` for European locales). Currently hardcoded to English comma separators.
- [ ] Serialisable views: serialise template tree for dev tooling — live inspector, visual debugger, hot style tweaking, record/replay for test snapshots. Connect to a running app over a socket, inspect the Op tree, tweak and push back.



split structure of api docs ino primitive/utility/higher order 
