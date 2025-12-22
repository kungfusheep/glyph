package tui

import "testing"

func BenchmarkPocFrame(b *testing.B) {
	// Pre-allocate once
	frame := NewPocFrame()
	buf := NewBuffer(50, 10)

	// Warm up pools
	BuildPocFrame(frame, buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		BuildPocFrame(frame, buf)
	}
}

func BenchmarkPocCompileReplay(b *testing.B) {
	// Compile happens once at init (dashboardActions is package-level var)
	replayer := NewReplayer()
	buf := NewBuffer(50, 10)

	// Warm up
	replayer.Reset()
	root := replayer.Replay(dashboardActions)
	replayer.Layout(root, 50, 10)
	replayer.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		replayer.Reset()
		root := replayer.Replay(dashboardActions)
		replayer.Layout(root, 50, 10)
		buf.Clear()
		replayer.Render(buf)
	}
}

func BenchmarkCompiledExecute(b *testing.B) {
	// Build happens once (like at init time)
	compiled := Build(declarativeUI)
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	// Warm up
	ExecuteCompiled(compiled, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteCompiled(compiled, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkDeclarative(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	// Warm up
	ExecuteInto(frame, declarativeUI)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, declarativeUI)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkDeclarativeWithForEach(b *testing.B) {
	// More complex UI with ForEach
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	// Warm up
	ExecuteInto(frame, ui)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, ui)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkCompiledWithForEach(b *testing.B) {
	// Complex UI with ForEach - compiled version
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	// Build once
	compiled := Build(ui)

	// Warm up
	ExecuteCompiled(compiled, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteCompiled(compiled, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

// Benchmark with conditionals and switch statements
func BenchmarkDeclarativeWithConditionals(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DSwitch(&demoData.Mode,
				DCase("normal", DText{Content: "Mode: Normal"}),
				DCase("debug", DText{Content: "Mode: DEBUG"}),
				DDefault(DText{Content: "Mode: Unknown"}),
			),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	ExecuteInto(frame, ui)
	frame.Layout(60, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, ui)
		frame.Layout(60, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkCompiledWithConditionals(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DSwitch(&demoData.Mode,
				DCase("normal", DText{Content: "Mode: Normal"}),
				DCase("debug", DText{Content: "Mode: DEBUG"}),
				DDefault(DText{Content: "Mode: Unknown"}),
			),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	compiled := Build(ui)

	ExecuteCompiled(compiled, frame)
	frame.Layout(60, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteCompiled(compiled, frame)
		frame.Layout(60, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

// === Fast (glint-style) benchmarks ===

func BenchmarkFastExecute(b *testing.B) {
	// Build happens once (like at init time)
	fast := BuildFast(declarativeUI)
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	// Warm up
	ExecuteFast(fast, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteFast(fast, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkFastWithForEach(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	fast := BuildFast(ui)

	ExecuteFast(fast, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteFast(fast, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkFastWithConditionals(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	fast := BuildFast(ui)

	ExecuteFast(fast, frame)
	frame.Layout(60, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteFast(fast, frame)
		frame.Layout(60, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkPocFrameWithCallback(b *testing.B) {
	// Pre-allocate once
	frame := NewPocFrame()
	buf := NewBuffer(50, 10)

	// Warm up
	frame.Reset()
	resetText()
	resetProgress()
	currentPocFrame = frame
	BuildPocFrame(frame, buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		frame.Reset()
		resetText()
		resetProgress()
		currentPocFrame = frame

		// This version uses callback (may allocate)
		sparkData := []int{3, 7, 2, 8, 4, 6, 9, 1, 5}
		sparkline := ACustom(int16(len(sparkData)), 1, func(buf *Buffer, x, y, w, h int16) {
			bars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
			for i, v := range sparkData {
				idx := v * len(bars) / 10
				if idx >= len(bars) {
					idx = len(bars) - 1
				}
				buf.Set(int(x)+i, int(y), NewCell(bars[idx], Style{}))
			}
		})

		root := PocColumn(
			PocText("Dashboard"),
			PocRow(
				PocText("CPU: "),
				PocProgress(75, 100),
			),
			PocRow(
				PocText("Load: "),
				sparkline,
			),
			PocText("Footer"),
		)

		frame.Layout(root, 50, 10)
		buf.Clear()
		frame.Render(buf)
	}
}

// === Zero-allocation (glint-style) benchmarks ===

func BenchmarkZeroExecute(b *testing.B) {
	// Build happens once (like at init time)
	zero := BuildZero(declarativeUI)
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	// Warm up
	ExecuteZero(zero, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteZero(zero, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkZeroWithForEach(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	zero := BuildZero(ui)

	ExecuteZero(zero, frame)
	frame.Layout(50, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteZero(zero, frame)
		frame.Layout(50, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkZeroWithConditionals(b *testing.B) {
	frame := NewDFrame()
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	zero := BuildZero(ui)

	ExecuteZero(zero, frame)
	frame.Layout(60, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteZero(zero, frame)
		frame.Layout(60, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

// === Serial (phased, inlinable) benchmarks ===

func BenchmarkSerialWithForEach(b *testing.B) {
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up
	serial.ExecuteSimple(buf, 50, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Clear()
		serial.ExecuteSimple(buf, 50, 20, nil)
	}
}

func BenchmarkSerialWithConditionals(b *testing.B) {
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up
	serial.ExecuteSimple(buf, 60, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Clear()
		serial.ExecuteSimple(buf, 60, 20, nil)
	}
}

// === No-Clear benchmarks (padded writes, skip Clear()) ===

func BenchmarkSerialNoClearForEach(b *testing.B) {
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up - first frame needs clear
	buf.Clear()
	serial.ExecuteNoClear(buf, 50, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// No Clear() - padded writes overwrite everything
		serial.ExecuteNoClear(buf, 50, 20, nil)
	}
}

func BenchmarkSerialNoClearConditionals(b *testing.B) {
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up
	buf.Clear()
	serial.ExecuteNoClear(buf, 60, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		serial.ExecuteNoClear(buf, 60, 20, nil)
	}
}

// === Dirty-rect clear benchmarks ===

func BenchmarkSerialDirtyForEach(b *testing.B) {
	buf := NewBuffer(50, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up
	buf.Clear()
	serial.ExecuteSimple(buf, 50, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty() // Only clears rows that were written
		serial.ExecuteSimple(buf, 50, 20, nil)
	}
}

func BenchmarkSerialDirtyConditionals(b *testing.B) {
	buf := NewBuffer(60, 20)

	ui := DCol{
		Children: []any{
			DText{Content: &demoData.Title},
			DIf(&demoData.ShowCPU,
				DRow{Children: []any{
					DText{Content: "CPU: "},
					DProgress{Value: &demoData.CPULoad, Width: 20},
				}},
			),
			DElse(DText{Content: "CPU hidden"}),
			DForEach(&demoData.Processes, func(p *DemoProcess) any {
				return DRow{Children: []any{
					DText{Content: &p.Name},
					DProgress{Value: &p.CPU, Width: 15},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	// Warm up
	buf.Clear()
	serial.ExecuteSimple(buf, 60, 20, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 60, 20, nil)
	}
}
