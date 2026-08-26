package main

import (
	"log"
	"time"

	. "github.com/kungfusheep/glyph"
)

func main() {
	app := NewApp()
	seed := 0

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
				if c.Rune == ' ' || c.Rune == 0 {
					return c
				}
				h := (x*374761393 + y*668265263 + seed*2654435761)
				if h < 0 {
					h = -h
				}
				if h%5 == 0 {
					glitches := []rune{'░', '▒', '▓', '█', '▄', '▀', '▌', '▐'}
					c.Rune = glitches[(h>>8)%len(glitches)]
					c.Style.FG = RGB(uint8(100+h%80), uint8(60+h%40), uint8(80+h%60))
				}
				return c
			})),
		),
	)

	app.Handle("q", app.Stop)

	go func() {
		for {
			time.Sleep(150 * time.Millisecond)
			seed++
			app.RequestRender()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
