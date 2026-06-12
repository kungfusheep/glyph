package glyph

import (
	"math"
	"time"
)

// oscWave selects the waveform an oscillator produces.
type oscWave uint8

const (
	oscSine oscWave = iota
	oscTriangle
	oscSaw
	oscSquare
	oscSteps
)

// OscC is a periodic value node: pure derivation from the frame clock,
// value = waveform(fract(elapsed*hz + phase)) eased then scaled into range.
// Bind it anywhere the property compiler accepts a dynamic value:
//
//	Text("●").FG(Osc(0.5).Lerp(uiDim, uiAccent))   // colour breathe
//	SEVignette().Strength(Osc(0.25).Range(0.2, 0.6))
//
// While an oscillator resolves it marks the template animating, riding the
// same gated frame ticker tweens use: visible means animating, hidden costs
// nothing. ADR 1.
type OscC struct {
	hz    float64
	hzPtr *float64
	wave  oscWave
	duty  float64
	steps int
	phase float64
	ease  func(float64) float64
	min   float64
	max   float64
	colA  Color
	colB  Color
	lerp  bool
}

// Osc creates an oscillator at hz cycles per second. Default waveform is
// sine starting from its trough; default output range is [0,1].
func Osc(hz float64) OscC {
	return OscC{hz: hz, min: 0, max: 1}
}

// Sine selects a sine waveform (trough at phase 0, peak at half cycle).
func (o OscC) Sine() OscC { o.wave = oscSine; return o }

// Triangle selects a linear rise-then-fall waveform.
func (o OscC) Triangle() OscC { o.wave = oscTriangle; return o }

// Saw selects a linear ramp that resets each cycle.
func (o OscC) Saw() OscC { o.wave = oscSaw; return o }

// Square selects a two-level waveform: 1 for the duty fraction of the cycle,
// 0 for the rest.
func (o OscC) Square(duty float64) OscC { o.wave = oscSquare; o.duty = duty; return o }

// Steps quantises the cycle into n discrete levels — frame-flipping.
func (o OscC) Steps(n int) OscC { o.wave = oscSteps; o.steps = n; return o }

// Range scales the waveform's [0,1] output into [min, max].
func (o OscC) Range(min, max float64) OscC { o.min, o.max = min, max; return o }

// Lerp maps the waveform's output to a colour blend from a to b. Bind the
// result to FG/BG/Fill.
func (o OscC) Lerp(a, b Color) OscC { o.colA, o.colB, o.lerp = a, b, true; return o }

// Phase offsets the cycle position by p (fraction of one cycle, [0,1)).
// Same-frequency oscillators share the global epoch and are otherwise
// perfectly synchronised; Phase is the deliberate de-synchroniser.
func (o OscC) Phase(p float64) OscC { o.phase = p; return o }

// Ease reshapes the output curve with an easing function (same signature as
// Animate.Ease; the existing easing library applies). It changes amplitude
// over the cycle, not the clock: the cycle still takes exactly 1/hz seconds.
func (o OscC) Ease(fn func(float64) float64) OscC { o.ease = fn; return o }

// Speed binds the frequency to a pointer, re-read every frame. Live-speed
// oscillators accumulate phase by delta time so a frequency change never
// makes the position jump.
func (o OscC) Speed(hz *float64) OscC { o.hzPtr = hz; return o }

// oscAccum carries phase for live-speed oscillators (frequency changes must
// not jump position, so phase integrates dt*hz instead of deriving from
// elapsed). Constant-frequency oscillators stay stateless.
type oscAccum struct {
	last  time.Duration
	phase float64
	armed bool
}

// resolve produces the oscillator's mapped value for the given elapsed time.
func (o *OscC) resolve(elapsed time.Duration, acc *oscAccum) float64 {
	var ph float64
	if o.hzPtr != nil {
		if !acc.armed {
			acc.armed = true
			acc.last = elapsed
		}
		dt := (elapsed - acc.last).Seconds()
		acc.last = elapsed
		acc.phase += dt * (*o.hzPtr)
		ph = acc.phase
	} else {
		ph = elapsed.Seconds() * o.hz
	}
	ph += o.phase
	ph -= math.Floor(ph)

	var v float64
	switch o.wave {
	case oscSine:
		v = (1 - math.Cos(2*math.Pi*ph)) / 2
	case oscTriangle:
		if ph < 0.5 {
			v = ph * 2
		} else {
			v = 2 - ph*2
		}
	case oscSaw:
		v = ph
	case oscSquare:
		if ph < o.duty {
			v = 1
		} else {
			v = 0
		}
	case oscSteps:
		n := o.steps
		if n < 2 {
			return o.min
		}
		idx := int(ph * float64(n))
		if idx >= n {
			idx = n - 1
		}
		v = float64(idx) / float64(n-1)
	}
	if o.ease != nil {
		v = o.ease(v)
	}
	return o.min + v*(o.max-o.min)
}

// stepIndex returns the discrete frame index for Steps-style consumers
// (the self-animating spinner).
func oscStepIndex(elapsed time.Duration, fps float64, n int) int {
	if n <= 0 {
		return 0
	}
	return int(elapsed.Seconds()*fps) % n
}
