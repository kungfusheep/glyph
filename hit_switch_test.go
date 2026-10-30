package glyph

import "testing"

// a tap on a list row that swaps the visible pane (phone layout: the list
// hides and a scrolling preview appears) must not wedge the input path or
// the next render.
func TestTapThatSwapsPanesThenRenders(t *testing.T) {
	type row struct{ Name string }
	rows := []row{{"a"}, {"b"}, {"c"}}
	lines := []string{"l0", "l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"}
	showList, showPreview := true, false
	sel := 0
	app := NewApp().Mouse(true)
	var sv *ScrollViewC
	tmpl := Build(HBox(
		If(&showList).Then(VBox.Grow(1)(
			List(&rows).Selection(&sel).OnSelect(func(*row) {
				showList, showPreview = false, true
			}).Render(func(r *row) Component {
				return VBox.PaddingVH(1, 2)(Text(&r.Name))
			}),
		)),
		If(&showPreview).Then(VBox.Grow(1)(
			ScrollView.Grow(1).Ref(func(s *ScrollViewC) { sv = s })(
				ForEach(&lines, func(l *string) Component { return Text(l) }),
			),
		)),
	))
	renderWithMouse(app, tmpl, 40, 6)
	if !app.handleMouse(press(3, 4)) || sel != 1 {
		t.Fatalf("tap on the second row should select it, sel=%d", sel)
	}
	renderWithMouse(app, tmpl, 40, 6)
	if sv == nil || !showPreview {
		t.Fatal("preview pane should be showing after the tap")
	}
	if !app.handleMouse(wheel(3, 3, true)) || sv.Layer().ScrollY() != 1 {
		t.Fatalf("wheel over the preview should scroll it, got %d", sv.Layer().ScrollY())
	}
	renderWithMouse(app, tmpl, 40, 6)
}
