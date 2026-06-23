package glyph

import (
	"strings"
	"testing"

	"github.com/kungfusheep/riffkey"
)

func dispatchRune(app *App, r rune) bool {
	return app.Input().Dispatch(riffkey.Key{Rune: r})
}

func dispatchKey(app *App, name string) bool {
	switch name {
	case "<Escape>":
		return app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEscape})
	}
	return false
}

func TestOnRegistersViewScopedHandler(t *testing.T) {
	app := NewApp()
	hits := 0

	app.SetView(VBox(
		Text("ready"),
		On(Key("x", func() { hits++ })),
	))

	if !dispatchRune(app, 'x') {
		t.Fatal("expected On handler to handle x")
	}
	if hits != 1 {
		t.Fatalf("expected one hit, got %d", hits)
	}
}

func TestOnInsideIfTogglesWithBranch(t *testing.T) {
	app := NewApp()
	active := true
	thenHits := 0
	elseHits := 0

	app.SetView(VBox(
		If(&active).
			Then(On(Key("j", func() { thenHits++ }))).
			Else(On(Key("j", func() { elseHits++ }))),
	))

	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected active then branch handler to handle j")
	}
	if thenHits != 1 || elseHits != 0 {
		t.Fatalf("then branch mismatch: then=%d else=%d", thenHits, elseHits)
	}

	active = false
	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected active else branch handler to handle j")
	}
	if thenHits != 1 || elseHits != 1 {
		t.Fatalf("else branch mismatch: then=%d else=%d", thenHits, elseHits)
	}
}

func TestOnInsideInactiveIfDoesNotShadowRoot(t *testing.T) {
	app := NewApp()
	active := false
	rootHits := 0
	branchHits := 0

	app.SetView(VBox(
		On(Key("j", func() { rootHits++ })),
		If(&active).Then(
			On(Key("j", func() { branchHits++ })),
		),
	))

	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected root handler to handle j")
	}
	if rootHits != 1 || branchHits != 0 {
		t.Fatalf("inactive branch shadowed root: root=%d branch=%d", rootHits, branchHits)
	}

	active = true
	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected active branch handler to handle j")
	}
	if rootHits != 1 || branchHits != 1 {
		t.Fatalf("active branch did not shadow root: root=%d branch=%d", rootHits, branchHits)
	}
}

func TestOnInsideSwitchTogglesWithCase(t *testing.T) {
	app := NewApp()
	pane := "files"
	filesHits := 0
	previewHits := 0

	app.SetView(VBox(
		Switch(&pane).
			Case("files", On(Key("j", func() { filesHits++ }))).
			Case("preview", On(Key("j", func() { previewHits++ }))).
			End(),
	))

	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected files case handler to handle j")
	}
	if filesHits != 1 || previewHits != 0 {
		t.Fatalf("files case mismatch: files=%d preview=%d", filesHits, previewHits)
	}

	pane = "preview"
	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected preview case handler to handle j")
	}
	if filesHits != 1 || previewHits != 1 {
		t.Fatalf("preview case mismatch: files=%d preview=%d", filesHits, previewHits)
	}
}

func TestOnGroupsMultipleKeysInOneConditionalScope(t *testing.T) {
	app := NewApp()
	active := true
	downHits := 0
	upHits := 0

	app.SetView(VBox(
		If(&active).Then(On(
			Key("j", func() { downHits++ }),
			Key("k", func() { upHits++ }),
		)),
	))

	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("expected active branch handler to handle j")
	}
	if !dispatchRune(app, 'k') {
		t.Fatal("expected active branch handler to handle k")
	}
	if downHits != 1 || upHits != 1 {
		t.Fatalf("expected both handlers to fire once, got down=%d up=%d", downHits, upHits)
	}

	active = false
	app.render()
	if dispatchRune(app, 'j') || dispatchRune(app, 'k') {
		t.Fatal("expected inactive On scope not to handle keys")
	}
	if downHits != 1 || upHits != 1 {
		t.Fatalf("inactive scope fired handlers: down=%d up=%d", downHits, upHits)
	}
}

func TestOnModalPushesWhileActiveAndShadowsRoot(t *testing.T) {
	app := NewApp()
	active := false
	rootHits := 0
	modalHits := 0

	app.SetView(VBox(
		On(Key("j", func() { rootHits++ })),
		If(&active).Then(On.Modal(
			Key("j", func() { modalHits++ }),
		)),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("inactive modal changed input depth: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'j') {
		t.Fatal("expected root handler to handle j")
	}
	if rootHits != 1 || modalHits != 0 {
		t.Fatalf("inactive modal mismatch: root=%d modal=%d", rootHits, modalHits)
	}

	active = true
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("active modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'j') {
		t.Fatal("expected modal handler to handle j")
	}
	if rootHits != 1 || modalHits != 1 {
		t.Fatalf("modal did not shadow root: root=%d modal=%d", rootHits, modalHits)
	}

	active = false
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("deactivated modal should pop input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'j') {
		t.Fatal("expected root handler to resume after modal closes")
	}
	if rootHits != 2 || modalHits != 1 {
		t.Fatalf("root did not resume after modal close: root=%d modal=%d", rootHits, modalHits)
	}
}

func TestOnModalWorksInsideOverlay(t *testing.T) {
	app := NewApp()
	showOverlay := false
	rootHits := 0
	modalHits := 0

	app.SetView(VBox(
		On(Key("j", func() { rootHits++ })),
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					Text("modal"),
					On.Modal(Key("j", func() { modalHits++ })),
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hidden overlay changed input depth: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'j') {
		t.Fatal("expected root handler to handle j while overlay hidden")
	}

	showOverlay = true
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("overlay modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'j') {
		t.Fatal("expected overlay modal handler to handle j")
	}
	if rootHits != 1 || modalHits != 1 {
		t.Fatalf("overlay modal mismatch: root=%d modal=%d", rootHits, modalHits)
	}

	showOverlay = false
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hidden overlay modal should pop input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
}

func TestOnModalDoesNotRequireOverlay(t *testing.T) {
	app := NewApp()
	active := true
	hits := 0

	app.SetView(VBox(
		If(&active).Then(On.Modal(
			Key("x", func() { hits++ }),
		)),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("non-overlay modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'x') {
		t.Fatal("expected non-overlay modal handler to handle x")
	}
	if hits != 1 {
		t.Fatalf("expected one hit, got %d", hits)
	}
}

func TestOnModalRoutesSiblingInputBinding(t *testing.T) {
	app := NewApp()
	showOverlay := false
	input := Input().Bind()

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					On.Modal(Key("<Esc>", func() { showOverlay = false })),
					input,
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hidden overlay changed input depth: %d -> %d", baseDepth, app.Input().Depth())
	}
	if dispatchRune(app, 'a') {
		t.Fatal("hidden modal input handled key")
	}
	if input.Value() != "" {
		t.Fatalf("hidden modal input changed value: %q", input.Value())
	}

	showOverlay = true
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("overlay modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'a') {
		t.Fatal("expected modal input to handle typed rune")
	}
	if input.Value() != "a" {
		t.Fatalf("expected modal input value %q, got %q", "a", input.Value())
	}

	showOverlay = false
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hidden overlay modal should pop input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
}

func TestOnModalRoutesSiblingFilterListBinding(t *testing.T) {
	app := NewApp()
	showOverlay := false
	items := []string{"compose", "reply", "refresh"}
	filter := FilterList(&items, func(s *string) string { return *s })

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					On.Modal(Key("<Esc>", func() { showOverlay = false })),
					filter,
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if dispatchRune(app, 'r') {
		t.Fatal("hidden modal filter list handled key")
	}
	if filter.Query() != "" {
		t.Fatalf("hidden modal filter query changed: %q", filter.Query())
	}

	showOverlay = true
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("overlay modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'r') {
		t.Fatal("expected modal filter list to handle typed rune")
	}
	if filter.Query() != "r" {
		t.Fatalf("expected modal filter query %q, got %q", "r", filter.Query())
	}
	if filter.Filter().Len() != 2 {
		t.Fatalf("expected two filtered items, got %d", filter.Filter().Len())
	}
}

func TestOnModalNestedConditionRoutesSiblingFilterListBinding(t *testing.T) {
	app := NewApp()
	showOverlay := false
	items := []string{"compose", "reply", "refresh"}
	filter := FilterList(&items, func(s *string) string { return *s })

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					If(&showOverlay).Then(
						On.Modal(Key("<Esc>", func() { showOverlay = false })),
					),
					filter,
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if dispatchRune(app, 'r') {
		t.Fatal("hidden modal filter list handled key")
	}
	if filter.Query() != "" {
		t.Fatalf("hidden modal filter query changed: %q", filter.Query())
	}

	showOverlay = true
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("overlay modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'r') {
		t.Fatal("expected modal filter list to handle typed rune")
	}
	if filter.Query() != "r" {
		t.Fatalf("expected modal filter query %q, got %q", "r", filter.Query())
	}

	showOverlay = false
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hidden overlay modal should pop input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
}

func TestOnModalSwitchesFromElseBranchToThenBranch(t *testing.T) {
	app := NewApp()
	confirming := false
	normalHits := 0
	confirmHits := 0

	app.SetView(VBox(
		If(&confirming).
			Then(On.Modal(Key("y", func() { confirmHits++ }))).
			Else(On.Modal(Key("d", func() { normalHits++; confirming = true }))),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("else modal should push one input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'd') {
		t.Fatal("expected else branch handler to handle d")
	}
	if normalHits != 1 || confirmHits != 0 {
		t.Fatalf("else branch mismatch: normal=%d confirm=%d", normalHits, confirmHits)
	}

	app.render()
	if app.Input().Depth() != baseDepth+1 {
		t.Fatalf("branch swap should keep one modal input frame: %d -> %d", baseDepth, app.Input().Depth())
	}
	if !dispatchRune(app, 'y') {
		t.Fatal("expected then branch handler to handle y after branch swap")
	}
	if normalHits != 1 || confirmHits != 1 {
		t.Fatalf("then branch mismatch: normal=%d confirm=%d", normalHits, confirmHits)
	}
}

// the calendar m298 handoff: a Form (FocusManager-managed Input) inside an
// If+Overlay must get keyboard focus activated when the overlay shows, and
// release it symmetrically when it hides — the FM rides the same visibility
// edges as the modal router.
func TestOnModalRoutesFormInputBinding(t *testing.T) {
	app := NewApp()
	showOverlay := false
	var title string

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					On.Modal(Key("<Esc>", func() { showOverlay = false })),
					Form(
						Field("title", Input(&title).Bind()),
					),
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	app.render()
	if dispatchRune(app, 'a') {
		t.Fatal("hidden modal form handled key")
	}
	if title != "" {
		t.Fatalf("hidden form input changed value: %q", title)
	}

	showOverlay = true
	app.render()
	if !dispatchRune(app, 'a') {
		t.Fatal("expected focused form input to handle typed rune")
	}
	if title != "a" {
		t.Fatalf("expected form input value %q, got %q", "a", title)
	}

	showOverlay = false
	app.render()
	if app.Input().Depth() != baseDepth {
		t.Fatalf("hiding the overlay must unwind both FM and modal frames: %d -> %d", baseDepth, app.Input().Depth())
	}
	if dispatchRune(app, 'b') {
		t.Fatal("hidden form input still handling keys after close")
	}
	if title != "a" {
		t.Fatalf("hidden form input mutated after close: %q", title)
	}
}

// the m305 follow-up: Escape must be able to dismiss a form-in-overlay. The
// FM pushes on the show EDGE only — the user's blur (first Escape) stays
// blurred instead of being re-pushed next frame, so the second Escape falls
// through to the modal router and closes the overlay.
func TestOnModalFormEscapeDismissesOverlay(t *testing.T) {
	app := NewApp()
	showOverlay := false
	var title string

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					On.Modal(Key("<Escape>", func() { showOverlay = false })),
					Form(
						Field("title", Input(&title).Bind()),
					),
				),
			),
		),
	))

	baseDepth := app.Input().Depth()
	showOverlay = true
	app.render()
	if !dispatchRune(app, 'a') || title != "a" {
		t.Fatalf("focused form input not receiving keys; title=%q", title)
	}

	dispatchKey(app, "<Escape>") // blur the field
	app.render()                 // a frame passes; blur must persist
	if dispatchRune(app, 'b') && title == "ab" {
		t.Fatal("input still focused after blur+render; FM re-pushed on visible frame")
	}

	dispatchKey(app, "<Escape>") // now reaches the modal router
	app.render()
	if showOverlay {
		t.Fatal("second Escape did not dismiss the overlay")
	}
	if app.Input().Depth() != baseDepth {
		t.Fatalf("depth not restored after dismiss: %d -> %d", baseDepth, app.Input().Depth())
	}

	// reopening restores focus from the top
	showOverlay = true
	app.render()
	if !dispatchRune(app, 'x') {
		t.Fatal("reopened form input not focused; show-edge latch failed to reset")
	}
}

// TestModalRouterUpBeforeNextKeyInRealLoop drives keys through riffkey's REAL
// loop (ReadKey -> Dispatch -> afterDispatch render), not a bare Dispatch. A key
// opens an overlay modal via a state flip + async RequestRender (no synchronous
// RenderNow), then the next key must reach the modal's On.Modal handler — proving
// the opening key's render pushes the modal router before the next key is read,
// so there is no dropped-first-key race in the real input loop (the race only
// appears when keys are dispatched with no render between them).
func TestModalRouterUpBeforeNextKeyInRealLoop(t *testing.T) {
	app := NewApp()
	modalOpen := false
	rootHits, modalHits := 0, 0
	app.SetView(VBox(
		On(Key("x", func() { modalOpen = true; app.RequestRender() })),
		On(Key("y", func() { rootHits++ })),
		If(&modalOpen).Then(Overlay.Centered()(VBox(
			Text("modal"),
			On.Modal(Key("y", func() { modalHits++ })),
		))),
	))
	app.RenderNow()

	reader := riffkey.NewReader(strings.NewReader("xy"))
	_ = app.Input().Run(reader, func(handled bool) { app.RenderNow() })

	if modalHits != 1 || rootHits != 0 {
		t.Fatalf("real loop: the modal should catch the y after x opened it (no RenderNow needed): root=%d modal=%d", rootHits, modalHits)
	}
}

// authoritative answer to Komorebi's question (recap #451): does a List's
// .BindVimNav() j/k navigation fire when the list is rendered inside an On.Modal
// key scope? It must — component bindings in a modal scope are wired into the
// modal router (app.go wireChildRouteScopes), same as a sibling Input/FilterList.
func TestOnModalRoutesListBindVimNav(t *testing.T) {
	app := NewApp()
	showOverlay := false
	items := []string{"alice", "bob", "carol"}
	sel := 0
	list := List(&items).Selection(&sel).
		Render(func(s *string) Component { return Text(s) }).
		BindVimNav()

	app.SetView(VBox(
		If(&showOverlay).Then(
			Overlay.Centered()(
				VBox(
					On.Modal(Key("<Esc>", func() { showOverlay = false })),
					list,
				),
			),
		),
	))

	app.render()
	if dispatchRune(app, 'j') {
		t.Fatal("hidden modal list handled 'j'")
	}

	showOverlay = true
	app.render()
	if !dispatchRune(app, 'j') {
		t.Fatal("modal-scoped List.BindVimNav should handle 'j'")
	}
	if sel != 1 {
		t.Fatalf("'j' under On.Modal should move selection 0->1, got %d", sel)
	}
	dispatchRune(app, 'j')
	if sel != 2 {
		t.Fatalf("second 'j' should move 1->2, got %d", sel)
	}
	dispatchRune(app, 'k')
	if sel != 1 {
		t.Fatalf("'k' should move 2->1, got %d", sel)
	}
}

// CheckList.Selection(*int) parity (recap #451 follow-up): the canonical modal
// multi-select — CheckList(&items).Selection(&sel).BindVimNav().BindToggle — must be
// fully component-driven, with selection held in an EXTERNAL pointer so it survives a
// rebuilt-every-frame overlay. j moves the external sel; space toggles the selected item.
func TestCheckListExternalSelectionUnderModal(t *testing.T) {
	type item struct {
		Name string
		Done bool
	}
	app := NewApp()
	showOverlay := false
	items := []item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	sel := 0

	build := func() Component {
		return VBox(
			If(&showOverlay).Then(
				Overlay.Centered()(
					VBox(
						On.Modal(Key("<Esc>", func() { showOverlay = false })),
						CheckList(&items).
							Selection(&sel).
							Checked(func(i *item) *bool { return &i.Done }).
							Render(func(i *item) Component { return Text(&i.Name) }).
							BindVimNav().
							BindToggle("<Space>"),
					),
				),
			),
		)
	}
	app.SetView(build())

	showOverlay = true
	app.render()

	if !dispatchRune(app, 'j') {
		t.Fatal("modal CheckList.BindVimNav should handle 'j'")
	}
	if sel != 1 {
		t.Fatalf("'j' should move EXTERNAL selection 0->1, got %d", sel)
	}
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialSpace}) {
		t.Fatal("modal CheckList.BindToggle should handle <Space>")
	}
	if !items[1].Done {
		t.Fatalf("space should toggle the selected (external sel=1) item's checkbox")
	}

	// the external pointer means selection survives a full View rebuild (the
	// overlay-rebuilt-every-frame case CheckList couldn't serve before)
	app.SetView(build())
	app.render()
	if sel != 1 {
		t.Fatalf("external selection must persist across rebuild, got %d", sel)
	}
	if !dispatchRune(app, 'j') || sel != 2 {
		t.Fatalf("nav continues from persisted selection after rebuild, got %d", sel)
	}
}
