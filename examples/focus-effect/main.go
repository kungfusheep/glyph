package main

import (
	"log"

	. "github.com/kungfusheep/glyph"
)

func main() {
	app := NewApp()

	var selRef NodeRef
	items := []string{"inbox", "drafts", "sent", "trash", "spam", "archive", "starred", "important"}

	app.SetView(
		VBox(
			VBox.Border(BorderRounded).Title("mail")(
				List(&items).SelectedRef(&selRef).BindVimNav(),
			),
			ScreenEffect(SEFocusDim(&selRef)),
		),
	)

	app.Handle("q", app.Stop)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
