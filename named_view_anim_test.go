package glyph

import (
	"bytes"
	"strings"
	"testing"
)

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

// A second View() under an existing name is a literal redefinition — warn once,
// toward control flow.
func TestViewGuardWarnsOnSameNameTwice(t *testing.T) {
	app := NewApp()
	var buf bytes.Buffer
	app.diagOut = &buf

	app.View("home", VBox(Text("a")))
	app.View("home", VBox(Text("a"))) // same name again

	out := buf.String()
	if !strings.Contains(out, "registered twice") || !strings.Contains(out, `"home"`) {
		t.Fatalf("expected a 'registered twice' warning naming the view, got: %q", out)
	}
	if !strings.Contains(out, "If().Then()") {
		t.Fatalf("warning should point at control flow, got: %q", out)
	}

	// re-registering a third time must NOT re-emit: each name warns at most once.
	buf.Reset()
	app.View("home", VBox(Text("a")))
	if buf.Len() != 0 {
		t.Fatalf("the redefine warning must fire at most once per name, got: %q", buf.String())
	}

	// UpdateView is the sanctioned recompile path — it must stay silent.
	buf.Reset()
	app.UpdateView("home", VBox(Text("b")))
	if buf.Len() != 0 {
		t.Fatalf("UpdateView must not warn (sanctioned recompile), got: %q", buf.String())
	}
}

// Distinct view names never warn — only re-registering the same name does. The
// toggle silences the guard entirely.
func TestViewGuardSilentWhenExpected(t *testing.T) {
	app := NewApp()
	var buf bytes.Buffer
	app.diagOut = &buf

	app.View("one", VBox(Text("x")))
	app.View("two", VBox(Text("x"))) // identical shape, different name — not a redefinition
	if buf.Len() != 0 {
		t.Fatalf("distinct view names must not warn, got: %q", buf.String())
	}

	app2 := NewApp()
	var buf2 bytes.Buffer
	app2.diagOut = &buf2
	app2.SetViewDiagnostic(false)
	app2.View("home", VBox(Text("a")))
	app2.View("home", VBox(Text("a"))) // redefinition, but diagnostic off
	if buf2.Len() != 0 {
		t.Fatalf("SetViewDiagnostic(false) must silence the guard, got: %q", buf2.String())
	}
}
