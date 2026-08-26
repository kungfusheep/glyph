// inline form prompt — like gum input chained / enquirer
package main

import (
	"fmt"

	. "github.com/kungfusheep/glyph"
)

func main() {
	name := Input().Placeholder("your name").Width(30)
	email := Input().Placeholder("you@example.com").Width(30)
	token := Input().Placeholder("ghp_...").Width(30).Mask('*')

	form := Form.Gap(0).LabelFG(Cyan)(
		Field("Name", name),
		Field("Email", email),
		Field("Token", token),
	)

	app := NewInlineApp()

	app.ClearOnExit(true).
		SetView(form).
		Handle("<Enter>", app.Stop).
		Handle("<Escape>", app.Stop).
		Run()

	fmt.Printf("name=%q email=%q token=%q\n", name.Value(), email.Value(), token.Value())
}
