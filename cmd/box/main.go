package main

import (
	"log"
	"time"

	"riffkey"
	. "tui"
)

var (
	tick    int
	cpuLoad = []int{45, 67, 32, 89}
	mode    = "dashboard"
)

func main() {
	EnableArenaTiming()

	app, err := NewArenaApp()
	if err != nil {
		log.Fatal(err)
	}

	app.SetBuildFunc(func() {
		buildUI()
	})

	// Tick every 100ms
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			tick++
			// Simulate CPU fluctuation
			for i := range cpuLoad {
				cpuLoad[i] = (cpuLoad[i] + (tick*(i+1))%7 - 3 + 100) % 100
			}
			app.RequestRender()
		}
	}()

	// Key handlers
	app.Handle("d", func(m riffkey.Match) { mode = "dashboard" })
	app.Handle("g", func(m riffkey.Match) { mode = "grid" })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildUI() NodeRef {
	switch mode {
	case "grid":
		return buildGrid()
	default:
		return buildDashboard()
	}
}

func buildDashboard() NodeRef {
	return ABox(
		// Header
		ABox(
			AText("Box Layout Demo").Bold(),
			ASpacer(),
			AText(time.Now().Format("15:04:05")),
		).Layout(HorizontalLayout),

		// Timing info
		AText(ArenaTimingString()).Dim(),

		// Main content
		ABox(
			// CPU Panel
			ABox(
				append([]NodeRef{AText("CPU Cores").Bold().Color(Cyan)}, buildCPUBars()...)...,
			).Layout(VerticalLayout).Space(1).BorderRounded().Padding(1).Grow(1).MinWidth(0),

			// Info Panel
			ABox(
				AText("System Info").Bold().Color(Green),
				ABox(AText("Hostname: "), AText("arena-demo")).Layout(HorizontalLayout),
				ABox(AText("Uptime:   "), ATextInt(tick/10), AText("s")).Layout(HorizontalLayout),
				ABox(AText("Mode:     "), AText(mode)).Layout(HorizontalLayout),
				ASpacer(),
				AText("Keys:").Bold(),
				AText("  d = dashboard"),
				AText("  g = grid view"),
				AText("  q = quit"),
			).Layout(VerticalLayout).Space(1).BorderRounded().Padding(1).Grow(1).MinWidth(0),
		).Layout(HorizontalLayout).Space(1).Grow(1),

		// Footer
		ABox(
			AText("Tick: "),
			ATextIntW(tick, 5),
			ASpacer(),
			AText("Zero allocations per frame!").Dim(),
		).Layout(HorizontalLayout),
	).Layout(VerticalLayout).Space(1)
}

func buildCPUBars() []NodeRef {
	bars := make([]NodeRef, len(cpuLoad))
	for i, load := range cpuLoad {
		bars[i] = ABox(
			AText("Core "),
			ATextInt(i),
			AText(": "),
			ATextIntW(load, 3),
			AText("% "),
			AProgress(load, 100).Width(20),
		).Layout(HorizontalLayout)
	}
	return bars
}

func buildGrid() NodeRef {
	// Build grid items
	items := make([]NodeRef, 12)
	for i := range items {
		val := (tick/2 + i*8) % 100
		items[i] = ABox(
			AText("Item "),
			ATextInt(i+1),
			AProgress(val, 100).Width(10),
		).Layout(VerticalLayout).BorderRounded().Padding(1)
	}

	return ABox(
		ABox(
			AText("Grid Layout Demo").Bold(),
			ASpacer(),
			AText(time.Now().Format("15:04:05")),
		).Layout(HorizontalLayout),

		AText(ArenaTimingString()).Dim(),

		ABox(items...).Layout(GridLayout(4)).Space(1).Grow(1),

		ABox(
			AText("d=dashboard g=grid q=quit").Dim(),
		).Layout(HorizontalLayout),
	).Layout(VerticalLayout).Space(1)
}
