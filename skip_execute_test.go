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

// TestAnimatingTemplateNotSkipped: an animating template under
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

// --- ADR 19 v2: an oscillator feeding a screen-effect parameter drives EFFECT
// frames (skip Execute over the cached render), not a full Execute every frame.

// TestEffectOscDoesNotForceExecute: an Osc on a screen-effect parameter must NOT
// mark the template animating (that would block the cache-skip gate), yet
// RunEffectEvals must advance it so the effect output changes over time.
func TestEffectOscDoesNotForceExecute(t *testing.T) {
	tmpl := Build(VBox(
		Text("static"),
		ScreenEffect(SEVignette().Smooth().Strength(Osc(1).Range(0.1, 0.9))),
	))
	var clk time.Duration
	tmpl.nowFn = func() time.Time { return time.Unix(0, 0).Add(clk) }

	buf := NewBuffer(20, 8)
	tmpl.Execute(buf, 20, 8)
	if tmpl.Animating() {
		t.Fatal("osc on a screen-effect param must NOT mark the template animating (it should drive effect frames)")
	}

	eff := tmpl.ScreenEffects()[0]
	cornerAt := func(d time.Duration) Color {
		clk = d
		tmpl.RunEffectEvals() // resolve the effect-param osc at this clock
		b := NewBuffer(20, 8)
		for y := 0; y < 8; y++ {
			for x := 0; x < 20; x++ {
				b.Set(x, y, Cell{Rune: ' ', Style: Style{BG: RGB(120, 120, 120)}})
			}
		}
		eff.Apply(b, PostContext{Width: 20, Height: 8})
		return b.Get(0, 0).Style.BG
	}
	// quarter-period apart on a 1Hz sine: strength (hence corner darkening) must differ
	if cornerAt(0) == cornerAt(250*time.Millisecond) {
		t.Fatal("effect-param osc did not advance via RunEffectEvals — vignette corner unchanged across the sine")
	}
}

// TestEffectOscOscillatesWhileSkippingExecute: the built-in SEVignette().Strength(Osc)
// over a STATIC template self-sustains effect-only frames (via requestEffectFrame in
// the effect pass) and Execute stays flat — i.e. the oscillator runs without forcing
// a full Execute every frame, the whole point of ADR 19 v2.
func TestEffectOscOscillatesWhileSkippingExecute(t *testing.T) {
	executes := 0
	tmpl := Build(VBox(
		Text(func() string { executes++; return "static" }),
		ScreenEffect(SEVignette().Smooth().Strength(Osc(1).Range(0.1, 0.9))),
	))
	var clk time.Duration
	tmpl.nowFn = func() time.Time { return time.Unix(0, 0).Add(clk) }
	a := newEffectTestApp(tmpl, 20, 8)

	a.appDirty.Store(true)
	a.render() // first full frame
	base := executes
	if base == 0 {
		t.Fatal("expected an Execute on the first frame")
	}
	if !a.effectFramePending.Load() {
		t.Fatal("the effect osc should have requested the next effect frame")
	}

	for i := 0; i < 8; i++ {
		clk += 120 * time.Millisecond
		a.render() // osc-driven effect frame; must skip Execute and re-request
	}
	if executes != base {
		t.Fatalf("Execute ran on osc-driven effect frames: %d -> %d (the effect osc forced Execute — v2 broken)", base, executes)
	}
	if !a.effectFramePending.Load() {
		t.Fatal("the effect osc stopped sustaining effect frames")
	}
}

// a real app scenario through the APP path (not bare Execute): an If-gated overlay
// carrying a NodeRef that a screen-effect dodges, with effect-only frames in the mix
// (the breathing FocusShade). After the overlay closes, the dodge must RELEASE — the
// ref must zero and stay zero across subsequent effect-only frames, or a phantom
// un-dimmed region persists (Pete's #442 report).
func TestDodgeReleasesAfterOverlayCloseThroughAppPath(t *testing.T) {
	open := true
	var ref NodeRef
	bg := "background"
	tmpl := Build(VBox(
		Text(&bg),
		If(&open).Then(
			Overlay.Centered()(
				VBox.Border(BorderRounded).NodeRef(&ref)(Text("HELP")),
			),
		),
		ScreenEffect(SEVignette().Smooth().Strength(0.9).Dodge(&ref)),
	))
	var clk time.Duration
	tmpl.nowFn = func() time.Time { return time.Unix(0, 0).Add(clk) }
	a := newEffectTestApp(tmpl, 30, 12)

	a.appDirty.Store(true)
	a.render() // open: ref populated, vignette dodges the help rect
	if ref.W == 0 || ref.H == 0 {
		t.Fatalf("ref should populate while open: W%d H%d", ref.W, ref.H)
	}

	// close (as a keypress does: RequestRender → appDirty → full Execute)
	open = false
	a.RequestRender()
	a.render()
	if ref.W != 0 || ref.H != 0 {
		t.Fatalf("ref not zeroed after close through app path: W%d H%d", ref.W, ref.H)
	}

	// breathing-effect frames must NOT resurrect the dodge region
	for i := 0; i < 5; i++ {
		clk += 100 * time.Millisecond
		a.requestEffectFrame()
		a.render()
	}
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("ref resurrected on effect-only frames: W%d H%d (phantom dodge persists)", ref.W, ref.H)
	}
}
