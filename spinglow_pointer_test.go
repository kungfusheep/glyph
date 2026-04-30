package glyph

import (
	"testing"
)

// TestSESpinGlowPointerParams verifies Strength/Radius/Speed/Falloff all
// accept pointer arguments and read through them live.
func TestSESpinGlowPointerParams(t *testing.T) {
	var (
		strength float64 = 0.5
		radius   int16   = 8
		speed    float64 = 1.0
		falloff  float64 = 0.0
	)

	buf := NewBuffer(40, 20)
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: RGB(200, 200, 200)}})
		}
	}

	ref := NodeRef{X: 15, Y: 8, W: 10, H: 4}
	eff := SESpinGlow(&ref, RGB(255, 0, 0)).
		Strength(&strength).
		Radius(&radius).
		Speed(&speed).
		Falloff(&falloff)

	// compile (as ScreenEffectNode would do during template Build)
	tmpl := &Template{}
	compiled := eff.compileEffect(tmpl).(spinGlowEffect)

	// baseline read-back
	if got := compiled.strength.resolve(); got != 0.5 {
		t.Errorf("strength initial: got %v want 0.5", got)
	}
	if got := compiled.radius.resolve(); got != 8 {
		t.Errorf("radius initial: got %v want 8", got)
	}
	if got := compiled.speed.resolve(); got != 1.0 {
		t.Errorf("speed initial: got %v want 1.0", got)
	}
	if got := compiled.falloff.resolve(); got != 0.0 {
		t.Errorf("falloff initial: got %v want 0.0", got)
	}

	// mutate and verify the effect sees the new values
	strength = 1.0
	radius = 12
	speed = 2.5
	falloff = 3.0

	if got := compiled.strength.resolve(); got != 1.0 {
		t.Errorf("strength after mutate: got %v want 1.0", got)
	}
	if got := compiled.radius.resolve(); got != 12 {
		t.Errorf("radius after mutate: got %v want 12", got)
	}
	if got := compiled.speed.resolve(); got != 2.5 {
		t.Errorf("speed after mutate: got %v want 2.5", got)
	}
	if got := compiled.falloff.resolve(); got != 3.0 {
		t.Errorf("falloff after mutate: got %v want 3.0", got)
	}

	// Apply should also read live values
	compiled.Apply(buf, PostContext{Width: 40, Height: 20, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12)})
}
