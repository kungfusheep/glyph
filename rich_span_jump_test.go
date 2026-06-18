package glyph

import "testing"

func TestWrapSpansRegistersJumpAtFirstRenderedLinkCell(t *testing.T) {
	buf := NewBuffer(20, 4)
	called := false
	spans := []Span{
		{Text: "before "},
		{Text: "link text", OnSelect: func() { called = true }},
		{Text: " after"},
	}

	var gotX, gotY int
	var got Span
	wrapSpansDraw(spans, buf, 2, 1, 20, 4, false, func(x, y int, span Span) {
		gotX, gotY = x, y
		got = span
	})

	if gotX != 9 || gotY != 1 {
		t.Fatalf("jump target = (%d,%d), want (9,1)", gotX, gotY)
	}
	if got.Text != "link text" || got.OnSelect == nil {
		t.Fatalf("jump span = %#v, want link text with callback", got)
	}
	got.OnSelect()
	if !called {
		t.Fatal("jump callback was not preserved")
	}
}

func TestWrapSpansRegistersJumpAfterWordWrap(t *testing.T) {
	buf := NewBuffer(12, 4)
	spans := []Span{
		{Text: "before"},
		{Text: "long link", OnSelect: func() {}},
	}

	var gotX, gotY int
	wrapSpansDraw(spans, buf, 0, 0, 8, 4, false, func(x, y int, span Span) {
		gotX, gotY = x, y
	})

	if gotX != 0 || gotY != 1 {
		t.Fatalf("jump target = (%d,%d), want wrapped link at (0,1)", gotX, gotY)
	}
}

func TestWrapSpansKeepsPendingSpaceStyle(t *testing.T) {
	buf := NewBuffer(20, 2)
	linkStyle := Style{FG: Blue, Attr: AttrUnderline}
	spans := []Span{
		{Text: "before"},
		{Text: " "},
		{Text: "link", Style: linkStyle},
	}

	wrapSpansDraw(spans, buf, 0, 0, 20, 2, false, nil)

	space := buf.Get(6, 0)
	if space.Rune != ' ' {
		t.Fatalf("cell 6 rune = %q, want space", space.Rune)
	}
	if space.Style.Attr&AttrUnderline != 0 || space.Style.FG.Equal(linkStyle.FG) {
		t.Fatalf("space style = %#v, want plain spacer before link", space.Style)
	}
	link := buf.Get(7, 0)
	if link.Rune != 'l' {
		t.Fatalf("cell 7 rune = %q, want link start", link.Rune)
	}
	if link.Style.Attr&AttrUnderline == 0 || !link.Style.FG.Equal(linkStyle.FG) {
		t.Fatalf("link style = %#v, want underlined link style", link.Style)
	}
}

func TestRichSpanRegistersNativeJumpTarget(t *testing.T) {
	app := NewApp()
	selected := false
	tree := Rich([]Span{
		{Text: "before "},
		{Text: "link", OnSelect: func() { selected = true }},
	})
	tmpl := Build(tree)
	tmpl.SetApp(app)

	app.jumpMode.setActive(true)
	buf := NewBuffer(20, 2)
	tmpl.Execute(buf, 20, 2)
	app.jumpMode.AssignLabels()

	if len(app.jumpMode.Targets) != 1 {
		t.Fatalf("targets = %#v, want one rich span jump target", app.jumpMode.Targets)
	}
	target := app.jumpMode.Targets[0]
	if target.X != 7 || target.Y != 0 {
		t.Fatalf("target = (%d,%d), want first link cell at (7,0)", target.X, target.Y)
	}
	target.OnSelect()
	if !selected {
		t.Fatal("target did not call rich span callback")
	}
}

func TestRichSpanSelectRefReceivesRenderedBounds(t *testing.T) {
	app := NewApp()
	var selected NodeRef
	tree := Rich([]Span{
		{Text: "before "},
		{Text: "link", OnSelectRef: func(ref NodeRef) { selected = ref }},
	})
	tmpl := Build(tree)
	tmpl.SetApp(app)

	app.jumpMode.setActive(true)
	buf := NewBuffer(20, 2)
	tmpl.Execute(buf, 20, 2)
	app.jumpMode.AssignLabels()

	if len(app.jumpMode.Targets) != 1 {
		t.Fatalf("targets = %#v, want one rich span jump target", app.jumpMode.Targets)
	}
	app.jumpMode.Targets[0].OnSelect()
	if selected.X != 7 || selected.Y != 0 || selected.W != 4 || selected.H != 1 {
		t.Fatalf("selected ref = %#v, want rendered span bounds", selected)
	}
}

func TestScrollViewRichSpanRegistersVisibleJumpTarget(t *testing.T) {
	app := NewApp()
	selected := false
	tree := VBox(
		HBox.Grow(1)(
			SpaceW(5),
			ScrollView.Grow(1)(
				SpaceH(2),
				Rich([]Span{
					{Text: "before "},
					{Text: "link", OnSelect: func() { selected = true }},
				}),
			),
		),
	)
	tmpl := Build(tree)
	tmpl.SetApp(app)

	app.jumpMode.setActive(true)
	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)
	app.jumpMode.AssignLabels()

	if len(app.jumpMode.Targets) != 1 {
		t.Fatalf("targets = %#v, want one visible scrollview rich span jump target", app.jumpMode.Targets)
	}
	target := app.jumpMode.Targets[0]
	if target.X != 12 || target.Y != 2 {
		t.Fatalf("target = (%d,%d), want visible link cell at (12,2)", target.X, target.Y)
	}
	target.OnSelect()
	if !selected {
		t.Fatal("target did not call scrollview rich span callback")
	}

	if got := buf.Get(12, 2).Rune; got != 'l' {
		t.Fatalf("scroll layer rendered label into content buffer: got %q, want original link text", got)
	}
	app.paintJumpLabels(buf, 4)
	if got := buf.Get(12, 2).Rune; got != 'a' {
		t.Fatalf("painted label = %q, want first jump label", got)
	}
}

func TestScrollViewRichSpanJumpTargetsAreVisibleOnlyAndOrderedByScreenPosition(t *testing.T) {
	app := NewApp()
	sv := ScrollView.Grow(1)(
		Rich([]Span{{Text: "hidden", OnSelect: func() {}}}),
		SpaceH(4),
		Rich([]Span{{Text: "first", OnSelect: func() {}}}),
		SpaceH(1),
		Rich([]Span{{Text: "second", OnSelect: func() {}}}),
	)
	tree := VBox(sv)
	tmpl := Build(tree)
	tmpl.SetApp(app)

	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)
	sv.Layer().ScrollTo(5)

	app.jumpMode.setActive(true)
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	app.jumpMode.AssignLabels()

	if len(app.jumpMode.Targets) != 2 {
		t.Fatalf("targets = %#v, want only two visible jump targets", app.jumpMode.Targets)
	}
	if app.jumpMode.Targets[0].Label != "a" {
		t.Fatalf("first visible target = %#v, want first home-row label", app.jumpMode.Targets[0])
	}
	if app.jumpMode.Targets[1].Label != "s" {
		t.Fatalf("second visible target = %#v, want second home-row label", app.jumpMode.Targets[1])
	}
	if app.jumpMode.Targets[0].Y >= app.jumpMode.Targets[1].Y {
		t.Fatalf("targets = %#v, want visible screen order", app.jumpMode.Targets)
	}
}

func TestScrollViewForEachRichSpanInheritsJumpViewport(t *testing.T) {
	app := NewApp()
	items := []struct {
		Spans []Span
	}{
		{Spans: []Span{{Text: "before "}, {Text: "link", OnSelect: func() {}}}},
	}
	tree := VBox(
		HBox.Grow(1)(
			SpaceW(5),
			ScrollView.Grow(1)(
				ForEach(&items, func(item *struct{ Spans []Span }) Component {
					return Rich(&item.Spans)
				}),
			),
		),
	)
	tmpl := Build(tree)
	tmpl.SetApp(app)

	app.jumpMode.setActive(true)
	buf := NewBuffer(24, 3)
	tmpl.Execute(buf, 24, 3)
	app.jumpMode.AssignLabels()

	if len(app.jumpMode.Targets) != 1 {
		t.Fatalf("targets = %#v, want one foreach rich span jump target", app.jumpMode.Targets)
	}
	target := app.jumpMode.Targets[0]
	if target.X != 12 || target.Y != 0 {
		t.Fatalf("target = (%d,%d), want translated foreach link at (12,0)", target.X, target.Y)
	}
}
