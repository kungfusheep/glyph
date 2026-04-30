package glyph_test

import (
	"testing"

	. "github.com/kungfusheep/glyph"
)

// Registering a view that contains a Form must not touch the global input
// stack. Activation (via PushView/Go/RunFrom) is what pushes the focus
// manager's initial sub-router; deactivation (PopView/Go-elsewhere) pops it.
//
// Regression: previously wireBindings called fm.initialPush() eagerly during
// View(), which corrupted input routing for any other view that was already
// active (e.g. typing in a SetView dashboard stopped working as soon as a
// modal form view was registered).
func TestViewWithFormDoesNotPushOnRegistration(t *testing.T) {
	app := NewApp()

	var name string
	form := Form(
		Field("name", Input(&name)),
	)

	beforeDepth := app.Input().Depth()
	app.View("modal", VBox(form))
	afterDepth := app.Input().Depth()

	if afterDepth != beforeDepth {
		t.Fatalf("registering a form-bearing view changed input depth: %d -> %d (form push leaked into global stack)", beforeDepth, afterDepth)
	}
}

// PushView for a form-bearing view should add two frames to the input stack:
// the view's main router, then the focus manager's first-field sub-router.
// PopView reverses both.
func TestViewActivationPushesFocusManagerFrame(t *testing.T) {
	app := NewApp()

	var name string
	form := Form(
		Field("name", Input(&name)),
	)

	app.View("modal", VBox(form))

	baseDepth := app.Input().Depth()
	app.PushView("modal")
	afterPush := app.Input().Depth()

	if afterPush != baseDepth+2 {
		t.Fatalf("PushView of form view should add 2 frames (view router + FM sub-router); got %d -> %d", baseDepth, afterPush)
	}

	app.PopView()
	afterPop := app.Input().Depth()

	if afterPop != baseDepth {
		t.Fatalf("PopView should restore input depth: started at %d, after pop got %d", baseDepth, afterPop)
	}
}

// A view with no focus manager should push only its own router on activation.
func TestViewWithoutFormPushesSingleFrame(t *testing.T) {
	app := NewApp()

	app.View("plain", VBox(Text("hello")))

	baseDepth := app.Input().Depth()
	app.PushView("plain")
	afterPush := app.Input().Depth()

	if afterPush != baseDepth+1 {
		t.Fatalf("PushView of plain view should add 1 frame; got %d -> %d", baseDepth, afterPush)
	}

	app.PopView()
	afterPop := app.Input().Depth()

	if afterPop != baseDepth {
		t.Fatalf("PopView should restore input depth: started at %d, after pop got %d", baseDepth, afterPop)
	}
}

