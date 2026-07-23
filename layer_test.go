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

// SetFeather exposes scroll-overflow feathering on a raw Layer (the common case is a
// layer view, not only ScrollView): the setter drives the same blit fade, and the
// off-path (0) is untouched. Getter mirrors the setter.
func TestLayerSetFeather(t *testing.T) {
	build := func(feather int) *Buffer {
		l := NewLayer()
		l.SetFeather(feather)
		l.defaultStyle = Style{BG: RGB(0, 0, 0)} // a real bg to blend toward
		if got := l.Feather(); got != feather {
			t.Fatalf("Feather() = %d, want %d", got, feather)
		}
		src := NewBuffer(10, 100)
		for y := 0; y < 100; y++ {
			for x := 0; x < 10; x++ {
				src.SetFast(x, y, Cell{Rune: 'x', Style: Style{FG: RGB(200, 200, 200)}})
			}
		}
		l.SetBuffer(src)
		l.SetViewport(10, 20)
		l.ScrollTo(40) // mid-scroll: top overflows, so the top edge should fade
		dst := NewBuffer(10, 20)
		l.blit(dst, 0, 0, 10, 20)
		return dst
	}

	plain := build(0)
	feathered := build(3)

	// off-path: feather 0 leaves the top row at full source intensity.
	if r := plain.Get(0, 0).Style.FG.R; r != 200 {
		t.Fatalf("feather 0 must not fade: top-row FG.R = %d, want 200", r)
	}
	// on-path: feather 3 fades the top row toward the (black) background.
	if r := feathered.Get(0, 0).Style.FG.R; r >= 200 {
		t.Fatalf("feather 3 must fade the overflowing top edge: top-row FG.R = %d, want < 200", r)
	}
	// the middle is untouched either way.
	if feathered.Get(0, 10).Style.FG.R != 200 {
		t.Fatalf("feather must not touch the middle band: FG.R = %d, want 200", feathered.Get(0, 10).Style.FG.R)
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
	l.ease.target = &target

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
	l.ease.target = &target
	l.ease.dur = 100 * time.Millisecond
	l.ease.fn = EaseLinear
	clock := time.Unix(1000, 0)
	l.ease.nowFn = func() time.Time { return clock }

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
	if l.ease.animating {
		t.Error("animation should be settled at the target")
	}
}

// The grow-guard: a target clamped to maxScroll must not snap when content later grows.
func TestLayerBoundOffsetGrowGuard(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 20)) // content 20
	l.SetViewport(10, 10)          // maxScroll 10
	target := 0
	l.ease.target = &target

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
	l.ease.target = &target

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
	l.ease.target = &target // instant path = the steady-state read
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boundRead(l)
	}
}

// Feather fades content toward the background it actually sits on (the destination
// cell's BG), not the layer's default style. Regression: with content on a grey panel
// but a blue default style, the edge was fading toward blue (the wrong colour).
func TestLayerFeatherFadesTowardDstBackground(t *testing.T) {
	l := NewLayer()
	l.SetFeather(3)
	l.defaultStyle = Style{BG: RGB(0, 0, 255)} // deliberately WRONG (blue) — must not be the fade target

	src := NewBuffer(10, 100)
	for y := 0; y < 100; y++ {
		for x := 0; x < 10; x++ {
			src.SetFast(x, y, Cell{Rune: 'x', Style: Style{FG: RGB(255, 255, 255)}}) // white text, default BG
		}
	}
	l.SetBuffer(src)
	l.SetViewport(10, 20)
	l.ScrollTo(40) // top overflows → top edge fades

	// dst pre-filled with the GREY panel the content sits on.
	dst := NewBuffer(10, 20)
	for y := 0; y < 20; y++ {
		for x := 0; x < 10; x++ {
			dst.SetFast(x, y, Cell{Rune: ' ', Style: Style{BG: RGB(128, 128, 128)}})
		}
	}
	l.blit(dst, 0, 0, 10, 20)

	fg := dst.Get(0, 0).Style.FG // top edge: white faded toward its background
	// toward grey keeps R==G==B (both endpoints are neutral); toward blue would leave B high, R/G low.
	if !(fg.R == fg.G && fg.G == fg.B) {
		t.Errorf("edge FG %+v is not neutral — faded toward a coloured default, not the grey background", fg)
	}
	if fg.B >= 255 {
		t.Errorf("edge FG faded toward the blue default (B=%d); expected fade toward the grey dst background", fg.B)
	}
	if !(fg.R > 128 && fg.R < 255) {
		t.Errorf("edge FG.R=%d, expected between grey(128) and white(255)", fg.R)
	}
}

// Arming through the mount is the ADR 137 surface. The pin the ADR names: an unarmed
// Layer behaves exactly as before, so mounting a bare LayerView cannot change a view.
func TestLayerViewScrollOffsetUnarmedUnchanged(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)
	l.ScrollTo(30)

	Build(VBox(LayerView(l).Grow(1))) // no ScrollOffset — must not arm
	if l.ease.target != nil {
		t.Fatalf("mounting a bare LayerView armed the layer: ease.target=%p", l.ease.target)
	}
	if got, want := l.ScrollY(), 30; got != want {
		t.Errorf("ScrollY after bare mount = %d, want %d", got, want)
	}
	if got, want := l.scrollY, 30; got != want {
		t.Errorf("legacy field after bare mount = %d, want %d", got, want)
	}
}

// Arming binds the cell and routes scrolling through it, whether declared on a bare
// LayerView or by ScrollView — both compile to the same arming site.
func TestLayerViewScrollOffsetArms(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)

	cell := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(cell).Grow(1)))
	if l.ease.target != cell {
		t.Fatalf("ScrollOffset did not bind the cell: got %p want %p", l.ease.target, cell)
	}
	l.ScrollTo(40)
	if *cell != 40 || boundRead(l) != 40 {
		t.Errorf("ScrollTo(40): cell=%d displayed=%d, want 40/40", *cell, boundRead(l))
	}
}

// Arming an already-scrolled layer must hold its place. The cell is seeded from the
// position in effect, so a pane that mounts after scrolling doesn't snap to the top.
func TestLayerViewScrollOffsetSeedsFromCurrentPosition(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)
	l.ScrollTo(12) // unarmed: writes the legacy field

	cell := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(cell).Grow(1)))
	if *cell != 12 {
		t.Errorf("arming a layer scrolled to 12 seeded cell=%d, want 12", *cell)
	}
	if got := l.ScrollY(); got != 12 {
		t.Errorf("ScrollY after arming = %d, want 12 (position dropped)", got)
	}
}

// Rebuilding re-runs compile, so the same cell is re-armed every frame the view is
// rebuilt. That must be a no-op: the guard is on pointer identity, because a live cell
// and a fresh one are indistinguishable by value.
func TestLayerViewScrollOffsetRearmIsNoOp(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)

	cell := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(cell).Grow(1)))
	l.ScrollTo(40)

	Build(VBox(LayerView(l).ScrollOffset(cell).Grow(1))) // rebuild, same cell
	if *cell != 40 {
		t.Errorf("re-arm clobbered the live cell: %d, want 40", *cell)
	}
	if got := l.ScrollY(); got != 40 {
		t.Errorf("ScrollY after rebuild = %d, want 40", got)
	}

	// a genuinely different cell re-seeds from the position in effect
	other := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(other).Grow(1)))
	if *other != 40 {
		t.Errorf("swapped cell seeded %d, want 40", *other)
	}
}

// An Animate over the cell carries duration and easing; a plain *int is instant. Both
// spellings arm the same cell, and re-spelling config doesn't move the position.
func TestLayerViewScrollOffsetAnimateCarriesConfig(t *testing.T) {
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, 100))
	l.SetViewport(10, 10)

	cell := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(Animate.Duration(200 * time.Millisecond)(cell)).Grow(1)))
	if l.ease.target != cell {
		t.Fatalf("Animate did not bind the cell")
	}
	if l.ease.dur != 200*time.Millisecond {
		t.Errorf("ease.dur = %v, want 200ms", l.ease.dur)
	}
}

// easedLayer is an armed layer on a fake clock, for the ADR 137 derived-reader pins.
func easedLayer(t *testing.T, contentH int) (*Layer, *int, func(time.Duration)) {
	t.Helper()
	l := NewLayer()
	l.SetBuffer(NewBuffer(10, contentH))
	l.SetViewport(10, 10)
	cell := ScrollState()
	Build(VBox(LayerView(l).ScrollOffset(Animate.Duration(100 * time.Millisecond).Ease(EaseLinear)(cell)).Grow(1)))
	clock := time.Unix(1000, 0)
	l.ease.nowFn = func() time.Time { return clock }
	return l, cell, func(d time.Duration) { clock = clock.Add(d) }
}

// ScrollY is the destination; DisplayedScrollY is where the content actually is. Mid-ease
// they differ — that gap is the whole reason the accessor exists, since "which rows are
// visible" is otherwise publicly uncomputable while a glide is in flight.
func TestLayerDisplayedScrollYLagsTargetMidEase(t *testing.T) {
	l, _, advance := easedLayer(t, 100) // maxScroll 90

	_ = boundRead(l) // establish shown at 0
	l.ScrollTo(80)
	if got := l.ScrollY(); got != 80 {
		t.Fatalf("ScrollY = %d, want 80 (destination)", got)
	}
	_ = boundRead(l) // the blit after a retarget is what starts the ease, at t0
	if got := l.DisplayedScrollY(); got != 0 {
		t.Errorf("DisplayedScrollY at ease start = %d, want 0", got)
	}
	advance(50 * time.Millisecond)
	_ = boundRead(l) // blit drives the ease
	if got, want := l.DisplayedScrollY(), 40; got != want {
		t.Errorf("DisplayedScrollY halfway = %d, want %d", got, want)
	}
	advance(50 * time.Millisecond)
	_ = boundRead(l)
	if got := l.DisplayedScrollY(); got != 80 {
		t.Errorf("DisplayedScrollY settled = %d, want 80", got)
	}
	if got := l.ScrollY(); got != 80 {
		t.Errorf("ScrollY settled = %d, want 80", got)
	}
}

// The accessor is an OBSERVER: reading it must not start or advance an ease, or a
// consumer polling from the input goroutine would drive animation timing.
func TestLayerDisplayedScrollYDoesNotDriveTheEase(t *testing.T) {
	l, _, advance := easedLayer(t, 100)

	_ = boundRead(l)
	l.ScrollTo(80)
	_ = boundRead(l) // ease starts at t0
	advance(50 * time.Millisecond)

	shownBefore, t0Before := l.ease.shown, l.ease.animT0
	for i := 0; i < 5; i++ {
		if got := l.DisplayedScrollY(); got != 0 {
			t.Fatalf("read %d moved the displayed offset to %d without a blit", i, got)
		}
	}
	if l.ease.shown != shownBefore || l.ease.animT0 != t0Before {
		t.Errorf("reading the accessor mutated ease state: shown %v→%v, t0 %v→%v",
			shownBefore, l.ease.shown, t0Before, l.ease.animT0)
	}
	if got := boundRead(l); got != 40 { // the blit that DOES drive it
		t.Errorf("after blit, displayed = %d, want 40", got)
	}
}

// An armed, scrolled layer must place its cursor with the text it belongs to. Reading the
// legacy field here put the cursor as if the pane had never scrolled.
func TestLayerScreenCursorTracksDisplayedWhenArmed(t *testing.T) {
	l, _, advance := easedLayer(t, 100)
	l.screenX, l.screenY = 0, 0
	l.cursor = Cursor{X: 2, Y: 25, Visible: true}

	_ = boundRead(l)
	l.ScrollTo(20)
	_ = boundRead(l)                // ease starts
	advance(100 * time.Millisecond) // past the duration
	_ = boundRead(l)                // settles at the target
	if got := l.DisplayedScrollY(); got != 20 {
		t.Fatalf("setup: displayed = %d, want 20", got)
	}

	x, y, visible := l.ScreenCursor()
	if !visible {
		t.Fatal("cursor at content row 25 with viewport [20,30) should be visible")
	}
	if x != 2 || y != 5 {
		t.Errorf("ScreenCursor = (%d,%d), want (2,5) — cursor row 25 minus displayed 20", x, y)
	}
}

// A new document opens at the top, even on an armed pane. Resetting only the legacy field
// left the bound target holding the previous document's offset.
func TestLayerContentSwapResetsThePairWhenArmed(t *testing.T) {
	l, cell, _ := easedLayer(t, 100)

	l.ScrollTo(60)
	if *cell != 60 {
		t.Fatalf("setup: cell = %d, want 60", *cell)
	}

	rows := make([]Component, 100)
	for i := range rows {
		rows[i] = Text("row")
	}
	l.SetContent(Build(VBox(rows...)), 10, 100) // page navigation
	if *cell != 0 {
		t.Errorf("after content swap the bound target = %d, want 0", *cell)
	}
	if got := l.ScrollY(); got != 0 {
		t.Errorf("ScrollY after content swap = %d, want 0", got)
	}
	if got := l.DisplayedScrollY(); got != 0 {
		t.Errorf("DisplayedScrollY after content swap = %d, want 0", got)
	}
	if l.ease.animating {
		t.Error("a content swap should not leave an ease in flight gliding from the old offset")
	}
}
