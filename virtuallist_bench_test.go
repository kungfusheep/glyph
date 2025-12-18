package tui

import (
	"fmt"
	"testing"
)

// Benchmark continuous scrolling - the real test
func BenchmarkVirtualListScroll(b *testing.B) {
	type Item struct {
		ID     int
		Name   string
		Value  int
		Status string
	}

	// Generate large dataset
	makeItems := func(n int) []Item {
		items := make([]Item, n)
		for i := range items {
			items[i] = Item{
				ID:     i,
				Name:   fmt.Sprintf("Item %d with some longer text", i),
				Value:  i * 100,
				Status: []string{"active", "pending", "done"}[i%3],
			}
		}
		return items
	}

	render := func(item Item, idx int) Component {
		style := DefaultStyle()
		if item.Status == "active" {
			style = style.Foreground(Green)
		} else if item.Status == "pending" {
			style = style.Foreground(Yellow)
		}
		return HStack(
			Text(fmt.Sprintf("%5d", item.ID)),
			Text(item.Name).Style(style),
			Text(fmt.Sprintf("%d", item.Value)),
		).Gap(2)
	}

	sizes := []int{1000, 10000, 100000, 1000000}

	for _, size := range sizes {
		items := makeItems(size)

		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			list := NewVirtualList(items, 1, render).Border(BorderRounded)
			buf := NewBuffer(120, 50)

			// Initial layout
			list.SetConstraints(120, 50)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Simulate continuous scrolling - scroll down then up
				list.ScrollBy(1)
				list.SetConstraints(120, 50)
				list.Render(buf, 0, 0)
			}
		})
	}
}

// Benchmark rapid scroll (page up/down style)
func BenchmarkVirtualListPageScroll(b *testing.B) {
	items := make([]string, 100000)
	for i := range items {
		items[i] = fmt.Sprintf("Row %d: Lorem ipsum dolor sit amet", i)
	}

	render := func(s string, idx int) Component {
		return Text(s)
	}

	list := NewVirtualList(items, 1, render).Border(BorderRounded)
	buf := NewBuffer(120, 50)
	list.SetConstraints(120, 50)

	pageSize := 48 // viewport rows

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Page down then page up
		if i%2 == 0 {
			list.ScrollBy(pageSize)
		} else {
			list.ScrollBy(-pageSize)
		}
		list.SetConstraints(120, 50)
		list.Render(buf, 0, 0)
	}
}

// Benchmark: scroll + full frame including buffer operations
func BenchmarkVirtualListFullFrame(b *testing.B) {
	items := make([]string, 100000)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d", i)
	}

	render := func(s string, idx int) Component {
		return Text(s)
	}

	list := NewVirtualList(items, 1, render)
	buf := NewBuffer(120, 50)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Scroll
		list.ScrollTo((i * 100) % len(items))

		// Full frame: constraints + render + clear
		list.SetConstraints(120, 50)
		buf.Clear()
		list.Render(buf, 0, 0)
	}
}

// Benchmark: compare virtual vs non-virtual for same visible output
func BenchmarkVirtualVsRegular(b *testing.B) {
	items := make([]string, 10000)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d", i)
	}

	buf := NewBuffer(120, 50)

	b.Run("VirtualList", func(b *testing.B) {
		render := func(s string, idx int) Component {
			return Text(s)
		}
		list := NewVirtualList(items, 1, render)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			list.ScrollTo(5000) // Middle of list
			list.SetConstraints(120, 50)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})

	b.Run("RegularList_AllItems", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Create ALL components (naive approach)
			children := make([]ChildItem, len(items))
			for j, item := range items {
				children[j] = Text(item)
			}
			list := VStack(children...)
			list.SetConstraints(120, 50)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})

	b.Run("RegularList_Sliced", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Only create visible components (manual culling)
			visible := items[5000:5050]
			children := make([]ChildItem, len(visible))
			for j, item := range visible {
				children[j] = Text(item)
			}
			list := VStack(children...)
			list.SetConstraints(120, 50)
			buf.Clear()
			list.Render(buf, 0, 0)
		}
	})
}

// Test: ensure virtual list renders correctly
func TestVirtualListRender(t *testing.T) {
	items := []string{"One", "Two", "Three", "Four", "Five"}
	render := func(s string, idx int) Component {
		return Text(s)
	}

	list := NewVirtualList(items, 1, render)
	buf := NewBuffer(20, 5)

	list.SetConstraints(20, 5)
	list.Render(buf, 0, 0)

	// Check first item is rendered
	line := ""
	for x := 0; x < 5; x++ {
		c := buf.Get(x, 0)
		if c.Rune != 0 {
			line += string(c.Rune)
		}
	}
	if line != "One  " && line != "One" {
		t.Errorf("expected 'One', got %q", line)
	}
}

// Test: scroll behavior
func TestVirtualListScroll(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = fmt.Sprintf("Item%d", i)
	}

	render := func(s string, idx int) Component {
		return Text(s)
	}

	list := NewVirtualList(items, 1, render)
	list.SetConstraints(20, 10) // 10 rows visible

	// Initial state
	start, end := list.VisibleRange()
	if start != 0 || end != 10 {
		t.Errorf("initial range: expected 0-10, got %d-%d", start, end)
	}

	// Scroll down
	list.ScrollBy(5)
	start, end = list.VisibleRange()
	if start != 5 || end != 15 {
		t.Errorf("after scroll: expected 5-15, got %d-%d", start, end)
	}

	// Scroll to end
	list.ScrollTo(90)
	start, end = list.VisibleRange()
	if start != 90 || end != 100 {
		t.Errorf("at end: expected 90-100, got %d-%d", start, end)
	}

	// Can't scroll past end
	list.ScrollBy(100)
	start, _ = list.VisibleRange()
	if start != 90 {
		t.Errorf("past end: expected start=90, got %d", start)
	}
}
