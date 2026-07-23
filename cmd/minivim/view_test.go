package main

import (
	"strings"
	"testing"

	glyph "github.com/kungfusheep/glyph"
)

// rowText reads one rendered row back out of the buffer as plain text.
func rowText(buf *glyph.Buffer, y int) string {
	var sb strings.Builder
	for x := 0; x < buf.Width(); x++ {
		sb.WriteRune(buf.Get(x, y).Rune)
	}
	return sb.String()
}

// newTestEditor builds an editor with a single window over a small buffer,
// sized as if the terminal were w x h.
func newTestEditor(w, h int) *Editor {
	buf := &Buffer{
		Lines:    []string{"one", "two", "three", "four", "five"},
		FileName: "test.txt",
		marks:    make(map[rune]Pos),
	}
	ed := &Editor{Mode: "NORMAL"}
	win := ed.initPool(buf)
	ed.focusedWindow = win
	ed.root = &SplitNode{Window: win}
	ed.resize(w, h)
	return ed
}

// layerBox reports the viewport the engine handed the window's content layer on
// the last frame. The engine calls SetViewport on the layer during render, so
// this is the box the window actually occupies, not what the editor asked for.
func layerBox(w *Window) (int, int) {
	return w.contentLayer.ViewportWidth(), w.contentLayer.ViewportHeight()
}

// TestViewResizesWithoutRebuild is the guard against the resize path having to
// recompile. One template, executed at two terminal sizes, must size its
// windows from the box it is given each frame. Baking the viewport into the
// component at compile time freezes the first size forever, which is what
// forced a SetView call from OnResize.
func TestViewResizesWithoutRebuild(t *testing.T) {
	ed := newTestEditor(80, 24)
	ed.updateAllWindows()
	layerBefore := ed.win().contentLayer

	tmpl := glyph.Build(buildView(ed))

	out := glyph.NewBuffer(80, 24)
	tmpl.Execute(out, 80, 24)
	if got := rowText(out, 0); !strings.Contains(got, "one") {
		t.Fatalf("at 80x24 the first buffer line did not render; row 0 = %q", got)
	}
	_, h0 := layerBox(ed.win())

	// A real resize: the editor refreshes its scroll arithmetic and re-sizes the
	// layers, exactly as OnResize does. Then the SAME compiled template renders
	// the new terminal — nothing is rebuilt.
	ed.resize(120, 40)
	ed.updateAllWindows()

	if ed.win().contentLayer != layerBefore {
		t.Fatal("the resize replaced the window's Layer — the compiled template still holds the old " +
			"pointer, so the window would render stale content until the next recompile")
	}

	out = glyph.NewBuffer(120, 40)
	tmpl.Execute(out, 120, 40)

	w1, h1 := layerBox(ed.win())
	if w1 != 120 {
		t.Fatalf("after resize to 120x40 the window layer is %d wide — the view baked its width "+
			"at compile time and cannot resize without a rebuild", w1)
	}
	if h1 <= h0 {
		t.Fatalf("after resize the window layer height is %d, was %d — it did not grow into the "+
			"taller terminal", h1, h0)
	}
	if got := rowText(out, 0); !strings.Contains(got, "one") {
		t.Fatalf("after resize the window rendered no content; row 0 = %q", got)
	}
}

// TestVerticalSplitSharesWidth pins the split geometry: two windows side by
// side each take half the terminal, decided by the layout engine rather than by
// a width baked into each window at compile time.
func TestVerticalSplitSharesWidth(t *testing.T) {
	ed := newTestEditor(80, 24)
	ed.splitVertical()

	if ed.root.IsLeaf() {
		t.Fatal("splitVertical did not split the tree")
	}
	left := ed.root.Children[0].Window
	right := ed.root.Children[1].Window

	tmpl := glyph.Build(buildView(ed))
	tmpl.Execute(glyph.NewBuffer(80, 24), 80, 24)

	lw, _ := layerBox(left)
	rw, _ := layerBox(right)
	if lw+rw != 80 {
		t.Fatalf("the two windows cover %d columns (%d + %d), want the full 80", lw+rw, lw, rw)
	}
	if lw < 38 || lw > 42 {
		t.Fatalf("left window got %d columns, want roughly half of 80", lw)
	}
}

// TestSplitStillRendersAfterResize is the regression this whole change turns
// on. A split window must keep drawing its content when the terminal grows: if
// the window tree does not claim its vertical space, each pane collapses to its
// status bar and the editor renders one row of text over a blank screen.
func TestSplitStillRendersAfterResize(t *testing.T) {
	ed := newTestEditor(80, 24)
	ed.splitVertical()
	tmpl := glyph.Build(buildView(ed)) // the split recompiles, as rebuildView does

	ed.resize(120, 40)
	ed.updateAllWindows()

	out := glyph.NewBuffer(120, 40)
	tmpl.Execute(out, 120, 40)

	painted := 0
	for y := 0; y < 40; y++ {
		if strings.TrimSpace(rowText(out, y)) != "" {
			painted++
		}
	}
	// five buffer lines in each of two panes, plus two status bars and the
	// message line: comfortably more than the 3 rows a collapsed pane leaves.
	if painted < 6 {
		t.Fatalf("only %d rows painted after splitting and resizing — the panes collapsed to their "+
			"status bars instead of growing into the taller terminal", painted)
	}
}

// TestEditorGeometryMatchesEngine guards the seam between the two. The layout
// engine decides how big a window is; the editor keeps its own copy for paging
// and cursor clamping. If they disagree, nothing misdraws — the editor just
// pages by the wrong number of lines, which is the sort of bug you chase for an
// hour.
func TestEditorGeometryMatchesEngine(t *testing.T) {
	ed := newTestEditor(80, 24)
	ed.splitVertical()
	tmpl := glyph.Build(buildView(ed))

	ed.resize(120, 40)
	ed.updateAllWindows()
	tmpl.Execute(glyph.NewBuffer(120, 40), 120, 40)

	for i, n := range []*SplitNode{ed.root.Children[0], ed.root.Children[1]} {
		w := n.Window
		ew, eh := layerBox(w)
		if ew != w.viewportWidth || eh != w.viewportHeight {
			t.Errorf("window %d: engine laid it out %dx%d but the editor's scroll arithmetic says %dx%d",
				i, ew, eh, w.viewportWidth, w.viewportHeight)
		}
	}
}

// TestWindowLayerReclaimsMemory guards the cost of keeping the layer alive.
// EnsureSize only grows, so a window that reuses its layer would carry the
// largest document it ever showed for the rest of the session.
//
// The layer POINTER must survive — the compiled template holds it, and swapping
// it is the bug the reuse exists to prevent — while the buffer behind it shrinks.
func TestWindowLayerReclaimsMemory(t *testing.T) {
	ed := newTestEditor(80, 24)
	w := ed.focusedWindow

	big := make([]string, 100000)
	for i := range big {
		big[i] = "x"
	}
	w.buffer.Lines = big
	ed.initWindowLayer(w, 80)

	layerBefore := w.contentLayer
	if h := w.contentLayer.Buffer().Height(); h < 100000 {
		t.Fatalf("buffer is %d rows for a 100k-line file, want at least 100000", h)
	}

	// open a short file in the same window
	w.buffer.Lines = []string{"one", "two", "three"}
	w.renderedMin, w.renderedMax = -1, -1
	ed.initWindowLayer(w, 80)

	if got := w.contentLayer.Buffer().Height(); got > 1000 {
		t.Errorf("buffer still %d rows after opening a 3-line file — the window carries every document it ever showed", got)
	}
	if w.contentLayer != layerBefore {
		t.Error("the layer pointer changed — the compiled template is now stranded on the old layer")
	}

	// and a narrowed terminal reclaims the width
	ed.initWindowLayer(w, 400)
	ed.initWindowLayer(w, 80)
	if got := w.contentLayer.Buffer().Width(); got > 80 {
		t.Errorf("buffer still %d columns after narrowing to 80", got)
	}
	if w.contentLayer != layerBefore {
		t.Error("the layer pointer changed across a resize")
	}

	// the content must still paint after the buffer was swapped underneath
	buf := glyph.NewBuffer(80, 24)
	glyph.Build(buildView(ed)).Execute(buf, 80, 24)
	if !strings.Contains(rowText(buf, 0), "one") {
		t.Errorf("window is blank after the buffer swap; row0 = %q", rowText(buf, 0))
	}
}

// TestOneTemplateSurvivesSplits is the bar the pane pool exists to meet: ONE
// compiled template for the life of the session. It compiles before any split
// happens, then splits twice and closes once against that same template, and
// requires every window to draw its own content each time.
//
// This is the test that fails on the design it replaced. When the view was a
// recursive tree of VBox/HBox, the shape was fixed at compile time, so a split
// after Build was invisible until SetView recompiled — which is exactly the
// rebuild the editor is no longer allowed to do.
func TestOneTemplateSurvivesSplits(t *testing.T) {
	ed := newTestEditor(80, 24)
	ed.updateAllWindows()

	// compiled ONCE, before the tree ever changes shape
	tmpl := glyph.Build(buildView(ed))

	paint := func() string {
		out := glyph.NewBuffer(80, 24)
		tmpl.Execute(out, 80, 24)
		var sb strings.Builder
		for y := 0; y < 24; y++ {
			sb.WriteString(rowText(out, y))
			sb.WriteByte('\n')
		}
		return sb.String()
	}

	// one window: its content is on screen
	if !strings.Contains(paint(), "one") {
		t.Fatal("the single window rendered no content before any split")
	}

	// split vertically — no recompile
	ed.splitVertical()
	left, right := ed.root.Children[0].Window, ed.root.Children[1].Window
	if left == right {
		t.Fatal("splitVertical reused the same window for both panes")
	}
	frame := paint()
	lw, _ := layerBox(left)
	rw, _ := layerBox(right)
	if lw == 0 || rw == 0 {
		t.Fatalf("after a split with no recompile a pane has no width (left %d, right %d) — "+
			"the template did not pick up the new tree", lw, rw)
	}
	if lw+rw != 80 {
		t.Errorf("panes cover %d columns (%d + %d), want 80", lw+rw, lw, rw)
	}
	// both panes show the buffer, so the text appears twice on the first row
	if n := strings.Count(strings.SplitN(frame, "\n", 2)[0], "one"); n != 2 {
		t.Errorf("row 0 shows %d panes of content, want 2: %q", n, strings.SplitN(frame, "\n", 2)[0])
	}

	// split again — still no recompile
	ed.splitHorizontal()
	if got := len(ed.root.AllWindows()); got != 3 {
		t.Fatalf("after a second split the tree holds %d windows, want 3", got)
	}
	for _, w := range ed.root.AllWindows() {
		if w.viewportWidth == 0 || w.viewportHeight == 0 {
			t.Errorf("window in slot %d has an empty viewport after the second split", w.slot)
		}
	}
	if !strings.Contains(paint(), "one") {
		t.Error("content vanished after the second split")
	}

	// closing returns the slot, and the survivors reclaim the space
	ed.closeWindow()
	if got := len(ed.root.AllWindows()); got != 2 {
		t.Fatalf("after closeWindow the tree holds %d windows, want 2", got)
	}
	if !strings.Contains(paint(), "one") {
		t.Error("content vanished after closing a window")
	}
}

// TestPoolExhaustionIsReported pins the one cost of the compile-once design:
// the pool is finite, so the (maxWindows+1)th split has to fail visibly rather
// than silently drop a window or corrupt the tree.
func TestPoolExhaustionIsReported(t *testing.T) {
	ed := newTestEditor(200, 80)
	for i := 0; i < maxWindows*2; i++ {
		ed.splitVertical()
	}
	if got := len(ed.root.AllWindows()); got != maxWindows {
		t.Fatalf("tree holds %d windows, want the pool cap of %d", got, maxWindows)
	}
	if ed.StatusLine == "" {
		t.Error("splitting past the pool cap reported nothing to the user")
	}
	// every window still owns a distinct slot
	seen := map[int]bool{}
	for _, w := range ed.root.AllWindows() {
		if seen[w.slot] {
			t.Fatalf("slot %d is used by two windows at once", w.slot)
		}
		seen[w.slot] = true
	}
}

// TestRefusedSplitLeavesViewportWhole pins the claim-before-mutate ordering: when the
// pane pool is drained, a split is refused (E36), and the focused window must keep its
// full viewport. The bug was halving viewport* before claimWindow, so a nil claim left
// the scroll arithmetic halved while the tree was unchanged and the layout drew the full
// rect. Measured at 120x400, pool drained: viewportHeight 398 -> 199 with the tree intact.
func TestRefusedSplitLeavesViewportWhole(t *testing.T) {
	ed := newTestEditor(120, 400)
	ed.updateAllWindows()
	ed.free = ed.free[:0] // drain the pool so the next claim fails

	h := ed.focusedWindow.viewportHeight
	if h <= 1 {
		t.Fatalf("setup: viewportHeight %d too small to detect a halving", h)
	}
	ed.SplitHoriz()
	if ed.StatusLine == "" {
		t.Error("refused horizontal split reported nothing to the user")
	}
	if got := ed.focusedWindow.viewportHeight; got != h {
		t.Errorf("refused horizontal split changed viewportHeight %d -> %d; claim must precede mutation", h, got)
	}

	w := ed.focusedWindow.viewportWidth
	if w <= 1 {
		t.Fatalf("setup: viewportWidth %d too small to detect a halving", w)
	}
	ed.SplitVert()
	if got := ed.focusedWindow.viewportWidth; got != w {
		t.Errorf("refused vertical split changed viewportWidth %d -> %d; claim must precede mutation", w, got)
	}
}
