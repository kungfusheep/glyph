package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/forme"
)

func main() {
	var name, email string
	role := 0
	agree := false
	status := "Tab: next | j/k: radio | Space: checkbox | Enter: submit"

	form := Form(
		Field("Name", Input(&name).Validate(VRequired, VOnBlur)),
		Field("Email", Input(&email).Validate(VEmail, VOnBlur)),
		Field("Role", Radio(&role, "Admin", "User", "Guest")),
		Field("Terms", Checkbox(&agree, "I accept").Validate(VTrue, VOnSubmit)),
	).LabelBold()

	form.OnSubmit(func() {
		if form.ValidateAll() {
			roles := []string{"Admin", "User", "Guest"}
			status = fmt.Sprintf("Registered: %s <%s> as %s", name, email, roles[role])
		} else {
			status = "Please fix the errors above"
		}
	})

	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.SetView(
		VBox.Border(BorderRounded).Title("registration")(
			form,
			SpaceH(1),
			HRule().FG(BrightBlack),
			Text(&status).FG(BrightBlack),
		),
	)

	app.Handle("<Escape>", app.Stop)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
