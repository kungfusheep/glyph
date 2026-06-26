package glyph

import (
	"sync"
	"testing"
	"time"
)

// TestLayerScrollConcurrentRenderNoRace guards the Layer scroll-state race:
// render() drives SetViewport→updateMaxScroll (writing
// scrollY/maxScroll/viewport) while an input handler calls ScrollTo/ScrollDown on
// the same Layer. With no sync those tear; this drives both goroutines. Run
// under -race. Also a deadlock guard: the layer's Render callback calls back into
// ScrollY()/ScrollTo() (a consumer's diff/scroll layer pattern), so the scroll lock must
// not be held across Render().
func TestLayerScrollConcurrentRenderNoRace(t *testing.T) {
	layer := NewLayer()
	layer.ShowCursor()
	layer.SetCursor(0, 12) // visible cursor → render() consults ScreenCursor (reads scrollY/viewHeight)
	layer.Render = func() {
		// consumer pattern: read+restore scroll across a re-render (renderDiffLayer)
		y := layer.ScrollY()
		layer.SetBuffer(NewBuffer(layer.ViewportWidth(), 40)) // content taller than viewport → maxScroll>0
		layer.ScrollTo(y)
	}
	tmpl := Build(VBox(LayerView(layer).Grow(1)))
	app := newEffectTestApp(tmpl, 20, 8)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 3000; i++ {
			app.render()         // SetViewport→updateMaxScroll + prepare→Render + blit(scrollY)
			layer.ScreenCursor() // render goroutine reads scrollY/viewHeight (c865) — must be guarded
		}
		close(done)
	}()
	for i := 0; i < 3000; i++ {
		layer.ScrollDown(2) // input goroutine: reads scrollY/maxScroll/viewHeight, writes scrollY
		layer.ScrollTo(0)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: render()/Render-callback re-entered the scroll lock")
	}
}

func TestLayerBlit(t *testing.T) {
	t.Run("single layer blits to correct position", func(t *testing.T) {
		// Create a layer with content
		layer := NewLayer()
		layerBuf := NewBuffer(10, 5)
		for y := 0; y < 5; y++ {
			layerBuf.WriteStringFast(0, y, string(rune('A'+y))+"----", Style{}, 10)
		}
		layer.SetBuffer(layerBuf)

		// Create screen and view
		screen := NewBuffer(20, 10)

		// Build view with layer at position
		view := VBox(
			Text("Header"),
			LayerView(layer).Height(3),
			Text("Footer"),
		)

		tmpl := Build(view)
		tmpl.Execute(screen, 20, 10)

		// Verify header at line 0
		if got := screen.GetLine(0); got != "Header" {
			t.Errorf("line 0: got %q, want %q", got, "Header")
		}

		// Verify layer content at lines 1-3
		if got := screen.GetLine(1); got != "A----" {
			t.Errorf("line 1: got %q, want %q", got, "A----")
		}
		if got := screen.GetLine(2); got != "B----" {
			t.Errorf("line 2: got %q, want %q", got, "B----")
		}
		if got := screen.GetLine(3); got != "C----" {
			t.Errorf("line 3: got %q, want %q", got, "C----")
		}

		// Verify footer at line 4
		if got := screen.GetLine(4); got != "Footer" {
			t.Errorf("line 4: got %q, want %q", got, "Footer")
		}
	})

	t.Run("multiple layers blit to correct positions", func(t *testing.T) {
		// Create first layer
		layer1 := NewLayer()
		buf1 := NewBuffer(10, 5)
		for y := 0; y < 5; y++ {
			buf1.WriteStringFast(0, y, "111111", Style{}, 10)
		}
		layer1.SetBuffer(buf1)

		// Create second layer
		layer2 := NewLayer()
		buf2 := NewBuffer(10, 5)
		for y := 0; y < 5; y++ {
			buf2.WriteStringFast(0, y, "222222", Style{}, 10)
		}
		layer2.SetBuffer(buf2)

		// Create third layer
		layer3 := NewLayer()
		buf3 := NewBuffer(10, 5)
		for y := 0; y < 5; y++ {
			buf3.WriteStringFast(0, y, "333333", Style{}, 10)
		}
		layer3.SetBuffer(buf3)

		screen := NewBuffer(20, 15)

		view := VBox(
			Text("=TOP="),
			LayerView(layer1).Height(2),
			Text("=MID1="),
			LayerView(layer2).Height(2),
			Text("=MID2="),
			LayerView(layer3).Height(2),
			Text("=BOT="),
		)

		tmpl := Build(view)
		tmpl.Execute(screen, 20, 15)

		expected := []struct {
			line int
			want string
		}{
			{0, "=TOP="},
			{1, "111111"},
			{2, "111111"},
			{3, "=MID1="},
			{4, "222222"},
			{5, "222222"},
			{6, "=MID2="},
			{7, "333333"},
			{8, "333333"},
			{9, "=BOT="},
		}

		for _, tc := range expected {
			if got := screen.GetLine(tc.line); got != tc.want {
				t.Errorf("line %d: got %q, want %q", tc.line, got, tc.want)
			}
		}
	})

	t.Run("layers scroll independently", func(t *testing.T) {
		// Create two layers with different content
		layer1 := NewLayer()
		buf1 := NewBuffer(10, 10)
		for y := 0; y < 10; y++ {
			buf1.WriteStringFast(0, y, string(rune('A'+y))+"AAAA", Style{}, 10)
		}
		layer1.SetBuffer(buf1)

		layer2 := NewLayer()
		buf2 := NewBuffer(10, 10)
		for y := 0; y < 10; y++ {
			buf2.WriteStringFast(0, y, string(rune('0'+y))+"0000", Style{}, 10)
		}
		layer2.SetBuffer(buf2)

		screen := NewBuffer(20, 10)

		view := VBox(
			LayerView(layer1).Height(3),
			Text("---"),
			LayerView(layer2).Height(3),
		)

		tmpl := Build(view)

		// Initial render - both at scroll 0
		tmpl.Execute(screen, 20, 10)

		if got := screen.GetLine(0); got != "AAAAA" {
			t.Errorf("initial layer1 line 0: got %q, want %q", got, "AAAAA")
		}
		if got := screen.GetLine(4); got != "00000" {
			t.Errorf("initial layer2 line 4: got %q, want %q", got, "00000")
		}

		// Scroll layer1 down by 2
		layer1.ScrollDown(2)
		tmpl.Execute(screen, 20, 10)

		// Layer1 should now show C, D, E (indices 2, 3, 4)
		if got := screen.GetLine(0); got != "CAAAA" {
			t.Errorf("after scroll layer1 line 0: got %q, want %q", got, "CAAAA")
		}
		if got := screen.GetLine(1); got != "DAAAA" {
			t.Errorf("after scroll layer1 line 1: got %q, want %q", got, "DAAAA")
		}

		// Layer2 should still be at scroll 0
		if got := screen.GetLine(4); got != "00000" {
			t.Errorf("layer2 should be unchanged: got %q, want %q", got, "00000")
		}

		// Now scroll layer2
		layer2.ScrollDown(5)
		tmpl.Execute(screen, 20, 10)

		// Layer2 should now show 5, 6, 7
		if got := screen.GetLine(4); got != "50000" {
			t.Errorf("after scroll layer2 line 4: got %q, want %q", got, "50000")
		}

		// Layer1 should still be at its scroll position
		if got := screen.GetLine(0); got != "CAAAA" {
			t.Errorf("layer1 should be unchanged: got %q, want %q", got, "CAAAA")
		}
	})

	t.Run("layer with nil buffer renders empty", func(t *testing.T) {
		layer := NewLayer()
		// Don't set any buffer

		screen := NewBuffer(20, 5)

		view := VBox(
			Text("Before"),
			LayerView(layer).Height(2),
			Text("After"),
		)

		tmpl := Build(view)
		screen.Clear()
		tmpl.Execute(screen, 20, 5)

		// Text should render - key is it shouldn't crash with nil buffer
		if got := screen.GetLine(0); got != "Before" {
			t.Errorf("line 0: got %q, want %q", got, "Before")
		}

		// After should be at line 3 (0=Before, 1-2=layer, 3=After)
		if got := screen.GetLine(3); got != "After" {
			t.Errorf("line 3: got %q, want %q", got, "After")
		}
	})

	t.Run("layer inside bordered container", func(t *testing.T) {
		layer := NewLayer()
		layerBuf := NewBuffer(30, 5)
		for y := 0; y < 5; y++ {
			layerBuf.WriteStringFast(0, y, string(rune('A'+y))+"----line", Style{}, 30)
		}
		layer.SetBuffer(layerBuf)

		screen := NewBuffer(40, 10)

		view := VBox(
			VBox.Border(BorderSingle).Title("Content")(
				LayerView(layer).Height(3),
			),
		)

		tmpl := Build(view)
		tmpl.Execute(screen, 40, 10)

		line0 := screen.GetLine(0)
		if !contains(line0, "Content") {
			t.Errorf("line 0 should have title: got %q", line0)
		}

		line1 := screen.GetLine(1)
		if !contains(line1, "A----line") {
			t.Errorf("line 1 should contain layer content: got %q", line1)
		}

		line4 := screen.GetLine(4)
		if !contains(line4, "└") && !contains(line4, "─") {
			t.Errorf("line 4 should have bottom border: got %q", line4)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLayerScrollBounds(t *testing.T) {
	t.Run("scroll clamps to bounds", func(t *testing.T) {
		layer := NewLayer()
		buf := NewBuffer(10, 20) // 20 lines of content
		layer.SetBuffer(buf)
		layer.SetViewport(10, 5) // 5 line viewport

		// MaxScroll should be 20 - 5 = 15
		if got := layer.MaxScroll(); got != 15 {
			t.Errorf("MaxScroll: got %d, want 15", got)
		}

		// Scroll past end should clamp
		layer.ScrollTo(100)
		if got := layer.ScrollY(); got != 15 {
			t.Errorf("ScrollY after overflow: got %d, want 15", got)
		}

		// Scroll before start should clamp
		layer.ScrollTo(-10)
		if got := layer.ScrollY(); got != 0 {
			t.Errorf("ScrollY after underflow: got %d, want 0", got)
		}
	})

	t.Run("page scroll methods", func(t *testing.T) {
		layer := NewLayer()
		buf := NewBuffer(10, 100)
		layer.SetBuffer(buf)
		layer.SetViewport(10, 10)

		layer.PageDown()
		if got := layer.ScrollY(); got != 10 {
			t.Errorf("after PageDown: got %d, want 10", got)
		}

		layer.PageUp()
		if got := layer.ScrollY(); got != 0 {
			t.Errorf("after PageUp: got %d, want 0", got)
		}

		layer.HalfPageDown()
		if got := layer.ScrollY(); got != 5 {
			t.Errorf("after HalfPageDown: got %d, want 5", got)
		}

		layer.ScrollToEnd()
		if got := layer.ScrollY(); got != 90 {
			t.Errorf("after ScrollToEnd: got %d, want 90", got)
		}

		layer.ScrollToTop()
		if got := layer.ScrollY(); got != 0 {
			t.Errorf("after ScrollToTop: got %d, want 0", got)
		}
	})
}

// BenchmarkLayerWithCursor measures rendering a layer with cursor tracking.
func BenchmarkLayerWithCursor(b *testing.B) {
	layer := NewLayer()
	buf := NewBuffer(80, 100)
	for y := 0; y < 100; y++ {
		buf.WriteStringFast(0, y, "Line content here", Style{}, 80)
	}
	layer.SetBuffer(buf)
	layer.SetViewport(80, 24)
	layer.ShowCursor()
	layer.SetCursorStyle(CursorBlock)

	screen := NewBuffer(80, 24)

	view := VBox(LayerView(layer).Height(24))
	tmpl := Build(view)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// simulate cursor movement each frame
		layer.SetCursor(i%80, (i/80)%100)
		screen.ClearDirty()
		tmpl.Execute(screen, 80, 24)
	}
}

// BenchmarkLayerScrollingWithCursor measures scrolling + cursor updates.
func BenchmarkLayerScrollingWithCursor(b *testing.B) {
	layer := NewLayer()
	buf := NewBuffer(80, 1000)
	for y := 0; y < 1000; y++ {
		buf.WriteStringFast(0, y, "Line content that we scroll through", Style{}, 80)
	}
	layer.SetBuffer(buf)
	layer.SetViewport(80, 24)
	layer.ShowCursor()

	screen := NewBuffer(80, 24)

	view := VBox(LayerView(layer).Height(24))
	tmpl := Build(view)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		layer.ScrollTo(i % 976) // scroll within bounds
		layer.SetCursor(i%80, layer.ScrollY()+(i%24))
		screen.ClearDirty()
		tmpl.Execute(screen, 80, 24)
	}
}

// BenchmarkLayerCursorScreenTranslation measures ScreenCursor() translation.
func BenchmarkLayerCursorScreenTranslation(b *testing.B) {
	layer := NewLayer()
	layer.SetViewport(80, 24)
	layer.ShowCursor()

	// simulate being positioned at screen offset
	layer.screenX = 10
	layer.screenY = 5

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		layer.SetCursor(i%80, i%100)
		layer.scrollY = (i / 10) % 50
		_, _, _ = layer.ScreenCursor()
	}
}

// TestLayerReRendersOnHeightChange guards the Layer strand (#341): NeedsRender
// must re-render when the viewport HEIGHT changes (a pane regrows / relayouts),
// not only on width. Without it, a stale l.buffer blits consistently (front==back,
// so the screen diff is blind to it) and the lower rows persist stale until the
// next Invalidate. lastRenderHeight was tracked in prepare() but never compared.
func TestLayerReRendersOnHeightChange(t *testing.T) {
	l := NewLayer()
	renders := 0
	l.Render = func() { renders++ }

	l.SetViewport(20, 5)
	l.prepare()
	if renders != 1 {
		t.Fatalf("first prepare should render once, got %d", renders)
	}

	// height grows (relayout) — must trigger a re-render
	l.SetViewport(20, 9)
	if !l.NeedsRender() {
		t.Fatal("NeedsRender false after a height change — stale buffer would blit (the strand)")
	}
	l.prepare()
	if renders != 2 {
		t.Fatalf("prepare should re-render after a height change, renders=%d", renders)
	}

	// no change → no re-render (don't over-render)
	if l.NeedsRender() {
		t.Fatal("NeedsRender true with no dimension change — would over-render every frame")
	}

	// width change still re-renders (control)
	l.SetViewport(30, 9)
	if !l.NeedsRender() {
		t.Fatal("width change should still re-render")
	}
}

// Feather adds work only at the overflowing edges and only when enabled; the off-path
// (feather 0) must match plain blit. These two benchmarks prove no per-frame regression
// off-path and bounded edge cost on-path.
func benchmarkLayerBlit(b *testing.B, feather int) {
	l := NewLayer()
	l.feather = feather
	l.defaultStyle = Style{BG: RGB(0, 0, 0)}
	src := NewBuffer(80, 400)
	for y := 0; y < 400; y++ {
		for x := 0; x < 80; x++ {
			src.SetFast(x, y, Cell{Rune: 'x', Style: Style{FG: RGB(200, 200, 200)}})
		}
	}
	l.SetBuffer(src)
	l.SetViewport(80, 40)
	l.ScrollTo(180) // mid-scroll: both edges feather
	dst := NewBuffer(80, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.blit(dst, 0, 0, 80, 40)
	}
}

func BenchmarkLayerBlit(b *testing.B)        { benchmarkLayerBlit(b, 0) }
func BenchmarkLayerBlitFeather(b *testing.B) { benchmarkLayerBlit(b, 3) }

// boundRead locks and returns the displayed (eased) offset blit would draw at.
func boundRead(l *Layer) int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	return l.displayedOffsetLocked()
}

// A bound offset with no easing is the single source of truth: scroll methods write it,
// the displayed offset tracks it instantly, and a stale programmatic scroll cannot
// clobber a later manual one (ADR 38 pending-vs-manual dissolved by construction).
func TestLayerBoundOffsetInstant(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100)) // content height 100
	l.SetViewport(10, 10)           // maxScroll = 90
	target := 0
	l.scrollTarget = &target

	l.ScrollTo(40)
	if got := boundRead(l); got != 40 || target != 40 {
		t.Fatalf("ScrollTo(40): displayed=%d target=%d, want 40/40", got, target)
	}
	l.HalfPageDown() // +5 from 40
	if target != 45 {
		t.Errorf("HalfPageDown from 40: target=%d, want 45", target)
	}
	l.ScrollToEnd()
	if got := boundRead(l); got != 90 {
		t.Errorf("ScrollToEnd: displayed=%d, want 90 (maxScroll)", got)
	}

	// pending-vs-manual: a stale programmatic scroll-to-bottom, then a manual scroll —
	// the manual wins; there is no separate pending slot to re-apply and yank back.
	l.ScrollTo(1 << 30) // programmatic "to bottom" (clamps to 90)
	l.ScrollTo(12)      // manual scroll lands at 12
	if got := boundRead(l); got != 12 {
		t.Fatalf("manual scroll after programmatic: displayed=%d, want 12 (no stale clobber)", got)
	}
}

// A bound offset with an Animate eases the displayed value toward the target over the
// duration; the logical position (ScrollY) is the target immediately.
func TestLayerBoundOffsetAnimates(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10) // maxScroll 90
	target := 0
	l.scrollTarget = &target
	l.scrollEaseDur = 100 * time.Millisecond
	l.scrollEaseFn = EaseLinear
	clock := time.Unix(1000, 0)
	l.nowFn = func() time.Time { return clock }

	_ = boundRead(l) // first read establishes shown=0 at target 0
	target = 80      // retarget; ScrollY is the destination immediately
	if got := l.ScrollY(); got != 80 {
		t.Fatalf("ScrollY after retarget = %d, want 80 (logical target)", got)
	}
	// t0: ease begins, displayed still ~0
	if got := boundRead(l); got != 0 {
		t.Errorf("ease start: displayed=%d, want 0", got)
	}
	clock = clock.Add(50 * time.Millisecond) // halfway, linear
	if got := boundRead(l); got != 40 {
		t.Errorf("ease halfway: displayed=%d, want 40", got)
	}
	clock = clock.Add(50 * time.Millisecond) // complete
	if got := boundRead(l); got != 80 {
		t.Errorf("ease complete: displayed=%d, want 80", got)
	}
	if l.scrollAnimating {
		t.Error("animation should be settled at the target")
	}
}

// The grow-guard: a target clamped to maxScroll must not snap when content later grows.
func TestLayerBoundOffsetGrowGuard(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 20)) // content 20
	l.SetViewport(10, 10)          // maxScroll 10
	target := 0
	l.scrollTarget = &target

	l.ScrollTo(1 << 30) // to bottom -> clamps to 10, and writes the clamp back
	if target != 10 {
		t.Fatalf("target after clamp = %d, want 10 (written back, not left huge)", target)
	}
	// content grows; maxScroll rises. A stale out-of-range target would snap to the new
	// max — but it was clamped to 10, so it stays at 10.
	l.SetBuffer(NewBuffer(10, 200)) // content 200
	l.SetViewport(10, 10)           // maxScroll 190
	if got := boundRead(l); got != 10 {
		t.Errorf("after content grow: displayed=%d, want 10 (no snap to new max)", got)
	}
}

// The bound-offset method path is concurrency-safe: scroll methods write the target and
// blit reads the displayed offset, both under scrollMu. Must be clean under -race.
func TestLayerBoundOffsetConcurrent(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 1000))
	l.SetViewport(10, 10)
	target := 0
	l.scrollTarget = &target

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); l.ScrollTo(i * 7) }(i)
		go func() { defer wg.Done(); _ = boundRead(l) }()
	}
	wg.Wait()
}

// The per-frame displayed-offset read must be alloc-free (ADR 38 perf requirement).
func BenchmarkLayerBoundOffsetRead(b *testing.B) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)
	target := 45
	l.scrollTarget = &target // instant path = the steady-state read
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boundRead(l)
	}
}
