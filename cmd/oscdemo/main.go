package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/glyph"
)

// oscdemo is the visual harness for ADR 1: oscillators and self-gated
// animation. Every motion on screen is framework-driven — there is not a
// single ticker, goroutine, or frame counter in this file.
//
// keys: space hide/show all (the app goes fully idle while hidden),
// +/- live spinner speed, q quit.
func main() {
	app := NewApp()

	show := true
	spinHz := 4.0
	speedLabel := "4.0 cycles/sec"

	cyan := Hex(0x4FC3F7)
	amber := Hex(0xFFB74D)
	green := Hex(0x81C784)
	red := Hex(0xE57373)
	dim := Hex(0x3A3A3A)

	waveBar := func(label string, o OscC, c Color) Component {
		return HBox.Gap(1).Height(1)(
			Text(label).Width(10).FG(Hex(0x9E9E9E)),
			HBox.Width(o.Range(1, 50)).Height(1).Fill(c)(),
		)
	}

	travel := make([]Component, 0, 36)
	for i := 0; i < 36; i++ {
		travel = append(travel,
			Text("█").FG(Osc(0.4).Sine().Phase(float64(i)/36).Lerp(dim, cyan)))
	}

	app.SetView(
		VBox.Gap(1).PaddingVH(1, 2)(
			HBox.Gap(2)(
				Text("oscdemo").Bold().FG(Osc(0.15).Sine().Lerp(cyan, amber)),
				Text("ADR 1 harness — zero app-side animation code").Dim(),
				Space(),
				Text("[space] hide  [+/-] speed  [q] quit").Dim(),
			),
			If(&show).Then(VBox.Gap(1)(

				Text("waveforms — same 0.5hz clock, phase-locked").Bold(),
				waveBar("sine", Osc(0.5).Sine(), cyan),
				waveBar("triangle", Osc(0.5).Triangle(), green),
				waveBar("saw", Osc(0.5).Saw(), amber),
				waveBar("square .3", Osc(0.5).Square(0.3), red),
				waveBar("steps 8", Osc(0.5).Steps(8), Hex(0xBA68C8)),

				Text("easing — identical triangle wave, reshaped output").Bold(),
				waveBar("linear", Osc(0.5).Triangle(), green),
				waveBar("out-cubic", Osc(0.5).Triangle().Ease(EaseOutCubic), green),
				waveBar("out-quart", Osc(0.5).Triangle().Ease(EaseOutQuart), green),

				Text("phase — one wave travelling through 36 cells").Bold(),
				HBox(travel...),

				Text("spinners — self-animating, no counters").Bold(),
				HBox.Gap(3)(
					HBox.Gap(1)(Spinner().Frames(SpinnerBraille).FG(cyan), Text("braille 12fps").Dim()),
					HBox.Gap(1)(Spinner().Frames(SpinnerDots).Fps(8).FG(green), Text("dots 8fps").Dim()),
					HBox.Gap(1)(Spinner().Frames(SpinnerLine).Fps(20).FG(amber), Text("line 20fps").Dim()),
					HBox.Gap(1)(Spinner().Frames(SpinnerCircle).Fps(4).FG(red), Text("circle 4fps").Dim()),
				),

				Text("live speed — phase accumulates, never skips").Bold(),
				HBox.Gap(1)(
					HBox.Width(Osc(0).Saw().Speed(&spinHz).Range(1, 50)).Height(1).Fill(amber)(),
					Text(&speedLabel).Dim(),
				),

				Text("blink + breathe — square gates, sine breathes").Bold(),
				HBox.Gap(3)(
					HBox.Gap(1)(Text("●").FG(Osc(1).Square(0.5).Lerp(dim, red)), Text("alert blink").Dim()),
					HBox.Gap(1)(Text("●").FG(Osc(0.25).Sine().Lerp(dim, green)), Text("LED breathe").Dim()),
					HBox.Gap(1)(Text("▁▂▃▄▅▆▇█").FG(Osc(0.3).Sine().Lerp(cyan, amber)), Text("colour cycle").Dim()),
				),
			)).Else(
				Text("everything hidden — the app is fully idle: no ticker, no frames, zero CPU").Dim(),
			),
			ScreenEffect(SEVignette().Strength(Osc(0.1).Sine().Range(0.05, 0.35))),
		),
	)

	app.Handle("q", app.Stop)
	app.Handle("<Space>", func() { show = !show })
	app.Handle("+", func() { spinHz += 1; speedLabel = fmt.Sprintf("%.1f cycles/sec", spinHz) })
	app.Handle("-", func() {
		if spinHz > 1 {
			spinHz -= 1
		}
		speedLabel = fmt.Sprintf("%.1f cycles/sec", spinHz)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
