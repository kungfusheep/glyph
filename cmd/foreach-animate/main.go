package main

import (
	"fmt"
	"log"
	"time"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

type row struct {
	Label    string
	Active   bool
	Selected bool
}

func main() {
	app := NewApp()

	items := []row{
		{Label: "alpha", Active: true, Selected: true},
		{Label: "bravo", Active: true},
		{Label: "charlie", Active: true},
		{Label: "delta", Active: true},
		{Label: "echo", Active: true},
		{Label: "foxtrot", Active: true},
	}
	selected := 0
	auto := false
	tick := 0

	bg := Hex(0x151515)
	panel := Hex(0x202020)
	text := Hex(0xd8d8d8)
	muted := Hex(0x686868)
	idle := Hex(0x262626)
	hot := Hex(0x3f6f5c)
	selectedBG := Hex(0x42345c)
	smooth := Animate.Duration(450 * time.Millisecond).Ease(EaseOutCubic)

	app.SetView(
		VBox.Fill(bg).Gap(1)(
			HBox.Gap(2)(
				Text("foreach animate harness").Bold().FG(text),
				Text("j/k select").FG(muted),
				Text("space remove/restore").FG(muted),
				Text("a auto").FG(muted),
				Text("r reset").FG(muted),
				Text("q quit").FG(muted),
			),
			VBox.Width(54).Fill(panel).PaddingVH(1, 2)(
				ForEach(&items, func(item *row) any {
					rowBG := smooth(If(&item.Selected).Then(selectedBG).Else(
						If(&item.Active).Then(hot).Else(idle),
					))
					labelFG := smooth(If(&item.Active).Then(Hex(0xf2d184)).Else(text))
					return If(&item.Active).Then(
						VBox.Height(
							In(int16(1)).Out(Animate.Duration(700 * time.Millisecond).Ease(EaseOutCubic)(int16(0))),
						)(
							HBox.Fill(rowBG).Gap(1).Opacity(
								In(1.0).Out(Animate.Duration(700*time.Millisecond).Ease(EaseOutCubic)(0.0)),
							)(
								Text(If(&item.Selected).Then(">").Else(" ")).Width(1).FG(labelFG),
								Text(&item.Label).Width(12).FG(labelFG),
								Text("visible").FG(muted),
								Text("yes").FG(labelFG),
							),
						),
					)
				}),
			),
			Text(fmt.Sprintf("selected: %d   auto: %v   tick: %d", selected+1, auto, tick)).FG(muted),
		),
	)

	app.OnBeforeRender(func() {
		if selected < 0 {
			selected = 0
		}
		if selected >= len(items) {
			selected = len(items) - 1
		}
		for i := range items {
			items[i].Selected = i == selected
		}
	})

	app.Handle("j", func(_ riffkey.Match) {
		if selected < len(items)-1 {
			selected++
		}
	})
	app.Handle("k", func(_ riffkey.Match) {
		if selected > 0 {
			selected--
		}
	})
	app.Handle("<Space>", func(_ riffkey.Match) {
		items[selected].Active = !items[selected].Active
	})
	app.Handle("a", func(_ riffkey.Match) {
		auto = !auto
	})
	app.Handle("r", func(_ riffkey.Match) {
		for i := range items {
			items[i].Active = true
		}
	})
	app.Handle("q", func(_ riffkey.Match) {
		app.Stop()
	})
	app.Handle("<Escape>", func(_ riffkey.Match) {
		app.Stop()
	})

	go func() {
		ticker := time.NewTicker(700 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if !auto {
				continue
			}
			tick++
			idx := tick % len(items)
			items[idx].Active = !items[idx].Active
			app.RequestRender()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
