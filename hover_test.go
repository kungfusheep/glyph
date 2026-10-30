package glyph

import (
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/kungfusheep/riffkey"
)

// newHoverApp builds an app that can run real frames at w×h with the screen
// output discarded, the way the resize test drives render().
func newHoverApp(w, h int, root Component) *App {
	app := NewApp().Mouse(true)
	app.screen.writer = io.Discard
	app.SetView(root)
	app.pool = NewBufferPool(w, h)
	app.screen.width, app.screen.height = w, h
	app.screen.front.Resize(w, h)
	app.screen.back.Resize(w, h)
	return app
}

func motion(x, y int) riffkey.Mouse {
	return riffkey.Mouse{X: x, Y: y, Button: riffkey.MouseNone, Action: riffkey.MouseMotion}
}

func shownBG(app *App, x, y int) Color { return app.screen.Buffer().Get(x, y).Style.BG }

func rowText(app *App, y, n int) string {
	s := make([]rune, 0, n)
	for x := 0; x < n; x++ {
		s = append(s, app.screen.Buffer().Get(x, y).Rune)
	}
	return string(s)
}

var hoverRed = Hex(0xff0000)

func TestHoverStylesTheTargetUnderThePointer(t *testing.T) {
	app := newHoverApp(12, 3, VBox(
		Jump(Text("go"), func() {}).Hover(Style{BG: hoverRed}),
		Text("plain"),
	))
	app.render()
	if shownBG(app, 0, 0) == hoverRed {
		t.Fatal("nothing hovers before the pointer has been seen")
	}
	app.handleMouse(motion(1, 0))
	app.render()
	if shownBG(app, 0, 0) != hoverRed || shownBG(app, 11, 0) != hoverRed {
		t.Fatal("a block target should be tinted across its row while hovered")
	}
	if shownBG(app, 0, 1) == hoverRed {
		t.Fatal("the row below the target must not be tinted")
	}
	app.handleMouse(motion(0, 1))
	app.render()
	if shownBG(app, 0, 0) == hoverRed {
		t.Fatal("moving off the target should clear the tint")
	}
}

func TestHoverRequestsAFrameOnlyWhenTheFootprintChanges(t *testing.T) {
	app := newHoverApp(12, 3, VBox(
		Tap(Text("one"), func() {}).Hover(Style{BG: hoverRed}),
		Text("plain"),
	))
	app.render()
	app.handleMouse(motion(1, 0))
	if !app.hoverFramePending.Load() {
		t.Fatal("entering a target should request a hover frame")
	}
	app.render()
	app.handleMouse(motion(5, 0))
	if app.hoverFramePending.Load() {
		t.Fatal("moving within the same target should not request a frame")
	}
	app.handleMouse(motion(5, 1))
	if !app.hoverFramePending.Load() {
		t.Fatal("leaving the target should request a frame to clear it")
	}
	app.render()
	app.handleMouse(motion(2, 1))
	if app.hoverFramePending.Load() {
		t.Fatal("moving over nothing should not request a frame")
	}
}

func TestHoverFrameRestylesTheCachedRenderWithoutExecuting(t *testing.T) {
	label := "old"
	app := newHoverApp(12, 2, VBox(
		Tap(Text(&label), func() {}).Hover(Style{BG: hoverRed}),
	))
	app.render()
	label = "new" // changed without RequestRender: a hover frame must not pick it up
	app.handleMouse(motion(0, 0))
	app.render()
	if got := rowText(app, 0, 3); got != "old" {
		t.Fatalf("hover frame re-executed the template: row reads %q", got)
	}
	if shownBG(app, 0, 0) != hoverRed {
		t.Fatal("hover frame should tint the target over the cached render")
	}
	app.RequestRender()
	app.render()
	if got := rowText(app, 0, 3); got != "new" || shownBG(app, 0, 0) != hoverRed {
		t.Fatalf("a full frame should show the new text, still hovered: %q", got)
	}
}

func TestListRowsHoverAcrossTheRow(t *testing.T) {
	rows := []string{"a", "b", "c"}
	sel := 0
	app := newHoverApp(10, 4, VBox(
		List(&rows).Selection(&sel).Hover(Style{BG: hoverRed}),
	))
	app.render()
	app.handleMouse(motion(6, 1))
	app.render()
	if shownBG(app, 0, 1) != hoverRed || shownBG(app, 9, 1) != hoverRed {
		t.Fatal("the hovered row should be tinted edge to edge")
	}
	if shownBG(app, 0, 0) == hoverRed || shownBG(app, 0, 2) == hoverRed {
		t.Fatal("neighbouring rows must not be tinted")
	}
	if sel != 0 {
		t.Fatal("hover must not move the selection")
	}
}

func TestHoverInsideScrollViewTranslatesAndClips(t *testing.T) {
	var sv *ScrollViewC
	svH := int16(3)
	lines := []string{"l0", "l1", "l2", "l3", "l4", "l5", "l6", "l7"}
	app := newHoverApp(10, 5, VBox(
		ScrollView.Height(&svH).Ref(func(s *ScrollViewC) { sv = s })(
			ForEach(&lines, func(l *string) Component {
				return TapItem(VBox.Height(2)(Text(l)), func(*string) {}).Hover(Style{BG: hoverRed})
			}),
		),
		Text("below"),
	))
	app.render()
	if sv == nil {
		t.Fatal("scrollview ref not set")
	}
	// l1 occupies content rows 2-3; only row 2 is inside the 3-row viewport
	app.handleMouse(motion(0, 2))
	app.render()
	if shownBG(app, 0, 2) != hoverRed {
		t.Fatal("the visible part of the hovered item should be tinted")
	}
	if shownBG(app, 0, 3) == hoverRed {
		t.Fatal("the tint must stop at the viewport edge, not spill onto the text below")
	}
	for i := 0; i < 2; i++ {
		app.handleMouse(wheel(0, 1, true))
	}
	if sv.Layer().ScrollY() != 2 {
		t.Fatalf("expected scroll 2, got %d", sv.Layer().ScrollY())
	}
	app.RequestRender()
	app.render()
	app.handleMouse(motion(0, 0))
	app.render()
	if shownBG(app, 0, 0) != hoverRed || shownBG(app, 0, 1) != hoverRed {
		t.Fatal("after scrolling, l1 sits at the top and both of its rows should be tinted")
	}
	if shownBG(app, 0, 2) == hoverRed {
		t.Fatal("the next item must not be tinted")
	}
}

func TestOverlayBlocksHoverBeneath(t *testing.T) {
	app := newHoverApp(20, 5, VBox(
		Tap(VBox.Height(5)(Text("under")), func() {}).Hover(Style{BG: hoverRed}),
		Overlay.Centered().Size(6, 3)(VBox(Text("dialog"))),
	))
	app.render()
	app.handleMouse(motion(10, 2))
	app.render()
	if shownBG(app, 0, 0) == hoverRed {
		t.Fatal("a pointer over the overlay must not hover the content under it")
	}
	app.handleMouse(motion(0, 0))
	app.render()
	if shownBG(app, 0, 0) != hoverRed {
		t.Fatal("a pointer beside the overlay still hovers the content")
	}
}

func TestRichTextSpanLinksHoverAcrossTheLink(t *testing.T) {
	linkHover := Style{Attr: AttrBold}
	spans := []Span{
		{Text: "read the "},
		{Text: "docs", OnSelect: func() {}, Hover: &linkHover},
		{Text: " now"},
	}
	app := newHoverApp(20, 2, VBox(Rich(spans)))
	app.render()
	app.handleMouse(motion(10, 0))
	app.render()
	buf := app.screen.Buffer()
	for x := 9; x < 13; x++ {
		if buf.Get(x, 0).Style.Attr&AttrBold == 0 {
			t.Fatalf("cell %d of the hovered link should be bold", x)
		}
	}
	if buf.Get(8, 0).Style.Attr&AttrBold != 0 || buf.Get(13, 0).Style.Attr&AttrBold != 0 {
		t.Fatal("text around the link must keep its own style")
	}
}

func TestHoverMixWashesExistingColoursAndReplacesBareOnes(t *testing.T) {
	blue := Hex(0x0000ff)
	app := newHoverApp(10, 2, VBox(
		Tap(VBox.Height(2)(
			VBox.Fill(blue)(Text("chip")),
			Text("bare"),
		), func() {}).Hover(Style{BG: hoverRed}, 0.5),
	))
	app.render()
	app.handleMouse(motion(0, 1))
	app.render()
	if got, want := shownBG(app, 0, 0), Lerp(blue, hoverRed, 0.5); got != want {
		t.Fatalf("a coloured cell should wash toward the hover colour: got %v want %v", got, want)
	}
	if got := shownBG(app, 0, 1); got != hoverRed {
		t.Fatalf("a cell with no colour of its own takes the hover colour: got %v", got)
	}
}

func TestTerminationSignalStopsTheAppAndEscalatesIfRunHangs(t *testing.T) {
	app := NewApp()
	app.nonInteractive = true // Stop must not close the test process's stdin
	app.running.Store(true)
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})
	escalated := make(chan os.Signal, 1)
	finished := make(chan struct{})
	old := terminationGrace
	terminationGrace = 10 * time.Millisecond
	defer func() { terminationGrace = old }()
	go func() {
		app.stopOnSignal(ch, done, func(s os.Signal) { escalated <- s })
		close(finished)
	}()
	ch <- syscall.SIGTERM
	<-finished
	if app.running.Load() {
		t.Fatal("SIGTERM should stop the app")
	}
	select {
	case s := <-escalated:
		if s != syscall.SIGTERM {
			t.Fatalf("escalated with %v, want SIGTERM", s)
		}
	default:
		t.Fatal("run never unwound, so the signal should have escalated to restore the terminal")
	}
}

func TestTerminationSignalDoesNotEscalateWhenRunUnwinds(t *testing.T) {
	app := NewApp()
	app.nonInteractive = true
	app.running.Store(true)
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})
	escalated := false
	finished := make(chan struct{})
	go func() {
		app.stopOnSignal(ch, done, func(os.Signal) { escalated = true })
		close(finished)
	}()
	ch <- syscall.SIGTERM
	close(done) // run returned inside the grace period
	<-finished
	if escalated {
		t.Fatal("a run that unwinds on its own restores the terminal itself")
	}
}
