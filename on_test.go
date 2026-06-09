package glyph

import (
	"testing"

	"github.com/kungfusheep/riffkey"
)

func dispatchRune(app *App, r rune) bool {
	return app.Input().Dispatch(riffkey.Key{Rune: r})
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
