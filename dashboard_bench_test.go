package tui

import (
	"fmt"
	"testing"
)

// Benchmark the "dumb" approach: rebuild entire UI tree + render every frame
func BenchmarkDashboardRebuild(b *testing.B) {
	// Simulated dashboard data
	type Process struct {
		Name string
		CPU  int
	}

	type Data struct {
		CPUCurrent int
		CPUTotal   int
		MemUsed    int
		MemTotal   int
		NetUp      string
		NetDown    string
		Processes  []Process
	}

	data := Data{
		CPUCurrent: 45,
		CPUTotal:   100,
		MemUsed:    8192,
		MemTotal:   16384,
		NetUp:      "1.2 MB/s",
		NetDown:    "5.4 MB/s",
		Processes: []Process{
			{"chrome", 25},
			{"code", 15},
			{"slack", 8},
			{"docker", 12},
			{"node", 5},
		},
	}

	progressBar := func(current, total, width int) Component {
		if total == 0 {
			total = 1
		}
		filled := (current * width) / total
		bar := "["
		for i := 0; i < width; i++ {
			if i < filled {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		bar += "]"
		return Text(bar)
	}

	buildUI := func() Component {
		procChildren := make([]ChildItem, len(data.Processes))
		for i, p := range data.Processes {
			procChildren[i] = Text(fmt.Sprintf("%-12s %3d%%", p.Name, p.CPU))
		}

		return VStack(
			// Title
			HStack(
				Text("System Monitor").Bold(),
				Spacer(),
				Text("15:04:05"),
			),
			Text("─────────────────────────────────────────"),
			// CPU
			Text("CPU Usage").Bold(),
			Text(fmt.Sprintf("  %d%% of %d%%", data.CPUCurrent, data.CPUTotal)),
			progressBar(data.CPUCurrent, data.CPUTotal, 30),
			Text(""),
			// Memory
			Text("Memory").Bold(),
			Text(fmt.Sprintf("  %d MB / %d MB", data.MemUsed, data.MemTotal)),
			progressBar(data.MemUsed, data.MemTotal, 30),
			Text(""),
			// Network
			Text("Network").Bold(),
			Text(fmt.Sprintf("  ↑ %s   ↓ %s", data.NetUp, data.NetDown)),
			Text(""),
			// Processes
			Text("Top Processes").Bold(),
			VStack(procChildren...),
		).Padding(1)
	}

	buf := NewBuffer(80, 30)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate data change
		data.CPUCurrent = 30 + (i % 40)
		data.MemUsed = 6000 + (i % 4000)

		// Rebuild entire UI tree
		root := buildUI()

		// Layout + render
		root.SetConstraints(80, 30)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Benchmark with more processes (stress test)
func BenchmarkDashboardRebuildManyProcesses(b *testing.B) {
	type Process struct {
		Name string
		CPU  int
	}

	// 100 processes
	processes := make([]Process, 100)
	for i := range processes {
		processes[i] = Process{
			Name: fmt.Sprintf("process-%d", i),
			CPU:  i % 100,
		}
	}

	buildUI := func() Component {
		procChildren := make([]ChildItem, len(processes))
		for i, p := range processes {
			procChildren[i] = Text(fmt.Sprintf("%-12s %3d%%", p.Name, p.CPU))
		}

		return VStack(
			Text("Processes").Bold(),
			VStack(procChildren...),
		).Padding(1)
	}

	buf := NewBuffer(80, 50)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(80, 50)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// How many frames per second can we achieve?
func BenchmarkDashboardFPS(b *testing.B) {
	type Data struct {
		Value int
	}

	data := Data{Value: 50}

	buildUI := func() Component {
		return VStack(
			Text("Dashboard"),
			Text(fmt.Sprintf("Value: %d", data.Value)),
			Text("[" + string(make([]byte, data.Value)) + "]"), // dynamic width bar
		)
	}

	buf := NewBuffer(120, 40)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data.Value = i % 100
		root := buildUI()
		root.SetConstraints(120, 40)
		buf.Clear()
		root.Render(buf, 0, 0)
	}

	// Report ns/op tells us potential FPS
	// 1,000,000 ns = 1ms = 1000 FPS max
	// 16,666 ns = 16.6µs = 60 FPS target
}
