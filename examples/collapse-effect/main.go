package main

import (
	"log"
	"math"
	"time"

	. "github.com/kungfusheep/glyph"
)

func main() {
	app := NewApp()
	radius := 0.0
	growing := true

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
			ScreenEffect(EachCell(func(x, y int, c Cell, ctx PostContext) Cell {
				cx := float64(ctx.Width) / 2
				cy := float64(ctx.Height) / 2
				dx := (float64(x) - cx) / cx
				dy := (float64(y) - cy) / cy
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < radius {
					t := 1.0 - dist/radius
					blocks := []rune{' ', '░', '▒', '▓', '█'}
					idx := int(t * float64(len(blocks)-1))
					c.Rune = blocks[idx]
					c.Style.FG = RGB(
						uint8(min(255, 104+t*151)),
						uint8(min(255, 40+t*56)),
						uint8(min(255, 80+t*16)),
					)
				}
				return c
			})),
		),
	)

	app.Handle("q", app.Stop)

	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			if growing {
				radius += 0.02
				if radius > 1.2 {
					growing = false
				}
			} else {
				radius -= 0.02
				if radius < 0.0 {
					radius = 0.0
					growing = true
				}
			}
			app.RequestRender()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
