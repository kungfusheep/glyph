package tui

import (
	"fmt"
	"testing"
)

// Simulate a data item
type BenchItem struct {
	Name   string
	Value  int
	Active bool
}

// Generate test data
func generateItems(n int) []BenchItem {
	items := make([]BenchItem, n)
	for i := range items {
		items[i] = BenchItem{
			Name:   fmt.Sprintf("Item %d", i),
			Value:  i * 10,
			Active: i%2 == 0,
		}
	}
	return items
}

// Benchmark: Pure component tree creation (no render)
func BenchmarkTreeCreation(b *testing.B) {
	items := generateItems(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = createTree(items)
	}
}

func createTree(items []BenchItem) Component {
	children := make([]ChildItem, len(items))
	for i, item := range items {
		children[i] = createRow(item)
	}
	return VStack(children...)
}

func createRow(item BenchItem) Component {
	style := DefaultStyle()
	if item.Active {
		style = style.Bold().Foreground(Green)
	}
	return HStack(
		Text(item.Name).Style(style),
		Text(fmt.Sprintf("%d", item.Value)),
	)
}

// Benchmark: Full render to buffer
func BenchmarkFullRender(b *testing.B) {
	items := generateItems(1000)
	buf := NewBuffer(120, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := createTree(items)
		tree.SetConstraints(120, 50)
		buf.Clear()
		tree.Render(buf, 0, 0)
	}
}

// Benchmark: Buffer diff (simulating screen flush)
func BenchmarkBufferDiff(b *testing.B) {
	buf1 := NewBuffer(120, 50)
	buf2 := NewBuffer(120, 50)

	// Fill with some content
	style := DefaultStyle()
	for y := 0; y < 50; y++ {
		buf1.WriteString(0, y, fmt.Sprintf("Line %d: some content here that fills the buffer", y), style)
		buf2.WriteString(0, y, fmt.Sprintf("Line %d: some content here that fills the buffer", y), style)
	}

	// Make small changes in buf2
	buf2.WriteString(10, 25, "CHANGED", style.Bold())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		changes := 0
		for y := 0; y < 50; y++ {
			for x := 0; x < 120; x++ {
				c1 := buf1.Get(x, y)
				c2 := buf2.Get(x, y)
				if c1.Rune != c2.Rune || c1.Style != c2.Style {
					changes++
				}
			}
		}
		_ = changes
	}
}

// Benchmark: Viewport culling - only render visible items
func BenchmarkViewportCulling(b *testing.B) {
	items := generateItems(10000) // 10k items
	buf := NewBuffer(120, 50)
	viewportHeight := 50
	scrollOffset := 5000 // Middle of list

	b.Run("WithoutCulling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tree := createTree(items)
			tree.SetConstraints(120, 50)
			buf.Clear()
			tree.Render(buf, 0, 0)
		}
	})

	b.Run("WithCulling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Only create visible items
			visibleStart := scrollOffset
			visibleEnd := scrollOffset + viewportHeight
			if visibleEnd > len(items) {
				visibleEnd = len(items)
			}
			visibleItems := items[visibleStart:visibleEnd]

			tree := createTree(visibleItems)
			tree.SetConstraints(120, 50)
			buf.Clear()
			tree.Render(buf, 0, 0)
		}
	})
}

// Benchmark: Single item update in large list
func BenchmarkSingleItemUpdate(b *testing.B) {
	items := generateItems(1000)
	buf := NewBuffer(120, 50)

	b.Run("FullRebuild", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			items[500].Value++ // Change one item
			tree := createTree(items)
			tree.SetConstraints(120, 50)
			buf.Clear()
			tree.Render(buf, 0, 0)
		}
	})

	// Simulate targeted update (just re-render one row)
	b.Run("TargetedUpdate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			items[500].Value++
			row := createRow(items[500])
			row.SetConstraints(120, 1)
			row.Render(buf, 0, 25) // Render at fixed position
		}
	})
}

// Benchmark: Memory allocations
func BenchmarkAllocations(b *testing.B) {
	items := generateItems(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := createTree(items)
		tree.SetConstraints(120, 50)
		_ = tree
	}
}

// Benchmark: Allocations with pooling (release after each iteration)
func BenchmarkAllocationsWithPool(b *testing.B) {
	items := generateItems(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := createTree(items)
		tree.SetConstraints(120, 50)
		ReleaseTree(tree) // Return to pool
	}
}

// Benchmark: Tree creation with pooling
func BenchmarkTreeCreationWithPool(b *testing.B) {
	items := generateItems(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := createTree(items)
		ReleaseTree(tree)
	}
}

// Benchmark: Full render with pooling
func BenchmarkFullRenderWithPool(b *testing.B) {
	items := generateItems(1000)
	buf := NewBuffer(120, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := createTree(items)
		tree.SetConstraints(120, 50)
		buf.Clear()
		tree.Render(buf, 0, 0)
		ReleaseTree(tree)
	}
}

// Benchmark: Compare List[T] vs fresh slice each time
func BenchmarkListVsSlice(b *testing.B) {
	items := generateItems(100)

	b.Run("FreshSlice", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			children := make([]*TextComponent, len(items))
			for j, item := range items {
				children[j] = Text(item.Name)
			}
		}
	})

	b.Run("ReuseList", func(b *testing.B) {
		list := NewList[*TextComponent]()
		// Pre-populate
		for _, item := range items {
			list.Add(Text(item.Name))
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Update existing items
			for j, item := range items {
				if tc := list.At(j); tc != nil {
					tc.SetText(item.Name)
				}
			}
		}
	})
}
