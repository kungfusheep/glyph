package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

func main() {
	selected := -1
	items := []string{"Apple", "Banana", "Cherry", "Date", "Elderberry", "Fig", "Grape"}
	status := "Press 'g' to enter jump mode, 'q' to quit"

	app := NewApp()

	// build UI with Jump-wrapped items
	children := make([]Component, 0, len(items)+4)
	children = append(children, Text("Jump Labels Demo").FG(Cyan).Bold())
	children = append(children, SpaceH(1))

	for i, item := range items {
		idx := i
		children = append(children, Jump(
			Text(fmt.Sprintf("  %s", item)),
			func() {
				selected = idx
				status = fmt.Sprintf("Selected: %s (index %d)", items[idx], idx)
			},
		))
	}

	children = append(children, SpaceH(1))
	children = append(children, Text(&status).FG(Yellow))

	app.SetView(VBox(children...)).
		JumpKey("g").
		Handle("q", func(_ riffkey.Match) {
			app.Stop()
		}).
		Handle("r", func(_ riffkey.Match) {
			selected = -1
			status = "Press 'g' to enter jump mode, 'q' to quit"
		})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

	if selected >= 0 {
		fmt.Printf("Final selection: %s\n", items[selected])
	} else {
		fmt.Println("No selection made")
	}
}
