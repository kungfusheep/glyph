package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/glyph"
)

func main() {
	var name, email string
	role := 0
	agree := false
	status := "Tab: next | j/k: radio | Space: checkbox | Enter: submit"

	register := func(f *FormC) {
		roles := []string{"Admin", "User", "Guest"}
		status = fmt.Sprintf("Registered: %s <%s> as %s", name, email, roles[role])
	}

	form := Form.LabelBold().OnSubmit(register)(
		Field("Name", Input(&name).Validate(VRequired, VOnBlur)),
		Field("Email", Input(&email).Validate(VEmail, VOnBlur)),
		Field("Role", Radio(&role, "Admin", "User", "Guest")),
		Field("Terms", Checkbox(&agree, "I accept").Validate(VTrue, VOnSubmit)),
	)

	app := NewApp()

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
