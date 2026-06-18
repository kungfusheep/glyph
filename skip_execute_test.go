package glyph

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func newEffectTestApp(tmpl *Template, w, h int) *App {
	var out bytes.Buffer
	screen := NewScreen(&out)
	screen.width = w
	screen.height = h
	screen.front.Resize(w, h)
	screen.back.Resize(w, h)
	app := &App{
		screen:     screen,
		template:   tmpl,
		pool:       NewBufferPool(w, h),
		renderChan: make(chan struct{}, 1),
	}
	tmpl.SetApp(app)
	return app
}

func frontLine(a *App, y, w int) string {
	s := make([]rune, w)
	for x := 0; x < w; x++ {
		s[x] = a.screen.front.Get(x, y).Rune
	}
	return strings.TrimRight(string(s), " \x00")
}

// TestSkipExecuteOnEffectFrame: a frame requested by an effect's animation skips
// Execute and reuses the cached render. We prove the skip by mutating bound state
// WITHOUT RequestRender — if Execute ran, the change would show; on a skip it must not.
func TestSkipExecuteOnEffectFrame(t *testing.T) {
	label := "AAA"
	tmpl := Build(VBox(
		Text(&label),
		ScreenEffect(SETint(RGB(200, 100, 50)).Strength(0.5)),
	))
	app := newEffectTestApp(tmpl, 20, 4)

	a := app
	a.appDirty.Store(true) // first frame is a full render
	a.render()
	if got := frontLine(a, 0, 20); !strings.Contains(got, "AAA") {
		t.Fatalf("first render should show AAA, got %q", got)
	}

	// mutate WITHOUT RequestRender, then request an effect frame and render
	label = "BBB"
	a.requestEffectFrame()
	a.render()
	if got := frontLine(a, 0, 20); strings.Contains(got, "BBB") {
		t.Fatalf("effect-only frame should SKIP Execute (still AAA), got %q", got)
	}
	if got := frontLine(a, 0, 20); !strings.Contains(got, "AAA") {
		t.Fatalf("effect-only frame should reuse cached render (AAA), got %q", got)
	}

	// a real change via RequestRender must do a full Execute and show BBB
	a.RequestRender()
	a.render()
	if got := frontLine(a, 0, 20); !strings.Contains(got, "BBB") {
		t.Fatalf("RequestRender should full-Execute and show BBB, got %q", got)
	}
}

// TestAnimatingTemplateNotSkipped (Komorebi's guard): an animating template under
// an active effect must NOT be skipped — its animated cells advance frame to frame.
func TestAnimatingTemplateNotSkipped(t *testing.T) {
	clock := time.Unix(0, 0)
	tmpl := Build(VBox(
		Spinner(),
		ScreenEffect(SETint(RGB(200, 100, 50)).Strength(0.5)),
	))
	tmpl.nowFn = func() time.Time { return clock }
	app := newEffectTestApp(tmpl, 20, 4)

	app.appDirty.Store(true)
	app.render()
	first := frontLine(app, 0, 20)

	// advance the clock and request an effect frame; because the template is
	// animating (spinner), the gate must NOT skip — the spinner must advance.
	clock = clock.Add(500 * time.Millisecond)
	app.requestEffectFrame()
	app.render()
	second := frontLine(app, 0, 20)

	if first == second {
		t.Fatalf("animating template must keep advancing (not skipped); both %q", first)
	}
}

// TestSkipMatchesFullRender: for a static effect scene the skip-frame output is
// byte-identical to a forced full render of the same state (no visible difference).
func TestSkipMatchesFullRender(t *testing.T) {
	label := "hello"
	mk := func() *Template {
		return Build(VBox(
			Text(&label),
			ScreenEffect(SETint(RGB(200, 100, 50)).Strength(0.5)),
		))
	}
	a := newEffectTestApp(mk(), 20, 4)
	a.appDirty.Store(true)
	a.render() // establish clean
	a.requestEffectFrame()
	a.render() // skip frame
	skipFront := snapshotCells(a.screen.front)

	// force a full render of the same state
	a.RequestRender()
	a.render()
	fullFront := snapshotCells(a.screen.front)

	for i := range fullFront {
		if skipFront[i] != fullFront[i] {
			t.Fatalf("cell %d: skip-frame %+v != full-render %+v", i, skipFront[i], fullFront[i])
		}
	}
}

func snapshotCells(b *Buffer) []Cell {
	out := make([]Cell, len(b.cells))
	copy(out, b.cells)
	return out
}

func benchRenderScene() *App {
	rows := make([]string, 25)
	for i := range rows {
		rows[i] = "dashboard row of content here"
	}
	children := []Component{}
	for i := range rows {
		children = append(children, Text(&rows[i]))
	}
	children = append(children, ScreenEffect(SETint(RGB(200, 100, 50)).Strength(0.5)))
	tmpl := Build(VBox(children...))
	return newEffectTestApp(tmpl, 60, 30)
}

func BenchmarkRenderFullFrame(b *testing.B) {
	a := benchRenderScene()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.RequestRender() // full Execute each frame
		a.render()
	}
}

func BenchmarkRenderEffectOnlyFrame(b *testing.B) {
	a := benchRenderScene()
	a.appDirty.Store(true)
	a.render() // establish clean
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.requestEffectFrame() // skip Execute, re-run effect
		a.render()
	}
}

// TestExecuteNotCalledOnSkip directly asserts Execute does NOT run on an
// effect-only frame: a Text bound to a counter-incrementing func only ticks when
// the template is Executed, so the count stays flat across effect frames.
func TestExecuteNotCalledOnSkip(t *testing.T) {
	executes := 0
	tmpl := Build(VBox(
		Text(func() string { executes++; return "x" }),
		ScreenEffect(SETint(RGB(200, 100, 50)).Strength(0.5)),
	))
	a := newEffectTestApp(tmpl, 20, 4)

	a.appDirty.Store(true)
	a.render() // full
	base := executes
	if base == 0 {
		t.Fatal("expected at least one Execute on the first frame")
	}

	// 5 effect-only frames must NOT increment the Execute counter
	for i := 0; i < 5; i++ {
		a.requestEffectFrame()
		a.render()
	}
	if executes != base {
		t.Fatalf("Execute ran on effect-only frames: count %d -> %d (expected flat)", base, executes)
	}

	// a real RequestRender must Execute again
	a.RequestRender()
	a.render()
	if executes <= base {
		t.Fatalf("RequestRender should Execute: count stayed %d", executes)
	}
}
