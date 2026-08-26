package main

import (
	"log"
	"time"

	. "github.com/kungfusheep/glyph"
)

func main() {
	app := NewApp()

	vignetteOn := false
	items := []string{"main.go", "handler.go", "config.go", "routes.go", "auth.go", "db.go"}

	smooth := Animate.Duration(500 * time.Millisecond).Ease(EaseOutCubic)

	app.SetView(
		VBox(
			HBox.Gap(2)(
				Text("animated effects").Bold(),
				Space(),
				Text("v: vignette  q: quit").FG(BrightBlack),
			),
			VBox.Grow(1).Border(BorderRounded).Title("files")(
				List(&items).BindVimNav(),
			),
			ScreenEffect(
				SEVignette().Strength(smooth(If(&vignetteOn).Then(0.8).Else(0.0))),
			),
		),
	)

	app.Handle("v", func() { vignetteOn = !vignetteOn })
	app.Handle("q", app.Stop)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
