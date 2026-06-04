package glyph

import (
	"testing"
)

func TestScrollView_RendersChildren(t *testing.T) {
	sv := ScrollView.Grow(1)(
		HBox(Text("Alice").Bold(), SpaceW(1), Text("12:30").Dim()),
		Text("Hello, how are you?"),
		SpaceH(1),
		HBox(Text("You").Bold(), SpaceW(1), Text("12:35").Dim()),
		Text("Good thanks!"),
	)

	screen := NewBuffer(40, 10)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 40, 10)

	if got := screen.GetLine(0); got != "Alice 12:30" {
		t.Errorf("line 0: got %q, want %q", got, "Alice 12:30")
	}
	if got := screen.GetLine(1); got != "Hello, how are you?" {
		t.Errorf("line 1: got %q, want %q", got, "Hello, how are you?")
	}
	if got := screen.GetLine(3); got != "You 12:35" {
		t.Errorf("line 3: got %q, want %q", got, "You 12:35")
	}
	if got := screen.GetLine(4); got != "Good thanks!" {
		t.Errorf("line 4: got %q, want %q", got, "Good thanks!")
	}
}

func TestScrollView_StyledAttributes(t *testing.T) {
	sv := ScrollView.Grow(1)(
		Text("bold").Bold(),
		Text("dim").Dim(),
	)

	screen := NewBuffer(20, 5)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 5)

	cell := screen.Get(0, 0)
	if cell.Rune != 'b' || cell.Style.Attr&AttrBold == 0 {
		t.Errorf("(0,0): rune=%c bold=%v, want 'b' bold=true", cell.Rune, cell.Style.Attr&AttrBold != 0)
	}

	cell = screen.Get(0, 1)
	if cell.Rune != 'd' || cell.Style.Attr&AttrDim == 0 {
		t.Errorf("(0,1): rune=%c dim=%v, want 'd' dim=true", cell.Rune, cell.Style.Attr&AttrDim != 0)
	}
}

func TestScrollView_ScrollsContent(t *testing.T) {
	// content taller than viewport
	sv := ScrollView.Grow(1)(
		Text("line0"),
		Text("line1"),
		Text("line2"),
		Text("line3"),
		Text("line4"),
		Text("line5"),
		Text("line6"),
		Text("line7"),
	)

	screen := NewBuffer(20, 4)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 4)

	// initially shows top
	if got := screen.GetLine(0); got != "line0" {
		t.Errorf("before scroll line 0: got %q, want %q", got, "line0")
	}
	if got := screen.GetLine(3); got != "line3" {
		t.Errorf("before scroll line 3: got %q, want %q", got, "line3")
	}

	// scroll down
	sv.Layer().ScrollDown(2)
	screen.ClearDirty()
	tmpl.Execute(screen, 20, 4)

	if got := screen.GetLine(0); got != "line2" {
		t.Errorf("after scroll line 0: got %q, want %q", got, "line2")
	}
	if got := screen.GetLine(3); got != "line5" {
		t.Errorf("after scroll line 3: got %q, want %q", got, "line5")
	}
}

func TestScrollView_Refresh(t *testing.T) {
	content := "original"
	sv := ScrollView.Grow(1)(
		Text(&content),
	)

	screen := NewBuffer(20, 5)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 5)

	if got := screen.GetLine(0); got != "original" {
		t.Errorf("before refresh: got %q, want %q", got, "original")
	}

	// change content and refresh
	content = "updated"
	sv.Refresh()
	screen.ClearDirty()
	tmpl.Execute(screen, 20, 5)

	if got := screen.GetLine(0); got != "updated" {
		t.Errorf("after refresh: got %q, want %q", got, "updated")
	}
}

func TestScrollView_WithScrollbarReservesGutterOutsideLayer(t *testing.T) {
	sv := ScrollView.Grow(1).Scrollbar()(
		Text("01234567890"),
		Text("line1"),
		Text("line2"),
		Text("line3"),
		Text("line4"),
		Text("line5"),
	)

	screen := NewBuffer(12, 3)
	tmpl := Build(sv)
	tmpl.Execute(screen, 12, 3)

	if got := sv.Layer().ViewportWidth(); got != 11 {
		t.Fatalf("scroll layer width = %d, want 11 with one-column gutter", got)
	}
	if got := screen.Get(10, 0).Rune; got != '0' {
		t.Fatalf("last content column = %q, want 0 before gutter\n%s", got, screen.String())
	}
	if got := screen.Get(11, 0).Rune; got != '█' {
		t.Fatalf("scrollbar top glyph = %q, want thumb in gutter\n%s", got, screen.String())
	}
}

func TestScrollViewScrollbarFollowsLayerScroll(t *testing.T) {
	sv := ScrollView.Grow(1).Scrollbar()(
		Text("line0"),
		Text("line1"),
		Text("line2"),
		Text("line3"),
		Text("line4"),
		Text("line5"),
	)

	screen := NewBuffer(12, 3)
	tmpl := Build(sv)
	tmpl.Execute(screen, 12, 3)

	sv.Layer().ScrollToEnd()
	screen.Clear()
	tmpl.Execute(screen, 12, 3)

	if got := screen.Get(11, 2).Rune; got != '█' {
		t.Fatalf("scrollbar bottom glyph = %q, want thumb at bottom\n%s", got, screen.String())
	}
}
