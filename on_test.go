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
