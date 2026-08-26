package main

import . "github.com/kungfusheep/glyph"

func main() {
	count := 0

	app := NewInlineApp()
	app.SetView(
		VBox(
			Text(&count),
			Text("↑/↓ to count, enter to quit"),
		),
	).
		Handle("<Up>", func() { count++ }).
		Handle("<Down>", func() { count-- }).
		Handle("<Enter>", app.Stop).
		ClearOnExit(true).
		Run()

	println("You selected", count)
}
