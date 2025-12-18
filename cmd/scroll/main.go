package main

import (
	"fmt"
	"log"
	"math/rand"

	"riffkey"
	"tui"
)

func main() {
	app, err := tui.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	longJunkString := "sf sdfg sdf g sdfg sdfg kliuhwieurhgiower giu is dfi gsdifu gsuidguis dfui hui sdfui gsui dguih sduihf gsuihd"

	// randomSlice returns a random length slice of longJunkString
	randomSlice := func() string {
		end := rand.Intn(len(longJunkString))
		return string(longJunkString[:end])
	}

	// Generate items
	const itemCount = 100_000_000
	items := make([]string, itemCount)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d - %v", i, randomSlice())
	}

	// Simple render function
	renderItem := func(s string, idx int) tui.Component {
		return tui.Text(s)
	}

	// Create virtual list
	list := tui.NewVirtualList(items, 1, renderItem).
		Border(tui.BorderRounded).
		Grow(1)

	// Status label
	posLabel := tui.Text("Pos: 0")

	// Simple layout
	root := tui.VStack(
		tui.Text("Virtual List Test").Bold(),
		list,
		posLabel,
	).Padding(1)



	app.SetRoot(root)

	updatePos := func() {
		start, end := list.VisibleRange()
		posLabel.SetText(fmt.Sprintf("Pos: %d-%d (rows: %d)", start, end, end-start))
	}

	app.Handle("q", func(m riffkey.Match) { app.Stop() })
	app.Handle("<C-c>", func(m riffkey.Match) { app.Stop() })

	app.Handle("j", func(m riffkey.Match) {
		list.ScrollBy(m.Count)
		updatePos()
	})
	app.Handle("k", func(m riffkey.Match) {
		list.ScrollBy(-m.Count)
		updatePos()
	})
	app.Handle("J", func(m riffkey.Match) {
		list.ScrollBy(10)
		updatePos()
	})
	app.Handle("<C-d>", func(m riffkey.Match) {
		list.ScrollBy(30)
		updatePos()
	})
	app.Handle("<C-u>", func(m riffkey.Match) {
		list.ScrollBy(-30)
		updatePos()
	})
	app.Handle("K", func(m riffkey.Match) {
		list.ScrollBy(-10)
		updatePos()
	})
	app.Handle("g", func(m riffkey.Match) {
		list.ScrollTo(0)
		updatePos()
	})
	app.Handle("G", func(m riffkey.Match) {
		list.ScrollTo(itemCount)
		updatePos()
	})

	updatePos()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
