package glyph

import (
	"fmt"
	"testing"
	"time"
)

// The headline: ScrollView.ScrollOffset(...) binds the offset, and the ScrollView's
// scroll methods drive it — no var, no pointer, no hard cut. Instant form here (the
// eased form's frame-by-frame easing is covered at the Layer level).
func TestScrollView_ScrollOffsetBindsAndDrives(t *testing.T) {
	rows := make([]Component, 60)
	for i := range rows {
		rows[i] = Text(fmt.Sprintf("line%d", i))
	}
	sv := ScrollView.Grow(1).ScrollOffset(ScrollState())(rows...)
	tmpl := Build(VBox(sv))

	if sv.Layer().scrollTarget == nil {
		t.Fatal("ScrollOffset(ScrollState()) did not bind the layer offset")
	}
	screen := NewBuffer(12, 6)
	tmpl.Execute(screen, 12, 6) // viewport 6, content 60 -> maxScroll 54

	// a scroll METHOD drives the bound target; instant offset reaches the end.
	sv.Layer().ScrollToEnd()
	if got := sv.Layer().ScrollY(); got != 54 {
		t.Fatalf("ScrollToEnd via bound offset: ScrollY=%d, want 54 (maxScroll)", got)
	}
	screen.Clear()
	tmpl.Execute(screen, 12, 6)
	if got := screen.GetLine(5); got != "line59" {
		t.Errorf("after ScrollToEnd: bottom row = %q, want line59", got)
	}
}

// ScrollOffset(Animate(ScrollState())) binds BOTH the offset and the ease config, so the
// same scroll methods become animation-aware (the headline DX).
func TestScrollView_ScrollOffsetAnimateBindsEase(t *testing.T) {
	sv := ScrollView.Grow(1).ScrollOffset(
		Animate.Duration(120 * time.Millisecond).Ease(EaseOutCubic)(ScrollState()),
	)(Text("a"), Text("b"))
	Build(VBox(sv))
	if sv.Layer().scrollTarget == nil {
		t.Fatal("Animate(ScrollState()) did not bind the offset")
	}
	if sv.Layer().scrollEaseDur != 120*time.Millisecond {
		t.Errorf("ease duration = %v, want 120ms (Animate config not bound)", sv.Layer().scrollEaseDur)
	}
	if sv.Layer().scrollEaseFn == nil {
		t.Error("ease function not bound from Animate")
	}
}

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

// AnchorBottom: content shorter than the viewport hugs the BOTTOM, slack at the top.
func TestScrollView_AnchorBottomUnderflow(t *testing.T) {
	sv := ScrollView.Grow(1).AnchorBottom()(
		Text("alpha"),
		Text("beta"),
	)
	screen := NewBuffer(12, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 12, 6)

	// 2 lines in a 6-tall viewport: rows 0-3 blank, content in rows 4-5.
	for r := 0; r < 4; r++ {
		if got := screen.GetLine(r); got != "" {
			t.Errorf("row %d = %q, want blank (slack at top)", r, got)
		}
	}
	if got := screen.GetLine(4); got != "alpha" {
		t.Errorf("row 4 = %q, want alpha (bottom-anchored)", got)
	}
	if got := screen.GetLine(5); got != "beta" {
		t.Errorf("row 5 = %q, want beta (bottom-anchored, just above where a composer sits)", got)
	}
}

// Without the flag, underflowing content stays TOP-anchored — regression guard for the default.
func TestScrollView_NoAnchorBottomStaysTop(t *testing.T) {
	sv := ScrollView.Grow(1)(
		Text("alpha"),
		Text("beta"),
	)
	screen := NewBuffer(12, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 12, 6)

	if got := screen.GetLine(0); got != "alpha" {
		t.Errorf("row 0 = %q, want alpha (default top-anchor)", got)
	}
	if got := screen.GetLine(1); got != "beta" {
		t.Errorf("row 1 = %q, want beta (default top-anchor)", got)
	}
	for r := 2; r < 6; r++ {
		if got := screen.GetLine(r); got != "" {
			t.Errorf("row %d = %q, want blank (default: slack below)", r, got)
		}
	}
}

// AnchorBottom is a no-op when content OVERFLOWS the viewport: normal scrolling resumes,
// and ScrollTo(maxScroll) still reaches the last line (the flag only affects underflow).
func TestScrollView_AnchorBottomOverflowIsNoOp(t *testing.T) {
	lines := make([]Component, 20) // 20 lines into a 6-tall viewport: overflow
	for i := range lines {
		lines[i] = Text(fmt.Sprintf("line%d", i))
	}
	sv := ScrollView.Grow(1).AnchorBottom()(lines...)
	screen := NewBuffer(12, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 12, 6)

	// top-anchored at rest (flag no-op when tall): first line at row 0.
	if got := screen.GetLine(0); got != "line0" {
		t.Fatalf("overflow at rest: row 0 = %q, want line0 (AnchorBottom must be a no-op when tall)", got)
	}
	// tail-follow still works.
	sv.Layer().ScrollToEnd()
	screen.Clear()
	tmpl.Execute(screen, 12, 6)
	if got := screen.GetLine(5); got != "line19" {
		t.Fatalf("overflow scroll-to-end: bottom row = %q, want line19 (ScrollTo must still tail-follow)", got)
	}
}

// The boundary seam: exact-fit (content == viewport) positions identically with and without
// the flag, and the AnchorBottom/ScrollTo handoff is clean — below the seam AnchorBottom
// positions and ScrollTo is a no-op; at the seam ScrollTo can tail-follow and AnchorBottom
// is inert. This is the case chat panes ride (tail-follow ScrollTo after every message).
func TestScrollView_AnchorBottomBoundaryHandoff(t *testing.T) {
	mk := func(n int, anchor bool) *ScrollViewC {
		lines := make([]Component, n)
		for i := range lines {
			lines[i] = Text(fmt.Sprintf("line%d", i))
		}
		f := ScrollView.Grow(1)
		if anchor {
			f = f.AnchorBottom()
		}
		return f(lines...)
	}

	// exact fit: 6 lines in a 6-tall viewport. With or without the flag, line0 at row 0,
	// line5 at row 5 — no bottom-shift because there is no slack.
	for _, anchor := range []bool{false, true} {
		sv := mk(6, anchor)
		screen := NewBuffer(12, 6)
		tmpl := Build(VBox(sv))
		tmpl.Execute(screen, 12, 6)
		if got := screen.GetLine(0); got != "line0" {
			t.Errorf("exact-fit anchor=%v: row 0 = %q, want line0", anchor, got)
		}
		if got := screen.GetLine(5); got != "line5" {
			t.Errorf("exact-fit anchor=%v: row 5 = %q, want line5", anchor, got)
		}
	}

	// one under the seam (5 lines, 6-tall): AnchorBottom positions to the bottom (line4 at
	// row 5), and ScrollTo is a no-op there (maxScroll == 0), so it cannot fight.
	sv := mk(5, true)
	screen := NewBuffer(12, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 12, 6)
	if got := screen.GetLine(5); got != "line4" {
		t.Fatalf("under-seam: bottom row = %q, want line4 (AnchorBottom positions)", got)
	}
	sv.Layer().ScrollToEnd() // no-op below the seam
	screen.Clear()
	tmpl.Execute(screen, 12, 6)
	if got := screen.GetLine(5); got != "line4" {
		t.Fatalf("under-seam after ScrollToEnd: bottom row = %q, want line4 (ScrollTo must be inert)", got)
	}
}

// Feather fades the overflowing edges toward the background: top only when scrolled down,
// bottom only when more remains; middle rows and the off-flag path are untouched.
func TestScrollView_FeatherFadesOverflowingEdges(t *testing.T) {
	const n = 30
	rows := func() []Component {
		cs := make([]Component, n)
		for i := range cs {
			cs[i] = Text(fmt.Sprintf("row%d", i)).FG(RGB(200, 200, 200))
		}
		return cs
	}
	render := func(feather, scrollTo int) *Buffer {
		sv := ScrollView.Grow(1).Feather(feather)(rows()...)
		screen := NewBuffer(10, 8)
		tmpl := Build(VBox(sv))
		sv.Layer().defaultStyle = Style{BG: RGB(0, 0, 0)}
		tmpl.Execute(screen, 10, 8) // first pass computes maxScroll
		sv.Layer().defaultStyle = Style{BG: RGB(0, 0, 0)}
		sv.Layer().ScrollTo(scrollTo)
		screen.Clear()
		tmpl.Execute(screen, 10, 8)
		return screen
	}
	fg := func(b *Buffer, y int) Color { return b.Get(0, y).Style.FG }

	// mid-scroll: both edges overflow → both fade; a middle row does not.
	base := render(0, 10)
	feat := render(2, 10)
	if fg(feat, 0) == fg(base, 0) {
		t.Error("mid-scroll: top edge row should be faded (scrolled down), unchanged")
	}
	if fg(feat, 7) == fg(base, 7) {
		t.Error("mid-scroll: bottom edge row should be faded (more below), unchanged")
	}
	if fg(feat, 4) != fg(base, 4) {
		t.Error("mid-scroll: middle row must NOT fade")
	}

	// at the top: top edge must NOT fade; bottom still overflows → fades.
	baseTop := render(0, 0)
	featTop := render(2, 0)
	if fg(featTop, 0) != fg(baseTop, 0) {
		t.Error("at top: top edge must NOT fade (nothing above)")
	}
	if fg(featTop, 7) == fg(baseTop, 7) {
		t.Error("at top: bottom edge should still fade (more below)")
	}

	// at the end: bottom edge must NOT fade; top still overflows → fades.
	baseEnd := render(0, 1<<20)
	featEnd := render(2, 1<<20)
	if fg(featEnd, 7) != fg(baseEnd, 7) {
		t.Error("at end: bottom edge must NOT fade (scrolled all the way down)")
	}
	if fg(featEnd, 0) == fg(baseEnd, 0) {
		t.Error("at end: top edge should still fade (more above)")
	}
}

// Feather(0) (the default) leaves blit output byte-for-byte identical — off-path guard.
func TestScrollView_FeatherZeroUnchanged(t *testing.T) {
	rows := make([]Component, 30)
	for i := range rows {
		rows[i] = Text(fmt.Sprintf("row%d", i)).FG(RGB(200, 200, 200))
	}
	mk := func(feather int) *Buffer {
		sv := ScrollView.Grow(1).Feather(feather)(rows...)
		screen := NewBuffer(10, 8)
		tmpl := Build(VBox(sv))
		sv.Layer().defaultStyle = Style{BG: RGB(0, 0, 0)}
		tmpl.Execute(screen, 10, 8)
		sv.Layer().defaultStyle = Style{BG: RGB(0, 0, 0)}
		sv.Layer().ScrollTo(10)
		screen.Clear()
		tmpl.Execute(screen, 10, 8)
		return screen
	}
	off := mk(0)
	for y := 0; y < 8; y++ {
		for x := 0; x < 10; x++ {
			c := off.Get(x, y)
			// with feather 0, an edge cell's FG is the raw content colour, not blended.
			if c.Rune == 'r' && c.Style.FG != RGB(200, 200, 200) {
				t.Fatalf("Feather(0): cell (%d,%d) FG = %+v, want raw RGB(200,200,200) — off-path altered", x, y, c.Style.FG)
			}
		}
	}
}
