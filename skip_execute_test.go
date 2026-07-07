package glyph

import (
	"bytes"
	"fmt"
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

// --- paint-only frames (frame-cost ADR, slice 1) ---

// paintProbe counts geometry-pass entries (measure) vs paint-pass entries (render),
// letting tests assert a frame skipped layout without any engine hooks.
func paintProbe(measures, renders *int) Component {
	return Custom(
		func(availW int16) (int16, int16) { *measures++; return 5, 1 },
		func(buf *Buffer, x, y, w, h int16) { *renders++ },
	)
}

// An opacity-oscillator frame with stable geometry skips the geometry passes and
// re-runs only paint; the skip must also keep output correct (opacity advances).
func TestExecutePaintSkipsGeometryPasses(t *testing.T) {
	var measures, renders int
	tmpl := Build(VBox(
		paintProbe(&measures, &renders),
		Text("breathing").Opacity(Osc(1)),
	))
	clock := time.Unix(1000, 0)
	tmpl.nowFn = func() time.Time { return clock }
	buf := NewBuffer(40, 5)

	tmpl.Execute(buf, 40, 5) // full frame lays out + snapshots
	mAfterFull, rAfterFull := measures, renders
	if mAfterFull == 0 || rAfterFull == 0 {
		t.Fatalf("full frame should measure and render (m=%d r=%d)", mAfterFull, rAfterFull)
	}

	clock = clock.Add(100 * time.Millisecond)
	if !tmpl.ExecutePaint(buf, 40, 5) {
		t.Fatal("stable-geometry opacity animation should be paint-safe")
	}
	if measures != mAfterFull {
		t.Errorf("paint frame ran the geometry passes: measures %d -> %d", mAfterFull, measures)
	}
	if renders <= rAfterFull {
		t.Errorf("paint frame did not repaint: renders %d -> %d", rAfterFull, renders)
	}
}

// A geometry tween (animated Height) must refuse the paint-only path.
func TestExecutePaintRefusesGeometryTween(t *testing.T) {
	target := 3
	tmpl := Build(VBox(
		VBox.Height(Animate.Duration(200*time.Millisecond)(&target))(Text("sized")),
	))
	clock := time.Unix(1000, 0)
	tmpl.nowFn = func() time.Time { return clock }
	buf := NewBuffer(40, 10)

	tmpl.Execute(buf, 40, 10)
	target = 8 // retarget: the dyn height value starts moving
	// the retarget frame itself may legitimately paint-skip (the tween starts at the
	// old value, so geometry is unchanged THAT frame); the gate must refuse as soon
	// as the value actually moves — within a few ticks of the 200ms tween.
	refused := false
	for i := 0; i < 5; i++ {
		clock = clock.Add(50 * time.Millisecond)
		if !tmpl.ExecutePaint(buf, 40, 10) {
			refused = true
			break
		}
	}
	if !refused {
		t.Fatal("a moving height tween must force a full frame once the value moves")
	}
}

// An If flip between frames must refuse the paint-only path (structure changed).
func TestExecutePaintRefusesIfFlip(t *testing.T) {
	show := false
	tmpl := Build(VBox(
		Text("always"),
		If(&show).Then(Text("sometimes")),
	))
	buf := NewBuffer(40, 5)
	tmpl.Execute(buf, 40, 5)

	show = true
	if tmpl.ExecutePaint(buf, 40, 5) {
		t.Fatal("a flipped If must force the full frame")
	}
	// after a full frame with the new branch, stable again — but with no animation
	// running the caller wouldn't be on the paint path anyway; verify safety only.
	tmpl.Execute(buf, 40, 5)
	if !tmpl.paintSafe() {
		t.Fatal("stable If should verify paint-safe after the full frame")
	}
}

// A ForEach length change must refuse the paint-only path.
func TestExecutePaintRefusesForEachGrowth(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		ForEach(&items, func(s *string) Component { return Text(s) }),
	))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	if !tmpl.paintSafe() {
		t.Fatal("unchanged ForEach should be paint-safe")
	}
	items = append(items, "d")
	if tmpl.ExecutePaint(buf, 40, 10) {
		t.Fatal("a grown ForEach must force the full frame")
	}
}

// The headline: an opacity oscillator over the pathological unbounded list renders
// paint-only frames at render cost, not layout cost.
func BenchmarkPaintOnlyFrameUnboundedList(b *testing.B) {
	long := strings.Repeat("wide 世界 and 🚀 emoji text in every row to stress measurement. ", 2)
	items := make([]string, 2000)
	for i := range items {
		items[i] = fmt.Sprintf("%04d %s", i, long)
	}
	tmpl := Build(VBox(
		Text("status dot").Opacity(Osc(1)),
		ForEach(&items, func(s *string) Component { return Text(s) }),
	))
	buf := NewBuffer(220, 60)
	tmpl.Execute(buf, 220, 60) // one full frame to lay out + snapshot
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !tmpl.ExecutePaint(buf, 220, 60) {
			b.Fatal("expected paint-safe frame")
		}
	}
}

// A DIRECT render() call (the input path renders synchronously; RenderNow; resize)
// must never take the paint-only skip, even when an animation tick has flagged a
// pending anim frame — direct frames can carry state changed without RequestRender,
// and painting them over stale geometry is exactly the live-bug this regressed as.
// Only the debounce goroutine's renderPaced may consume the anim flag.
func TestDirectRenderNeverPaintSkips(t *testing.T) {
	var measures, renders int
	items := []string{"a", "b"}
	tmpl := Build(VBox(
		paintProbe(&measures, &renders),
		Text("dot").Opacity(Osc(1)),
		ForEach(&items, func(s *string) Component { return Text(s) }),
	))
	app := newEffectTestApp(tmpl, 40, 10)

	app.render() // first full frame lays out + snapshots
	mAfterFull := measures

	// an animation tick raced in, then input mutated state and rendered DIRECTLY
	// (no RequestRender — the input callback's synchronous path)
	app.requestAnimFrame()
	items = append(items, "c")
	app.render()
	if measures <= mAfterFull {
		t.Fatal("direct render() with a pending anim flag skipped layout — stale geometry for the grown list")
	}

	// the paced path still gets the skip when geometry is genuinely stable
	mAfterFull = measures
	rBefore := renders
	app.requestAnimFrame()
	app.renderPaced()
	if measures != mAfterFull {
		t.Fatalf("paced anim frame with stable geometry should skip layout (measures %d -> %d)", mAfterFull, measures)
	}
	if renders <= rBefore {
		t.Fatal("paced anim frame did not repaint")
	}
}

// Paced (anim-tick) frames must be pixel-identical to full frames through a modal's
// whole In-fade lifecycle — the composition is a help-overlay shape: If → screen
// effect (dodged vignette) → centred Overlay → VBox.Opacity(In/Out). Regression for
// the live report of a modal fade freezing when driven by paced frames.
func TestModalFadePacedMatchesFullFrames(t *testing.T) {
	var ref NodeRef
	run := func(paced bool) string {
		show := false
		tmpl := Build(VBox(
			Text("background row asdf asdf asdf"),
			If(&show).Then(VBox(
				ScreenEffect(SEVignette().Dodge(&ref).Strength(0.8)),
				Overlay.Centered()(
					VBox.Width(20).Fill(RGB(80, 80, 80)).NodeRef(&ref).
						Opacity(In(Animate(1.0)).Out(Animate(0)))(
						Text("modal content").FG(RGB(255, 255, 255)),
					),
				),
			)),
		))
		clock := time.Unix(1000, 0)
		tmpl.nowFn = func() time.Time { return clock }
		app := newEffectTestApp(tmpl, 40, 9)
		app.render() // closed
		show = true
		app.RequestRender()
		app.render() // open: overlay appears, In-tween arms
		out := ""
		for i := 0; i < 12; i++ {
			clock = clock.Add(40 * time.Millisecond)
			if paced {
				app.requestAnimFrame()
				app.renderPaced()
			} else {
				app.RequestRender()
				app.render()
			}
			s1 := app.screen.back.Get(2, 0).Style  // background text under the vignette
			s2 := app.screen.back.Get(20, 4).Style // modal content cell
			out += fmt.Sprintf("%v|%v|%v;", s1, s2, tmpl.Animating())
		}
		return out
	}
	full := run(false)
	pacedRun := run(true)
	if full != pacedRun {
		t.Fatalf("paced fade diverges from full frames:\nfull:  %s\npaced: %s", full, pacedRun)
	}
}
