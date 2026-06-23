package glyph

import "testing"

// Named views must wire their template's requestRender (via SetApp) so animations can
// drive their own frames. The anim ticker only spins when requestRender != nil; SetView
// set it explicitly, but View/UpdateView relied on SetApp — which previously left it nil,
// freezing every tween in a named view (the "focus animations broken between views" bug
// that surfaces once an app moves panes to named views). This guards the wiring at the source.
func TestNamedViewWiresAnimationRenderer(t *testing.T) {
	app := NewApp()

	app.View("main", VBox(Text("hi")))
	tmpl := app.viewTemplates["main"]
	if tmpl == nil {
		t.Fatal("view not registered")
	}
	if tmpl.requestRender == nil {
		t.Fatal("named-view template has no requestRender — the animation ticker can never start, so tweens freeze")
	}

	// recompiling via UpdateView must keep it wired
	app.UpdateView("main", VBox(Text("bye")))
	if app.viewTemplates["main"].requestRender == nil {
		t.Fatal("UpdateView dropped the requestRender wiring — animations would freeze after a theme/view rebuild")
	}
}
