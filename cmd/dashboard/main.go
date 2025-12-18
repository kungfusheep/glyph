package main

import (
	"fmt"
	"log"
	"time"

	"riffkey"
	"tui"
)

// Dashboard data - updated by background goroutines
var data struct {
	CPU struct {
		Current int
		Total   int
	}
	Memory struct {
		Used  int
		Total int
	}
	Network struct {
		Up         string
		Down       string
		TotalToday string
	}
	Processes []Process
}

type Process struct {
	Name string
	CPU  int
}

func init() {
	data.CPU.Total = 100
	data.CPU.Current = 45
	data.Memory.Total = 16384
	data.Memory.Used = 8192
	data.Network.Up = "1.2 MB/s"
	data.Network.Down = "5.4 MB/s"
	data.Network.TotalToday = "2.3 GB"
	data.Processes = []Process{
		{"chrome", 25},
		{"code", 15},
		{"slack", 8},
		{"docker", 12},
		{"node", 5},
		{"spotify", 3},
	}
}

func buildUI() tui.Component {
	clock := tui.Text(time.Now().Format("15:04:05"))

	return tui.VStack(
		tui.Title("System Monitor", clock),
		tui.Cols2(
			// CPU
			tui.Fragment(
				tui.Text("CPU Usage").Bold(),
				tui.Textf("  %d%% of %d%%", data.CPU.Current, data.CPU.Total),
				tui.Progress(data.CPU.Current, data.CPU.Total),
			),
			// Memory
			tui.Fragment(
				tui.Text("Memory").Bold(),
				tui.Textf("  %d MB / %d MB", data.Memory.Used, data.Memory.Total),
				tui.Progress(data.Memory.Used, data.Memory.Total),
			),
			// Network
			tui.Fragment(
				tui.Text("Network").Bold(),
				tui.Textf("  ↑ %s  ↓ %s", data.Network.Up, data.Network.Down),
				tui.Textf("  Total: %s today", data.Network.TotalToday),
			),
			// Processes
			tui.Fragment(
				tui.Text("Processes").Bold(),
				tui.DataList(data.Processes, func(p Process, i int) tui.Component {
					return tui.Textf("  %-10s %3d%%", p.Name, p.CPU)
				}),
			),
		).Grow(1),
	).Padding(1)
}

func main() {
	app, err := tui.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.SetBuildFunc(buildUI)

	// Update data in background - smooth animation
	go func() {
		tick := 0
		for {
			time.Sleep(50 * time.Millisecond)
			tick++

			// Smooth incrementing values that wrap
			data.CPU.Current = tick % 100
			data.Memory.Used = (tick * 100) % data.Memory.Total

			// Network counters
			data.Network.Up = fmt.Sprintf("%.1f MB/s", float64(tick%50)/10)
			data.Network.Down = fmt.Sprintf("%.1f MB/s", float64(tick%100)/10)

			// Rotate process CPU values
			for i := range data.Processes {
				data.Processes[i].CPU = (tick + i*10) % 100
			}

			app.RequestRender()
		}
	}()

	app.Handle("q", func(m riffkey.Match) { app.Stop() })
	app.Handle("<C-c>", func(m riffkey.Match) { app.Stop() })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
