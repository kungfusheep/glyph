package main

import (
	"fmt"
	"log"
	"time"

	"riffkey"
	. "tui"
)

var (
	tick     int
	gridSize = 4 // Start with 4x4
	mode     = "grid"
)

func buildGridUI() Component {
	count := gridSize * gridSize
	children := make([]ChildItem, count)

	for i := 0; i < count; i++ {
		val := (tick + i*7) % 100
		children[i] = Fragment(
			Textf("Cell %d", i).Bold(),
			Textf("Val: %d%%", val),
			Progress(val, 100),
		)
	}

	return VStack(
		Title(
			fmt.Sprintf("Grid %dx%d (%d fragments) [mode: %s]", gridSize, gridSize, count, mode),
			Text(time.Now().Format("15:04:05")),
		),
		Textf("Keys: a=2x2 | s=4x4 | d=6x6 | f=8x8 | m=next mode | q=quit"),
		Text(TimingString()).Dim(),
		Grid(gridSize, children...).Grow(1),
	).Padding(1)
}

func buildDenseList() Component {
	items := make([]int, 200)
	for i := range items {
		items[i] = i
	}

	return VStack(
		Title("Stress Test: 200 item DataList", Text(time.Now().Format("15:04:05"))),
		Text("Keys: 1-4 grid size | m=mode | q=quit"),
		Fragment(
			DataList(items, func(i int, idx int) Component {
				val := (tick + i*3) % 100
				return HStack(
					Textf("%4d", i),
					Progress(val, 100).Width(20).Chars('>', '.'),
					Textf("%3d%%", val),
				).Gap(2)
			}),
		).Grow(1),
	).Padding(1)
}

func buildNestedUI() Component {
	// Deep nesting stress test
	var inner Component = Text(fmt.Sprintf("Tick: %d", tick)).Bold()

	depth := 20
	for i := 0; i < depth; i++ {
		val := (tick + i*5) % 100
		inner = VStack(
			Textf("Level %d", depth-i),
			Progress(val, 100).Width(15),
			inner,
		)
	}

	return VStack(
		Title("Stress Test: Deep Nesting (20 levels)", Text(time.Now().Format("15:04:05"))),
		Text("Keys: a/s/d/f=grid size | m=next mode | q=quit"),
		Text(TimingString()).Dim(),
		Fragment(inner).Grow(1),
	).Padding(1)
}

func buildComplexUI() Component {
	// Everything combined
	procs := make([]struct {
		Name string
		CPU  int
	}, 30)
	for i := range procs {
		procs[i].Name = fmt.Sprintf("proc-%d", i)
		procs[i].CPU = (tick + i*10) % 100
	}

	return VStack(
		Title("Stress Test: Complex Dashboard", Text(time.Now().Format("15:04:05"))),
		Text("Keys: 1-4 grid size | m=mode | q=quit"),
		Cols2(
			VStack(
				Fragment(
					Text("CPU Cores").Bold(),
					Textf("Core 0: %d%%", (tick+0)%100),
					Progress((tick+0)%100, 100),
					Textf("Core 1: %d%%", (tick+10)%100),
					Progress((tick+10)%100, 100),
					Textf("Core 2: %d%%", (tick+20)%100),
					Progress((tick+20)%100, 100),
					Textf("Core 3: %d%%", (tick+30)%100),
					Progress((tick+30)%100, 100),
					Textf("Core 4: %d%%", (tick+40)%100),
					Progress((tick+40)%100, 100),
					Textf("Core 5: %d%%", (tick+50)%100),
					Progress((tick+50)%100, 100),
					Textf("Core 6: %d%%", (tick+60)%100),
					Progress((tick+60)%100, 100),
					Textf("Core 7: %d%%", (tick+70)%100),
					Progress((tick+70)%100, 100),
				),
				Fragment(
					Text("Memory").Bold(),
					Textf("RAM: %d%%", (tick*2)%100),
					Progress((tick*2)%100, 100),
					Textf("Swap: %d%%", (tick*3)%100),
					Progress((tick*3)%100, 100),
				),
				Fragment(
					Text("Disk").Bold(),
					Textf("Read: %d MB/s", tick%500),
					Textf("Write: %d MB/s", (tick*2)%300),
				),
			).Grow(1),
			Fragment(
				Text("Processes (30)").Bold(),
				DataList(procs, func(p struct {
					Name string
					CPU  int
				}, i int) Component {
					return HStack(
						Textf("%-10s", p.Name),
						Progress(p.CPU, 100).Width(15),
						Textf("%3d%%", p.CPU),
					).Gap(1)
				}),
			),
		).Grow(1),
	).Padding(1)
}

func buildUI() Component {
	switch mode {
	case "grid":
		return buildGridUI()
	case "list":
		return buildDenseList()
	case "nested":
		return buildNestedUI()
	case "complex":
		return buildComplexUI()
	default:
		return buildGridUI()
	}
}

func main() {
	DebugTiming = true // Enable timing display

	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.SetBuildFunc(buildUI)

	// Update at 20 FPS
	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			tick++
			app.RequestRender()
		}
	}()

	// Grid size controls (a/s/d/f for 2/4/6/8 columns)
	app.Handle("a", func(m riffkey.Match) { gridSize = 2; app.RequestRender() })
	app.Handle("s", func(m riffkey.Match) { gridSize = 4; app.RequestRender() })
	app.Handle("d", func(m riffkey.Match) { gridSize = 6; app.RequestRender() })
	app.Handle("f", func(m riffkey.Match) { gridSize = 8; app.RequestRender() })

	// Mode switching
	modes := []string{"grid", "list", "nested", "complex"}
	modeIdx := 0
	app.Handle("m", func(m riffkey.Match) {
		modeIdx = (modeIdx + 1) % len(modes)
		mode = modes[modeIdx]
		app.RequestRender()
	})

	app.Handle("q", func(m riffkey.Match) { app.Stop() })
	app.Handle("<C-c>", func(m riffkey.Match) { app.Stop() })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
