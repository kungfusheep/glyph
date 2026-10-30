package glyph

import (
	"testing"

	"github.com/kungfusheep/riffkey"
)

// renderWithMouse executes tmpl at w×h the way a full frame does, so the
// frame's pointer regions are committed for hit testing.
func renderWithMouse(app *App, tmpl *Template, w, h int) {
	tmpl.SetApp(app)
	buf := NewBuffer(w, h)
	app.beginHits()
	tmpl.Execute(buf, int16(w), int16(h))
	app.commitHits()
}

func press(x, y int) riffkey.Mouse {
	return riffkey.Mouse{X: x, Y: y, Button: riffkey.MouseLeft, Action: riffkey.MousePress}
}

func wheel(x, y int, down bool) riffkey.Mouse {
	b := riffkey.MouseWheelUp
	if down {
		b = riffkey.MouseWheelDown
	}
	return riffkey.Mouse{X: x, Y: y, Button: b, Action: riffkey.MousePress}
}

func TestTapTargetsResolveByPosition(t *testing.T) {
	app := NewApp().Mouse(true)
	var hit string
	tmpl := Build(VBox(
		Tap(Text("first"), func() { hit = "first" }),
		Tap(Text("second"), func() { hit = "second" }),
		Text("plain"),
	))
	renderWithMouse(app, tmpl, 20, 5)

	if !app.handleMouse(press(2, 1)) || hit != "second" {
		t.Fatalf("press on row 1 should tap second, got %q", hit)
	}
	if !app.handleMouse(press(0, 0)) || hit != "first" {
		t.Fatalf("press on row 0 should tap first, got %q", hit)
	}
	if app.handleMouse(press(0, 2)) {
		t.Fatal("press on plain text should not be handled")
	}
	// a block target owns its whole row, like a list row does
	if !app.handleMouse(press(10, 0)) || hit != "first" {
		t.Fatalf("press further along row 0 should still tap first, got %q", hit)
	}
}

func TestMouseOffIgnoresEverything(t *testing.T) {
	app := NewApp()
	called := false
	tmpl := Build(VBox(Tap(Text("x"), func() { called = true })))
	renderWithMouse(app, tmpl, 10, 2)
	if app.handleMouse(press(0, 0)) || called {
		t.Fatal("mouse off: no regions, no handling")
	}
}

func TestTapItemReceivesTheRowItem(t *testing.T) {
	type row struct{ Name string }
	rows := []row{{"a"}, {"b"}, {"c"}}
	app := NewApp().Mouse(true)
	var got string
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return TapItem(Text(&r.Name), func(r *row) { got = r.Name })
		}),
	))
	renderWithMouse(app, tmpl, 10, 5)
	if !app.handleMouse(press(0, 2)) || got != "c" {
		t.Fatalf("row 2 should resolve to item c, got %q", got)
	}
	if !app.handleMouse(press(0, 0)) || got != "a" {
		t.Fatalf("row 0 should resolve to item a, got %q", got)
	}
}

func TestJumpTargetsAreTappable(t *testing.T) {
	app := NewApp().Mouse(true)
	called := false
	tmpl := Build(VBox(Jump(Text("go"), func() { called = true })))
	renderWithMouse(app, tmpl, 10, 2)
	if !app.handleMouse(press(1, 0)) || !called {
		t.Fatal("a Jump target should respond to a tap")
	}
}

func TestOverlaySwallowsTapsOnContentBeneath(t *testing.T) {
	app := NewApp().Mouse(true)
	under := false
	tmpl := Build(VBox(
		Tap(VBox.Height(5)(Text("under")), func() { under = true }),
		Overlay.Centered().Size(6, 3)(VBox(Text("dialog"))),
	))
	renderWithMouse(app, tmpl, 20, 5)
	if app.handleMouse(press(10, 2)) || under {
		t.Fatal("a tap on the overlay body must not reach the content under it")
	}
	if !app.handleMouse(press(0, 0)) || !under {
		t.Fatal("a tap beside the overlay still reaches the content")
	}
}

func TestListRowsTapToSelectAndWheelToScroll(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f"}
	sel := 0
	app := NewApp().Mouse(true)
	moves, taps := 0, 0
	tmpl := Build(VBox.Height(3)(
		List(&items).Selection(&sel).OnSelect(func(*string) { moves++ }).OnTap(func(*string) { taps++ }),
	))
	renderWithMouse(app, tmpl, 10, 3)

	if !app.handleMouse(press(0, 2)) || sel != 2 || moves != 1 || taps != 1 {
		t.Fatalf("tap on row 2: sel=%d moves=%d taps=%d", sel, moves, taps)
	}
	// a second tap on the selected row is still a tap, but not a move
	if !app.handleMouse(press(0, 2)) || moves != 1 || taps != 2 {
		t.Fatalf("repeat tap: moves=%d taps=%d", moves, taps)
	}
	// wheel down past the window drags the selection along with it
	for i := 0; i < 3; i++ {
		if !app.handleMouse(wheel(0, 1, true)) {
			t.Fatal("wheel over the list should be handled")
		}
	}
	if sel != 3 {
		t.Fatalf("after scrolling three rows the selection should sit at the window top, got %d", sel)
	}
	if taps != 2 {
		t.Fatalf("a wheel move is not a tap, taps=%d", taps)
	}
	renderWithMouse(app, tmpl, 10, 3)
	// the window now starts at row 3, so the top screen row is item d
	if !app.handleMouse(press(0, 0)) || sel != 3 {
		t.Fatalf("tap on the top row should select d (3), got %d", sel)
	}
	if !app.handleMouse(press(0, 2)) || sel != 5 {
		t.Fatalf("tap on the bottom row should select f (5), got %d", sel)
	}
}

func TestScrollViewScrollsOnWheelAndTranslatesInnerTaps(t *testing.T) {
	app := NewApp().Mouse(true)
	var sv *ScrollViewC
	var tapped string
	svH := int16(3)
	lines := []string{"l0", "l1", "l2", "l3", "l4", "l5", "l6", "l7"}
	tmpl := Build(VBox.Height(3)(
		ScrollView.Height(&svH).Ref(func(s *ScrollViewC) { sv = s })(
			ForEach(&lines, func(l *string) Component {
				return TapItem(Text(l), func(l *string) { tapped = *l })
			}),
		),
	))
	renderWithMouse(app, tmpl, 10, 3)
	if sv == nil {
		t.Fatal("scrollview ref not set")
	}
	if !app.handleMouse(press(0, 1)) || tapped != "l1" {
		t.Fatalf("unscrolled tap on row 1 should hit l1, got %q", tapped)
	}
	for i := 0; i < 2; i++ {
		if !app.handleMouse(wheel(0, 1, true)) {
			t.Fatal("wheel over the scrollview should be handled")
		}
	}
	if sv.Layer().ScrollY() != 2 {
		t.Fatalf("two wheel downs should scroll two rows, got %d", sv.Layer().ScrollY())
	}
	renderWithMouse(app, tmpl, 10, 3)
	if !app.handleMouse(press(0, 1)) || tapped != "l3" {
		t.Fatalf("scrolled tap on row 1 should hit l3, got %q", tapped)
	}
}

func TestInlineTapTargetsAreContentSized(t *testing.T) {
	app := NewApp().Mouse(true)
	var hit string
	tmpl := Build(HBox(
		Tap(Text("ab"), func() { hit = "ab" }),
		Tap(Text("cd"), func() { hit = "cd" }),
	))
	renderWithMouse(app, tmpl, 20, 1)
	if !app.handleMouse(press(3, 0)) || hit != "cd" {
		t.Fatalf("press at x=3 should tap cd, got %q", hit)
	}
	if !app.handleMouse(press(1, 0)) || hit != "ab" {
		t.Fatalf("press at x=1 should tap ab, got %q", hit)
	}
	if app.handleMouse(press(8, 0)) {
		t.Fatal("press past both targets should not be handled")
	}
}

func TestRichTextSpanLinksAreTappable(t *testing.T) {
	app := NewApp().Mouse(true)
	var hit string
	spans := []Span{
		{Text: "read the "},
		{Text: "docs", OnSelect: func() { hit = "docs" }},
		{Text: " or the "},
		{Text: "source", OnSelect: func() { hit = "source" }},
	}
	tmpl := Build(VBox(Rich(spans)))
	renderWithMouse(app, tmpl, 40, 2)

	if !app.handleMouse(press(10, 0)) || hit != "docs" {
		t.Fatalf("press inside 'docs' should tap it, got %q", hit)
	}
	if !app.handleMouse(press(22, 0)) || hit != "source" {
		t.Fatalf("press inside 'source' should tap it, got %q", hit)
	}
	if app.handleMouse(press(2, 0)) {
		t.Fatal("press on plain text should not be handled")
	}
}

func TestRichTextSpanLinksInsideScrollViewTranslate(t *testing.T) {
	app := NewApp().Mouse(true)
	var sv *ScrollViewC
	var hit string
	lines := []string{"l0", "l1", "l2", "l3", "l4"}
	link := []Span{{Text: "open", OnSelect: func() { hit = "open" }}}
	svH := int16(3)
	tmpl := Build(VBox.Height(3)(
		ScrollView.Height(&svH).Ref(func(s *ScrollViewC) { sv = s })(
			ForEach(&lines, func(l *string) Component { return Text(l) }),
			Rich(link),
		),
	))
	renderWithMouse(app, tmpl, 20, 3)
	if app.handleMouse(press(1, 1)) {
		t.Fatal("the link is below the viewport before scrolling")
	}
	sv.Layer().ScrollToEnd() // six content rows in a three-row viewport: the link sits on screen row 2
	renderWithMouse(app, tmpl, 20, 3)
	if app.handleMouse(press(1, 0)) {
		t.Fatal("row 0 is a plain line after scrolling")
	}
	if !app.handleMouse(press(1, 2)) || hit != "open" {
		t.Fatalf("after scrolling, a press on the link's row should tap it, got %q", hit)
	}
}
