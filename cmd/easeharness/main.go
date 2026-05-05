package main

import (
	"log"
	"time"

	. "github.com/kungfusheep/glyph"
)

// easeharness — visual comparison of every EaseOut across three properties:
// fill (Progress value), fade (Text opacity), and slide (margin position).
// Press space to retrigger, q to quit. All rows share one target value per
// property; each row's tween keeps its own internal state, so the curves
// race against the same timeline.
func main() {
	app := NewApp()

	type easeRow struct {
		name string
		fn   func(float64) float64
	}

	rows := []easeRow{
		{"Linear", Linear},
		{"EaseOutSine", EaseOutSine},
		{"EaseOutQuad", EaseOutQuad},
		{"EaseOutCubic", EaseOutCubic},
		{"EaseOutQuart", EaseOutQuart},
		{"EaseOutQuint", EaseOutQuint},
		{"EaseOutExpo", EaseOutExpo},
		{"EaseOutCirc", EaseOutCirc},
		{"EaseOutBack", EaseOutBack},
		{"EaseOutBounce", EaseOutBounce},
	}

	const dur = 300 * time.Millisecond
	const fillW = int16(28)
	const slideTrack = int16(28)

	fill := 100
	fade := 1.0
	slide := slideTrack

	header := HBox.Gap(2)(
		Text("").Width(16),
		Text("FILL").Width(fillW).FG(BrightBlack),
		Text("FADE").Width(8).FG(BrightBlack),
		Text("SLIDE").Width(slideTrack).FG(BrightBlack),
	)

	bars := make([]Component, 0, len(rows))
	for _, r := range rows {
		anim := Animate.Duration(dur).Ease(r.fn)

		bars = append(bars,
			HBox.Gap(2)(
				Text(r.name).Width(16).FG(BrightBlack),
				Progress(anim(&fill)).Width(fillW),
				Text("●●●●●").Opacity(anim(&fade)).FG(Hex(0xFF00FF)).Width(8),
				VBox.Width(slideTrack+1)(
					HBox(
						Space().Width(anim(&slide)),
						Text("►").FG(Magenta),
					),
				),
			),
		)
	}

	app.SetView(
		VBox.Gap(1).Margin(2)(
			Text("EaseOut comparison").Bold(),
			Text("space  retrigger    q  quit").FG(BrightBlack),
			header,
			VBox.Gap(1)(bars...),
			Space(),
		),
	)

	flipped := false
	app.Handle("<Space>", func() {
		flipped = !flipped
		if flipped {
			fill, fade, slide = 0, 0.0, 0
		} else {
			fill, fade, slide = 100, 1.0, slideTrack
		}
	})
	app.Handle("q", app.Stop)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
