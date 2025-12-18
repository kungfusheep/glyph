package main

import (
	"log"
	"time"

	"riffkey"
	. "tui"
)

var (
	tick     int
	mode     = "dashboard"
	listSize = 50
)

// Simulated data
type Process struct {
	Name string
	CPU  int
	Mem  int
}

var processes = []Process{
	{"nginx", 0, 0},
	{"postgres", 0, 0},
	{"redis", 0, 0},
	{"node", 0, 0},
	{"go-server", 0, 0},
	{"python", 0, 0},
	{"docker", 0, 0},
	{"kubelet", 0, 0},
}

func updateData() {
	for i := range processes {
		processes[i].CPU = (tick + i*13) % 100
		processes[i].Mem = (tick*2 + i*7) % 100
	}
}

func buildDashboard() NodeRef {
	updateData()

	// Build CPU rows
	cpuRows := make([]NodeRef, 8)
	for i := 0; i < 8; i++ {
		cpuRows[i] = AHStack(
			AText("Core "),
			ATextInt(i),
			AText(": "),
			AProgress((tick+i*10)%100, 100),
		)
	}

	// Build process rows
	procRows := make([]NodeRef, len(processes))
	for i, p := range processes {
		procRows[i] = AHStack(
			AText(p.Name),
			ASpacer(),
			ATextIntW(p.CPU, 3), // Fixed width for stable layout
			AText("% "),
			AProgress(p.CPU, 100).Width(15),
		)
	}

	return AVStack(
		// Header
		AHStack(
			AText("Arena Dashboard").Bold(),
			ASpacer(),
			AText(time.Now().Format("15:04:05")),
		),
		// Timing
		AText(ArenaTimingString()).Dim(),
		// Mode info
		AText("Mode: "+mode+" | Keys: d=dashboard l=list s=stress q=quit"),
		// Two columns (MinWidth(0) ensures equal flex distribution)
		AHStack(
			AVStack(append([]NodeRef{AText("CPU Usage").Bold()}, cpuRows...)...).Grow(1).MinWidth(0),
			AVStack(append([]NodeRef{AText("Processes").Bold()}, procRows...)...).Grow(1).MinWidth(0),
		).Grow(1),
		// Footer
		AHStack(
			AText("Tick: "),
			ATextInt(tick),
			ASpacer(),
			AText("Zero allocations!").Dim(),
		),
	)
}

func buildList() NodeRef {
	// Build list items
	items := make([]NodeRef, listSize)
	for i := 0; i < listSize; i++ {
		val := (tick + i*3) % 100
		items[i] = AHStack(
			ATextIntW(i+1, 3),
			AText(". Item "),
			ATextIntW(i, 3),
			ASpacer(),
			AProgress(val, 100).Width(20),
			AText(" "),
			ATextIntW(val, 3),
			AText("%"),
		)
	}

	return AVStack(
		AHStack(
			AText("Arena List - ").Bold(),
			ATextInt(listSize),
			AText(" items"),
			ASpacer(),
			AText(time.Now().Format("15:04:05")),
		),
		AText(ArenaTimingString()).Dim(),
		AText("Keys: +/- size | d=dashboard | q=quit"),
		AVStack(items...).Grow(1),
	)
}

func buildStress() NodeRef {
	// 10x10 grid of progress bars
	rows := make([]NodeRef, 10)
	for row := 0; row < 10; row++ {
		cols := make([]NodeRef, 20) // 10 progress bars + 10 spaces
		for col := 0; col < 10; col++ {
			idx := row*10 + col
			val := (tick + idx*7) % 100
			cols[col*2] = AProgress(val, 100).Width(10)
			cols[col*2+1] = AText(" ")
		}
		rows[row] = AHStack(cols...)
	}

	return AVStack(
		AHStack(
			AText("Arena Stress Test").Bold(),
			ASpacer(),
			AText(time.Now().Format("15:04:05")),
		),
		AText(ArenaTimingString()).Dim(),
		AText("Keys: d=dashboard | q=quit"),
		AVStack(rows...).Grow(1),
		AHStack(
			AText("100 progress bars @ 20 FPS"),
			ASpacer(),
			AText("Tick: "),
			ATextInt(tick),
		),
	)
}

func buildUI() NodeRef {
	switch mode {
	case "dashboard":
		return buildDashboard()
	case "list":
		return buildList()
	case "stress":
		return buildStress()
	default:
		return buildDashboard()
	}
}

func main() {
	EnableArenaTiming()

	app, err := NewArenaApp()
	if err != nil {
		log.Fatal(err)
	}

	// Set build function that returns NodeRef
	app.SetBuildFunc(func() {
		buildUI()
	})

	// Update at 20 FPS
	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			tick++
			app.RequestRender()
		}
	}()

	// Key handlers
	app.Handle("d", func(m riffkey.Match) { mode = "dashboard" })
	app.Handle("l", func(m riffkey.Match) { mode = "list" })
	app.Handle("s", func(m riffkey.Match) { mode = "stress" })

	app.Handle("+", func(m riffkey.Match) {
		listSize += 10
		if listSize > 500 {
			listSize = 500
		}
	})
	app.Handle("-", func(m riffkey.Match) {
		listSize -= 10
		if listSize < 10 {
			listSize = 10
		}
	})

	app.Handle("q", func(m riffkey.Match) { app.Stop() })
	app.Handle("<C-c>", func(m riffkey.Match) { app.Stop() })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
