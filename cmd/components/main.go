package main

import (
	"fmt"
	"log"

	"riffkey"
	"tui"
)

func main() {
	app, err := tui.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	// Some sample data
	users := []User{
		{Name: "Alice", Email: "alice@example.com", Online: true},
		{Name: "Bob", Email: "bob@example.com", Online: false},
		{Name: "Charlie", Email: "charlie@example.com", Online: true},
		{Name: "Diana", Email: "diana@example.com", Online: true},
	}

	// References to components we want to update
	var (
		statusLabel *tui.TextComponent
		countLabel  *tui.TextComponent
	)

	counter := 0

	// Build the UI declaratively!
	root := tui.VStack(
		// Header
		tui.HStack(
			tui.Text("Component Demo").Bold().Color(tui.Cyan),
			tui.Spacer(),
			tui.Text("Press 'q' to quit").Dim(),
		).Padding(1).Border(tui.BorderRounded).Background(tui.BrightBlack),

		// Main content
		tui.HStack(
			// Left panel - user list
			tui.VStack(
				tui.Text("Users").Bold().Underline().Color(tui.Yellow),
				tui.FixedSpacer(1),
				// Dynamic list using iterator!
				tui.Map(users, func(u User) tui.Component {
					status := tui.Text("●").Color(tui.Red)
					if u.Online {
						status = tui.Text("●").Color(tui.Green)
					}
					return tui.HStack(
						status,
						tui.Text(" "+u.Name),
						tui.Spacer(),
						tui.Text(u.Email).Dim(),
					).Gap(1)
				}),
			).Padding(1).Border(tui.BorderSingle).Grow(1),

			// Right panel - controls
			tui.VStack(
				tui.Text("Controls").Bold().Underline().Color(tui.Yellow),
				tui.FixedSpacer(1),
				tui.HStack(
					tui.Text("Counter:"),
					tui.Text("0").Bold().Color(tui.Green).Ref(&countLabel),
				).Gap(1),
				tui.FixedSpacer(1),
				tui.Text("Keys:").Bold(),
				tui.Text("  +/-  Increment/Decrement").Dim(),
				tui.Text("  r    Reset").Dim(),
				tui.Text("  q    Quit").Dim(),
				tui.Spacer(),
				tui.Text("Ready").Color(tui.Green).Ref(&statusLabel),
			).Padding(1).Border(tui.BorderSingle).Grow(1),
		).Gap(1).Grow(1),

		// Footer
		tui.HStack(
			tui.Text("Status: ").Dim(),
			tui.Text("Running").Color(tui.Green),
			tui.Spacer(),
			tui.Text(fmt.Sprintf("%d users loaded", len(users))).Dim(),
		).Padding(1).Border(tui.BorderRounded).Background(tui.BrightBlack),
	).Gap(1).Padding(1)

	app.SetRoot(root)

	// Key bindings with riffkey
	app.Handle("q", func(m riffkey.Match) {
		app.Stop()
	})

	app.Handle("<C-c>", func(m riffkey.Match) {
		app.Stop()
	})

	app.Handle("+", func(m riffkey.Match) {
		counter++
		countLabel.SetText(fmt.Sprintf("%d", counter))
		statusLabel.SetText("Incremented!").Color(tui.Green)
	})

	app.Handle("=", func(m riffkey.Match) {
		counter++
		countLabel.SetText(fmt.Sprintf("%d", counter))
		statusLabel.SetText("Incremented!").Color(tui.Green)
	})

	app.Handle("-", func(m riffkey.Match) {
		counter--
		countLabel.SetText(fmt.Sprintf("%d", counter))
		statusLabel.SetText("Decremented!").Color(tui.Red)
	})

	app.Handle("r", func(m riffkey.Match) {
		counter = 0
		countLabel.SetText("0")
		statusLabel.SetText("Reset!").Color(tui.Yellow)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// User is our sample data type
type User struct {
	Name   string
	Email  string
	Online bool
}
