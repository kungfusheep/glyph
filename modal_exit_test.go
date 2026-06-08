package glyph

import "testing"

// regression: a modal overlay with a fade-out exit animation must RELEASE its On.Modal
// router when its condition goes false — even while the fade plays. Otherwise the router
// orphans on the input stack and swallows every key (dead keys). This is the framework
// fix for what consumers were bodging by manually popping the input stack.
func TestModalOverlayExitReleasesRouter(t *testing.T) {
	app := NewApp()
	open := false
	app.SetView(VBox.Height(20)(
		On(Key("x", func() {})), // base router
		If(&open).Then(
			Overlay.Centered()(
				VBox.Width(20).Opacity(In(1).Out(Animate(0)))(
					On.Modal(Key("a", func() {})),
					Text("modal"),
				),
			),
		),
	))
	app.RenderNow()
	base := app.Input().Depth()

	open = true
	app.RenderNow()
	if app.Input().Depth() <= base {
		t.Fatalf("open modal should push a router (base=%d, got=%d)", base, app.Input().Depth())
	}

	open = false
	app.RenderNow() // exiting (fading) frame — router must be released here, not deferred
	if d := app.Input().Depth(); d != base {
		t.Fatalf("exiting modal orphaned its router: depth %d, want base %d", d, base)
	}
}
