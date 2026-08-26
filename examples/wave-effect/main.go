package main

import (
	"log"
	"math"
	"time"

	. "github.com/kungfusheep/glyph"
)

type waveEffect struct{ phase *float64 }

func (w waveEffect) Apply(buf *Buffer, ctx PostContext) {
	tmp := NewBuffer(ctx.Width, ctx.Height)
	for y := range ctx.Height {
		offset := int(4.0 * math.Sin(float64(y)*0.7+*w.phase))
		for x := range ctx.Width {
			srcX := x - offset
			if srcX >= 0 && srcX < ctx.Width {
				tmp.Set(x, y, buf.Get(srcX, y))
			}
		}
	}
	for y := range ctx.Height {
		for x := range ctx.Width {
			buf.Set(x, y, tmp.Get(x, y))
		}
	}
}

func main() {
	app := NewApp()
	phase := 0.0

	app.SetView(
		VBox(
			Text("08:42  London Euston          On time"),
			Text("08:47  Bristol Temple Meads    Platform 4"),
			Text("08:55  Birmingham New Street   Platform 6"),
			Text("09:10  Manchester Piccadilly   Platform 3"),
			Text("09:15  Edinburgh Waverley      Delayed"),
			Text("09:22  Leeds Central           Platform 8"),
			Text("09:30  Glasgow Central         Platform 1"),
			Text("09:45  Cardiff Central         On time"),
			Text("09:52  Newcastle               Platform 2"),
			Text("10:00  York                    Platform 5"),
			Text("10:15  Liverpool Lime Street   Platform 7"),
			Text("10:22  Sheffield               Cancelled"),
			Text("10:30  Nottingham              Platform 9"),
			Text("10:45  Cambridge               On time"),
		),
	)

	app.AddEffect(waveEffect{phase: &phase})
	app.Handle("q", app.Stop)

	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			phase += 0.15
			app.RequestRender()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
