package tui

import "testing"

// Benchmark: Arena tree creation (should be zero alloc after warmup)
func BenchmarkArenaTreeCreation(b *testing.B) {
	// Pre-allocate frame with enough capacity
	frame := NewFrame(5000, 50000)

	// Items to render (pre-allocated)
	type item struct {
		Name  string
		Value int
	}
	items := make([]item, 1000)
	for i := range items {
		items[i] = item{Name: "Item", Value: i * 10}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		frame.Build(func() {
			rows := make([]NodeRef, len(items))
			for j, item := range items {
				rows[j] = AHStack(
					AText(item.Name),
					ASpacer(),
					ATextInt(item.Value),
				)
			}
			AVStack(rows...)
		})
	}
}

// Benchmark: Arena full render
func BenchmarkArenaFullRender(b *testing.B) {
	frame := NewFrame(5000, 50000)
	buf := NewBuffer(120, 50)

	type item struct {
		Name  string
		Value int
	}
	items := make([]item, 1000)
	for i := range items {
		items[i] = item{Name: "Item", Value: i * 10}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		frame.Build(func() {
			rows := make([]NodeRef, len(items))
			for j, item := range items {
				rows[j] = AHStack(
					AText(item.Name),
					ASpacer(),
					ATextInt(item.Value),
				)
			}
			AVStack(rows...)
		})
		frame.Layout(120, 50)
		buf.Clear()
		frame.Render(buf)
	}
}

// Benchmark: Compare pooled vs arena
func BenchmarkPooledVsArena(b *testing.B) {
	type item struct {
		Name  string
		Value int
	}
	items := make([]item, 1000)
	for i := range items {
		items[i] = item{Name: "Item", Value: i * 10}
	}

	buf := NewBuffer(120, 50)

	b.Run("Pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tree := VStack(
				Map(items, func(item item) Component {
					return HStack(
						Text(item.Name),
						Spacer(),
						Text(string(rune('0' + item.Value%10))),
					)
				}),
			)
			tree.SetConstraints(120, 50)
			buf.Clear()
			tree.Render(buf, 0, 0)
			ReleaseTree(tree)
		}
	})

	b.Run("Arena", func(b *testing.B) {
		frame := NewFrame(5000, 50000)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			frame.Build(func() {
				rows := make([]NodeRef, len(items))
				for j, item := range items {
					rows[j] = AHStack(
						AText(item.Name),
						ASpacer(),
						ATextInt(item.Value),
					)
				}
				AVStack(rows...)
			})
			frame.Layout(120, 50)
			buf.Clear()
			frame.Render(buf)
		}
	})
}

// Benchmark: Zero-alloc verification
func BenchmarkArenaZeroAlloc(b *testing.B) {
	frame := NewFrame(100, 1000)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		frame.Build(func() {
			AVStack(
				AText("Header"),
				AHStack(
					AText("Left"),
					ASpacer(),
					AText("Right"),
				),
				AProgress(50, 100),
			)
		})
		frame.Layout(80, 24)
	}
}
