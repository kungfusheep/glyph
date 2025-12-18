package tui

import (
	"fmt"
	"testing"
)

// Visual tests - see what the stress tests actually look like
func TestStressVisual4x4Grid(t *testing.T) {
	children := make([]ChildItem, 16)
	for i := 0; i < 16; i++ {
		children[i] = Fragment(
			Textf("Cell %d", i).Bold(),
			Textf("Val: %d%%", i*6),
			Progress(i*6, 100),
		)
	}

	root := VStack(
		Title("4x4 Grid Stress Test", Text("12:34:56")),
		Grid(4, children...).Grow(1),
	).Padding(1)

	buf := NewBuffer(100, 30)
	root.SetConstraints(100, 30)
	root.Render(buf, 0, 0)

	fmt.Println("=== 4x4 Grid (16 fragments) ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println()
}

func TestStressVisualComplexDashboard(t *testing.T) {
	procs := make([]struct{ Name string; CPU int }, 10)
	for i := range procs {
		procs[i] = struct{ Name string; CPU int }{
			fmt.Sprintf("proc-%d", i),
			(i * 10) % 100,
		}
	}

	root := VStack(
		Title("Complex Dashboard", Text("12:34:56")),
		Cols2(
			VStack(
				Fragment(
					Text("CPU").Bold(),
					Textf("Core 0: %d%%", 45),
					Progress(45, 100),
					Textf("Core 1: %d%%", 67),
					Progress(67, 100),
				),
				Fragment(
					Text("Memory").Bold(),
					Progress(50, 100),
				),
			).Grow(1),
			Fragment(
				Text("Processes").Bold(),
				DataList(procs, func(p struct{ Name string; CPU int }, i int) Component {
					return Textf("%-10s %3d%%", p.Name, p.CPU)
				}),
			),
		).Grow(1),
	).Padding(1)

	buf := NewBuffer(80, 25)
	root.SetConstraints(80, 25)
	root.Render(buf, 0, 0)

	fmt.Println("=== Complex Dashboard ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println()
}

// Stress test: Dense grid of fragments
func BenchmarkDenseGrid4x4(b *testing.B) {
	benchmarkDenseGrid(b, 4, 4)
}

func BenchmarkDenseGrid8x8(b *testing.B) {
	benchmarkDenseGrid(b, 8, 8)
}

func BenchmarkDenseGrid16x16(b *testing.B) {
	benchmarkDenseGrid(b, 16, 16)
}

func benchmarkDenseGrid(b *testing.B, cols, rows int) {
	count := cols * rows

	buildUI := func() Component {
		children := make([]ChildItem, count)
		for i := 0; i < count; i++ {
			children[i] = Fragment(
				Text(fmt.Sprintf("Cell %d", i)).Bold(),
				Textf("Value: %d", i*100),
				Progress(i%100, 100),
			)
		}
		return Grid(cols, children...)
	}

	buf := NewBuffer(200, 100)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(200, 100)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Stress test: Deep nesting
func BenchmarkDeepNesting10(b *testing.B) {
	benchmarkDeepNesting(b, 10)
}

func BenchmarkDeepNesting50(b *testing.B) {
	benchmarkDeepNesting(b, 50)
}

func BenchmarkDeepNesting100(b *testing.B) {
	benchmarkDeepNesting(b, 100)
}

func benchmarkDeepNesting(b *testing.B, depth int) {
	buildUI := func() Component {
		var root Component = Text("Innermost")
		for i := 0; i < depth; i++ {
			root = VStack(
				Text(fmt.Sprintf("Level %d", depth-i)),
				root,
			).Padding(1)
		}
		return root
	}

	buf := NewBuffer(200, 200)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(200, 200)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Stress test: Many text components
func BenchmarkManyTexts100(b *testing.B) {
	benchmarkManyTexts(b, 100)
}

func BenchmarkManyTexts500(b *testing.B) {
	benchmarkManyTexts(b, 500)
}

func BenchmarkManyTexts1000(b *testing.B) {
	benchmarkManyTexts(b, 1000)
}

func BenchmarkManyTexts5000(b *testing.B) {
	benchmarkManyTexts(b, 5000)
}

func benchmarkManyTexts(b *testing.B, count int) {
	buildUI := func() Component {
		children := make([]ChildItem, count)
		for i := 0; i < count; i++ {
			children[i] = Textf("Line %d: This is some text content that varies per line", i)
		}
		return VStack(children...)
	}

	buf := NewBuffer(100, 50)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(100, 50)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Stress test: Complex dashboard with everything
func BenchmarkComplexDashboard(b *testing.B) {
	type Process struct {
		Name string
		CPU  int
		Mem  int
	}

	procs := make([]Process, 50)
	for i := range procs {
		procs[i] = Process{
			Name: fmt.Sprintf("process-%d", i),
			CPU:  i % 100,
			Mem:  (i * 100) % 16384,
		}
	}

	buildUI := func() Component {
		return VStack(
			Title("System Dashboard", Text("12:34:56")),
			Cols2(
				// Left column - multiple fragments stacked
				VStack(
					Fragment(
						Text("CPU Usage").Bold(),
						Textf("Core 0: %d%%", 45),
						Progress(45, 100),
						Textf("Core 1: %d%%", 67),
						Progress(67, 100),
						Textf("Core 2: %d%%", 23),
						Progress(23, 100),
						Textf("Core 3: %d%%", 89),
						Progress(89, 100),
					),
					Fragment(
						Text("Memory").Bold(),
						Textf("Used: %d MB / %d MB", 8192, 16384),
						Progress(8192, 16384),
						Textf("Swap: %d MB / %d MB", 1024, 4096),
						Progress(1024, 4096),
					),
					Fragment(
						Text("Disk I/O").Bold(),
						Textf("Read:  %s", "125 MB/s"),
						Textf("Write: %s", "45 MB/s"),
						Progress(125, 200),
					),
				).Grow(1),
				// Right column - process list
				Fragment(
					Text("Processes (50)").Bold(),
					DataList(procs, func(p Process, i int) Component {
						return HStack(
							Textf("%-15s", p.Name),
							Textf("%3d%%", p.CPU),
							Textf("%6d MB", p.Mem),
						).Gap(2)
					}),
				),
			).Grow(1),
			// Bottom status bar
			HStack(
				Text("Status: Running"),
				Spacer(),
				Text("Uptime: 3d 14h 22m"),
				Spacer(),
				Text("Load: 2.45"),
			),
		).Padding(1)
	}

	buf := NewBuffer(120, 60)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(120, 60)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Stress test: Full screen buffer operations
func BenchmarkFullScreenRender80x24(b *testing.B) {
	benchmarkFullScreen(b, 80, 24)
}

func BenchmarkFullScreenRender120x40(b *testing.B) {
	benchmarkFullScreen(b, 120, 40)
}

func BenchmarkFullScreenRender200x60(b *testing.B) {
	benchmarkFullScreen(b, 200, 60)
}

func BenchmarkFullScreenRender300x100(b *testing.B) {
	benchmarkFullScreen(b, 300, 100)
}

func benchmarkFullScreen(b *testing.B, width, height int) {
	// Fill entire screen with content
	rows := height - 2
	buildUI := func() Component {
		children := make([]ChildItem, rows)
		for i := 0; i < rows; i++ {
			children[i] = Textf("%4d | %-*s |", i, width-12, "Content that fills the row")
		}
		return VStack(children...).Border(BorderSingle)
	}

	buf := NewBuffer(width, height)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := buildUI()
		root.SetConstraints(width, height)
		buf.Clear()
		root.Render(buf, 0, 0)
	}
}

// Stress test: Buffer clear + write (raw operation)
func BenchmarkBufferClearWrite80x24(b *testing.B) {
	buf := NewBuffer(80, 24)
	cell := NewCell('X', DefaultStyle())
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Clear()
		for y := 0; y < 24; y++ {
			for x := 0; x < 80; x++ {
				buf.Set(x, y, cell)
			}
		}
	}
}

func BenchmarkBufferClearWrite200x60(b *testing.B) {
	buf := NewBuffer(200, 60)
	cell := NewCell('X', DefaultStyle())
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Clear()
		for y := 0; y < 60; y++ {
			for x := 0; x < 200; x++ {
				buf.Set(x, y, cell)
			}
		}
	}
}

// Stress test: String rendering (what the terminal actually receives)
func BenchmarkBufferToString80x24(b *testing.B) {
	buf := NewBuffer(80, 24)
	// Fill with content
	for y := 0; y < 24; y++ {
		buf.WriteString(0, y, "This is a line of text that fills the buffer width.", DefaultStyle())
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buf.String()
	}
}

func BenchmarkBufferToString200x60(b *testing.B) {
	buf := NewBuffer(200, 60)
	for y := 0; y < 60; y++ {
		buf.WriteString(0, y, "This is a line of text that fills the buffer width, and it keeps going to be longer.", DefaultStyle())
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buf.String()
	}
}

// Compare: VirtualList vs building all components
func BenchmarkVirtualListVsFullList1000(b *testing.B) {
	items := make([]string, 1000)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d with some extra text", i)
	}

	b.Run("VirtualList", func(b *testing.B) {
		render := func(s string, i int) Component {
			return Text(s)
		}
		buf := NewBuffer(80, 40)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			list := NewVirtualList(items, 1, render)
			list.SetConstraints(80, 40)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})

	b.Run("FullList", func(b *testing.B) {
		buf := NewBuffer(80, 40)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			children := make([]ChildItem, len(items))
			for j, item := range items {
				children[j] = Text(item)
			}
			list := VStack(children...)
			list.SetConstraints(80, 40)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})
}

func BenchmarkVirtualListVsFullList10000(b *testing.B) {
	items := make([]string, 10000)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d with some extra text", i)
	}

	b.Run("VirtualList", func(b *testing.B) {
		render := func(s string, i int) Component {
			return Text(s)
		}
		buf := NewBuffer(80, 40)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			list := NewVirtualList(items, 1, render)
			list.SetConstraints(80, 40)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})

	b.Run("FullList", func(b *testing.B) {
		buf := NewBuffer(80, 40)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			children := make([]ChildItem, len(items))
			for j, item := range items {
				children[j] = Text(item)
			}
			list := VStack(children...)
			list.SetConstraints(80, 40)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})
}
