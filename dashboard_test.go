package tui

import (
	"fmt"
	"testing"
)

func TestGridDashboard(t *testing.T) {
	// Data
	cpu := struct{ Current, Total int }{45, 100}
	mem := struct{ Used, Total int }{8200, 16000}
	net := struct{ Up, Down, Total string }{"1.2 MB/s", "5.4 MB/s", "2.3 GB"}
	procs := []struct{ Name string; CPU int }{
		{"chrome", 25}, {"code", 15}, {"slack", 8}, {"docker", 12},
	}

	// Build tiled dashboard
	root := VStack(
		Title("System Monitor", Text("15:04:32")),
		Cols2(
			// Top-left: CPU
			Fragment(
				Text("CPU Usage").Bold(),
				Textf("  %d%% of %d%%", cpu.Current, cpu.Total),
				Progress(cpu.Current, cpu.Total),
			),
			// Top-right: Memory
			Fragment(
				Text("Memory").Bold(),
				Textf("  %d MB / %d MB", mem.Used, mem.Total),
				Progress(mem.Used, mem.Total),
			),
			// Bottom-left: Network
			Fragment(
				Text("Network").Bold(),
				Textf("  ↑ %s  ↓ %s", net.Up, net.Down),
				Textf("  Total: %s today", net.Total),
			),
			// Bottom-right: Processes
			Fragment(
				Text("Processes").Bold(),
				DataList(procs, func(p struct{ Name string; CPU int }, i int) Component {
					return Textf("  %-10s %3d%%", p.Name, p.CPU)
				}),
			),
		).Grow(1),
	).Padding(1)

	buf := NewBuffer(70, 20)
	root.SetConstraints(70, 20)
	root.Render(buf, 0, 0)

	fmt.Println("=== Tiled Grid Dashboard ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println("============================")
}

func TestDashboardRender(t *testing.T) {
	// Data structure matching the dream code
	type Process struct {
		Name string
		Perc int
	}

	data := struct {
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
	}{
		CPU:    struct{ Current, Total int }{67, 100},
		Memory: struct{ Used, Total int }{8192, 16384},
		Network: struct{ Up, Down, TotalToday string }{
			"1.2 MB/s", "5.4 MB/s", "2.3 GB",
		},
		Processes: []Process{
			{"chrome", 25},
			{"code", 15},
			{"slack", 8},
			{"docker", 12},
			{"node", 5},
		},
	}

	clock := func() Component {
		return Text("15:04:05")
	}

	// BUILD UI - matching dream code style!
	root := Window(
		Title("System Monitor", clock()),
		Text(""),
		Fragment(
			Text("CPU Usage").Bold(),
			Textf("  %d%% of %d%%", data.CPU.Current, data.CPU.Total),
			Progress(data.CPU.Current, data.CPU.Total),
		),
		Text(""),
		Fragment(
			Text("Memory").Bold(),
			Textf("  Used: %d MB / %d MB", data.Memory.Used, data.Memory.Total),
			Progress(data.Memory.Used, data.Memory.Total),
		),
		Text(""),
		Fragment(
			Text("Network").Bold(),
			Textf("  ↑ %s   ↓ %s", data.Network.Up, data.Network.Down),
			Textf("  Total: %s today", data.Network.TotalToday),
		),
		Text(""),
		Fragment(
			Text("Top Processes").Bold(),
			DataList(data.Processes, func(p Process, i int) Component {
				return Textf("  %-12s %3d%%", p.Name, p.Perc)
			}),
		).Grow(1), // Take remaining space
	)

	buf := NewBuffer(50, 25)
	root.SetConstraints(50, 25)
	root.Render(buf, 0, 0)

	fmt.Println("=== Dashboard (Dream Code Style) ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println("=====================================")
}

func TestSimpleComponents(t *testing.T) {
	tests := []struct {
		name   string
		build  func() Component
		width  int
		height int
	}{
		{
			name: "Text",
			build: func() Component {
				return Text("Hello World")
			},
			width: 20, height: 1,
		},
		{
			name: "Bold Text",
			build: func() Component {
				return Text("Bold").Bold()
			},
			width: 10, height: 1,
		},
		{
			name: "VStack",
			build: func() Component {
				return VStack(
					Text("Line 1"),
					Text("Line 2"),
					Text("Line 3"),
				)
			},
			width: 20, height: 5,
		},
		{
			name: "HStack",
			build: func() Component {
				return HStack(
					Text("Left"),
					Spacer(),
					Text("Right"),
				)
			},
			width: 30, height: 1,
		},
		{
			name: "Nested",
			build: func() Component {
				return VStack(
					Text("Header").Bold(),
					HStack(
						Text("A"),
						Text("B"),
						Text("C"),
					).Gap(2),
				)
			},
			width: 20, height: 3,
		},
		{
			name: "Box",
			build: func() Component {
				return VStack(
					Text("In a box"),
				).Border(BorderRounded).Padding(1)
			},
			width: 20, height: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewBuffer(tt.width, tt.height)
			root := tt.build()
			root.SetConstraints(tt.width, tt.height)
			root.Render(buf, 0, 0)

			fmt.Printf("=== %s ===\n", tt.name)
			fmt.Println(buf.StringTrimmed())
			fmt.Println()
		})
	}
}
