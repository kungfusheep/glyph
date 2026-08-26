package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

func main() {
	selected := -1
	// 32 targets (>27) so GenerateLabels assigns TWO-char labels — that's what
	// makes the incremental feedback visible: type the first key and the matched
	// prefix dims while every non-matching label greys out.
	items := make([]string, 32)
	for i := range items {
		items[i] = fmt.Sprintf("item %02d", i+1)
	}
	status := "press 'g' to jump — then type the first key and watch the prefix dim"

	app := NewApp()

	// lay the targets out as a 4-column grid of Jump cells
	const cols = 4
	rows := (len(items) + cols - 1) / cols
	columns := make([]Component, cols)
	for c := 0; c < cols; c++ {
		cells := make([]Component, 0, rows)
		for r := 0; r < rows; r++ {
			i := r*cols + c
			if i >= len(items) {
				break
			}
			idx := i
			cells = append(cells, Jump(
				Text(fmt.Sprintf("  %s", items[idx])).Width(12),
				func() {
					selected = idx
					status = fmt.Sprintf("selected: %s (index %d)", items[idx], idx)
				},
			))
		}
		columns[c] = VBox(cells...)
	}

	app.SetView(VBox(
		Text("Jump Labels Demo — multi-char feedback").FG(Cyan).Bold(),
		SpaceH(1),
		HBox.Gap(2)(columns...),
		SpaceH(1),
		Text(&status).FG(Yellow),
	)).
		JumpKey("g").
		Handle("q", func(_ riffkey.Match) {
			app.Stop()
		}).
		Handle("r", func(_ riffkey.Match) {
			selected = -1
			status = "press 'g' to jump — then type the first key and watch the prefix dim"
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
