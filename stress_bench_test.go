package tui

import "testing"

// Stress test data
var stressData = struct {
	Title     string
	Items     []StressItem
	BigGrid   [][]int
	WideRatio float32
}{
	Title:     "Stress Test Dashboard",
	WideRatio: 0.67,
}

type StressItem struct {
	Name  string
	Value int
	CPU   float32
}

func init() {
	// 100 items for ForEach stress
	stressData.Items = make([]StressItem, 100)
	for i := range stressData.Items {
		stressData.Items[i] = StressItem{
			Name:  "process-" + string(rune('A'+i%26)) + string(rune('0'+i%10)),
			Value: i * 7 % 100,
			CPU:   float32(i%100) / 100.0,
		}
	}

	// 20x20 grid for grid stress
	stressData.BigGrid = make([][]int, 20)
	for i := range stressData.BigGrid {
		stressData.BigGrid[i] = make([]int, 20)
		for j := range stressData.BigGrid[i] {
			stressData.BigGrid[i][j] = (i + j) % 100
		}
	}
}

// BenchmarkStress100Items - 100 ForEach items
func BenchmarkStress100Items(b *testing.B) {
	buf := NewBuffer(80, 120)

	ui := DCol{
		Children: []any{
			DText{Content: &stressData.Title},
			DForEach(&stressData.Items, func(item *StressItem) any {
				return DRow{Children: []any{
					DText{Content: &item.Name},
					DProgress{Value: &item.CPU, Width: 30},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	buf.Clear()
	serial.ExecuteSimple(buf, 80, 120, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 80, 120, nil)
	}
}

// BenchmarkStressWideProgress - 100-char wide progress bars
func BenchmarkStressWideProgress(b *testing.B) {
	buf := NewBuffer(120, 30)

	items := make([]StressItem, 20)
	for i := range items {
		items[i] = StressItem{
			Name: "wide-" + string(rune('A'+i)),
			CPU:  float32(i) / 20.0,
		}
	}

	ui := DCol{
		Children: []any{
			DText{Content: "Wide Progress Bars"},
			DForEach(&items, func(item *StressItem) any {
				return DRow{Children: []any{
					DText{Content: &item.Name},
					DProgress{Value: &item.CPU, Width: 100},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	buf.Clear()
	serial.ExecuteSimple(buf, 120, 30, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 120, 30, nil)
	}
}

// BenchmarkStressDenseGrid - Many small items in a grid pattern
func BenchmarkStressDenseGrid(b *testing.B) {
	buf := NewBuffer(100, 50)

	// 10x10 grid of progress bars
	rows := make([][]StressItem, 10)
	for i := range rows {
		rows[i] = make([]StressItem, 10)
		for j := range rows[i] {
			rows[i][j] = StressItem{
				CPU: float32((i*10+j)%100) / 100.0,
			}
		}
	}

	ui := DCol{
		Children: []any{
			DText{Content: "Dense Grid"},
			DForEach(&rows, func(row *[]StressItem) any {
				return DRow{Children: []any{
					DForEach(row, func(item *StressItem) any {
						return DProgress{Value: &item.CPU, Width: 8}
					}),
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	buf.Clear()
	serial.ExecuteSimple(buf, 100, 50, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 100, 50, nil)
	}
}

// BenchmarkStressHeavyDashboard - Everything combined
func BenchmarkStressHeavyDashboard(b *testing.B) {
	buf := NewBuffer(120, 80)

	// Multiple sections with different content
	cpuCores := stressData.Items[:8]
	memBanks := stressData.Items[8:16]
	procList := stressData.Items[16:50]

	ui := DCol{
		Children: []any{
			DText{Content: &stressData.Title},
			DText{Content: "═══════════════════════════════════════════════════════════════"},
			DText{Content: "CPU Cores"},
			DForEach(&cpuCores, func(item *StressItem) any {
				return DRow{Children: []any{
					DText{Content: &item.Name},
					DProgress{Value: &item.CPU, Width: 50},
				}}
			}),
			DText{Content: ""},
			DText{Content: "Memory Banks"},
			DForEach(&memBanks, func(item *StressItem) any {
				return DRow{Children: []any{
					DText{Content: &item.Name},
					DProgress{Value: &item.CPU, Width: 50},
				}}
			}),
			DText{Content: ""},
			DText{Content: "Process List"},
			DForEach(&procList, func(item *StressItem) any {
				return DRow{Children: []any{
					DText{Content: &item.Name},
					DProgress{Value: &item.CPU, Width: 40},
				}}
			}),
		},
	}

	serial := BuildSerial(ui)

	buf.Clear()
	serial.ExecuteSimple(buf, 120, 80, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 120, 80, nil)
	}
}

// BenchmarkStressTextHeavy - Lots of text, minimal progress bars
func BenchmarkStressTextHeavy(b *testing.B) {
	buf := NewBuffer(100, 60)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "This is line number " + string(rune('0'+i/10)) + string(rune('0'+i%10)) + " with some extra text to make it longer and stress the text rendering path"
	}

	ui := DCol{
		Children: []any{
			DText{Content: "Text Heavy Benchmark"},
			DForEach(&lines, func(line *string) any {
				return DText{Content: line}
			}),
		},
	}

	serial := BuildSerial(ui)

	buf.Clear()
	serial.ExecuteSimple(buf, 100, 60, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.ClearDirty()
		serial.ExecuteSimple(buf, 100, 60, nil)
	}
}
