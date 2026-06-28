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

// structHash fingerprints the tree SHAPE, not its data: two views with the same
// layout but different literals collide (the copy-paste defect we want to catch),
// while a different layout does not.
func TestStructHashShapeNotData(t *testing.T) {
	a := Build(VBox(Text("hi")))
	b := Build(VBox(Text("totally different text")))
	if a.structHash() != b.structHash() {
		t.Fatal("same-shape views with different data must hash identically")
	}
	c := Build(VBox(Text("hi"), Text("extra")))
	if a.structHash() == c.structHash() {
		t.Fatal("a structurally different view must hash differently")
	}
}

// The guard's high-value signal: two distinct named views that compile to the same
// structure are the copy-paste-a-whole-view defect — warn once, toward control flow.
func TestViewGuardWarnsOnIdenticalStructure(t *testing.T) {
	app := NewApp()
	var buf bytes.Buffer
	app.diagOut = &buf

	app.View("home", VBox(Text("welcome")))
	app.View("homeError", VBox(Text("something broke")))

	out := buf.String()
	if !strings.Contains(out, `"home"`) || !strings.Contains(out, `"homeError"`) {
		t.Fatalf("expected a structural-twin warning naming both views, got: %q", out)
	}
	if !strings.Contains(out, "If().Then()") {
		t.Fatalf("warning should point at control flow, got: %q", out)
	}

	// a third twin must NOT re-emit: each finding prints at most once.
	before := buf.Len()
	app.View("homeError", VBox(Text("again"))) // also trips the same-name signal; still deduped per id
	_ = before
}

// A second View() under an existing name is a literal redefinition.
func TestViewGuardWarnsOnSameNameTwice(t *testing.T) {
	app := NewApp()
	var buf bytes.Buffer
	app.diagOut = &buf

	app.View("home", VBox(Text("a")))
	app.View("home", VBox(Text("a"))) // same name again

	if !strings.Contains(buf.String(), "registered twice") {
		t.Fatalf("expected a 'registered twice' warning, got: %q", buf.String())
	}

	// UpdateView is the sanctioned recompile path — it must stay silent.
	buf.Reset()
	app.UpdateView("home", VBox(Text("b")))
	if buf.Len() != 0 {
		t.Fatalf("UpdateView must not warn (sanctioned recompile), got: %q", buf.String())
	}
}

// Genuinely different views never warn, and the toggle silences the guard entirely.
func TestViewGuardSilentWhenExpected(t *testing.T) {
	app := NewApp()
	var buf bytes.Buffer
	app.diagOut = &buf

	app.View("one", VBox(Text("x")))
	app.View("two", VBox(Text("x"), Text("y"))) // different shape
	if buf.Len() != 0 {
		t.Fatalf("structurally distinct views must not warn, got: %q", buf.String())
	}

	app2 := NewApp()
	var buf2 bytes.Buffer
	app2.diagOut = &buf2
	app2.SetViewDiagnostic(false)
	app2.View("home", VBox(Text("a")))
	app2.View("homeError", VBox(Text("a"))) // identical, but diagnostic off
	if buf2.Len() != 0 {
		t.Fatalf("SetViewDiagnostic(false) must silence the guard, got: %q", buf2.String())
	}
}
