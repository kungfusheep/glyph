package glyph

import (
	"testing"
	"time"
)

func TestOscWaveforms(t *testing.T) {
	at := func(o OscC, ms int) float64 {
		var acc oscAccum
		return o.resolve(time.Duration(ms)*time.Millisecond, &acc)
	}

	saw := Osc(1).Saw()
	if got := at(saw, 0); got != 0 {
		t.Fatalf("saw@0 = %v, want 0", got)
	}
	if got := at(saw, 500); got != 0.5 {
		t.Fatalf("saw@500ms = %v, want 0.5", got)
	}

	tri := Osc(1).Triangle()
	if got := at(tri, 250); got != 0.5 {
		t.Fatalf("triangle@250ms = %v, want 0.5", got)
	}
	if got := at(tri, 750); got != 0.5 {
		t.Fatalf("triangle@750ms = %v, want 0.5", got)
	}

	sq := Osc(1).Square(0.25)
	if got := at(sq, 100); got != 1 {
		t.Fatalf("square@100ms (duty .25) = %v, want 1", got)
	}
	if got := at(sq, 300); got != 0 {
		t.Fatalf("square@300ms (duty .25) = %v, want 0", got)
	}

	steps := Osc(1).Steps(4)
	if got := at(steps, 0); got != 0 {
		t.Fatalf("steps@0 = %v, want 0", got)
	}
	if got := at(steps, 900); got != 1 {
		t.Fatalf("steps@900ms = %v, want 1 (last level)", got)
	}

	// range scales, phase shifts
	ranged := Osc(1).Saw().Range(10, 20)
	if got := at(ranged, 500); got != 15 {
		t.Fatalf("saw.range(10,20)@500ms = %v, want 15", got)
	}
	phased := Osc(1).Saw().Phase(0.5)
	if got := at(phased, 0); got != 0.5 {
		t.Fatalf("saw.phase(0.5)@0 = %v, want 0.5", got)
	}

	// ease reshapes output, applied before range
	eased := Osc(1).Saw().Ease(func(v float64) float64 { return v * v }).Range(0, 100)
	if got := at(eased, 500); got != 25 {
		t.Fatalf("saw.ease(sq).range(0,100)@500ms = %v, want 25", got)
	}
}

func TestOscLiveSpeedAccumulatesWithoutSkip(t *testing.T) {
	hz := 1.0
	o := Osc(0).Saw().Speed(&hz)
	var acc oscAccum

	o.resolve(0, &acc) // arm at t=0
	if got := o.resolve(500*time.Millisecond, &acc); got != 0.5 {
		t.Fatalf("live saw@500ms hz=1: %v, want 0.5", got)
	}
	hz = 4.0 // speed up mid-flight: phase must continue, not jump
	// 0.5 + 0.125s*4hz = phase 1.0, which wraps to exactly 0 — continuity, no skip
	if got := o.resolve(625*time.Millisecond, &acc); got != 0 {
		t.Fatalf("live saw after hz change: %v, want exact wrap to 0", got)
	}
	// quarter cycle later at the new speed
	if got := o.resolve(687500*time.Microsecond, &acc); got != 0.25 {
		t.Fatalf("live saw quarter-cycle at hz=4: %v, want 0.25", got)
	}
}

func TestOscColorLerpThroughTemplate(t *testing.T) {
	label := "x"
	tmpl := Build(VBox(
		Text(&label).FG(Osc(1).Saw().Lerp(RGB(0, 0, 0), RGB(200, 100, 50))),
	))

	base := time.Unix(1000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(10, 2)
	tmpl.Execute(buf, 10, 2) // epoch arms at base; saw=0 -> colour A
	if fg := buf.Get(0, 0).Style.FG; fg.R != 0 || fg.G != 0 || fg.B != 0 {
		t.Fatalf("frame@0 fg = %+v, want black", fg)
	}
	if !tmpl.Animating() {
		t.Fatal("oscillator resolved but template not animating")
	}

	clock = base.Add(500 * time.Millisecond)
	buf2 := NewBuffer(10, 2)
	tmpl.Execute(buf2, 10, 2) // saw=0.5 -> midpoint blend
	fg := buf2.Get(0, 0).Style.FG
	if fg.R != 100 || fg.G != 50 || fg.B != 25 {
		t.Fatalf("frame@500ms fg = %+v, want midpoint (100,50,25)", fg)
	}
}

func TestSpinnerSelfAnimates(t *testing.T) {
	tmpl := Build(VBox(Spinner().Frames(SpinnerDots)))

	base := time.Unix(2000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(4, 2)
	tmpl.Execute(buf, 4, 2)
	first := buf.Get(0, 0).Rune
	if !tmpl.Animating() {
		t.Fatal("self-animating spinner did not mark template animating")
	}

	clock = base.Add(250 * time.Millisecond) // 12fps -> frame index 3
	buf2 := NewBuffer(4, 2)
	tmpl.Execute(buf2, 4, 2)
	second := buf2.Get(0, 0).Rune
	if first == second {
		t.Fatalf("spinner frame did not advance: %q both frames", string(first))
	}
}

func TestSpinnerManualPointerStillWorks(t *testing.T) {
	frame := 0
	tmpl := Build(VBox(Spinner(&frame).Frames(SpinnerDots)))
	buf := NewBuffer(4, 2)
	tmpl.Execute(buf, 4, 2)
	if tmpl.Animating() {
		t.Fatal("manual spinner must not self-mark animating")
	}
	first := buf.Get(0, 0).Rune
	frame = 3
	buf2 := NewBuffer(4, 2)
	tmpl.Execute(buf2, 4, 2)
	if buf2.Get(0, 0).Rune == first {
		t.Fatal("manual frame pointer ignored")
	}
}

func TestOscInsideInactiveBranchDoesNotAnimate(t *testing.T) {
	show := false
	tmpl := Build(VBox(
		If(&show).Then(Spinner().Frames(SpinnerDots)).Else(Text("idle")),
	))

	base := time.Unix(3000, 0)
	tmpl.nowFn = func() time.Time { return base }

	buf := NewBuffer(10, 2)
	tmpl.Execute(buf, 10, 2)
	if tmpl.Animating() {
		t.Fatal("hidden spinner kept the template animating; gate must follow visibility")
	}

	show = true
	buf2 := NewBuffer(10, 2)
	tmpl.Execute(buf2, 10, 2)
	if !tmpl.Animating() {
		t.Fatal("visible spinner did not mark the template animating")
	}
}
