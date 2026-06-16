package glyph

import (
	"strings"
	"testing"
)

type TestItem struct {
	Name string
	Done bool
}

func TestListCNavigation(t *testing.T) {
	items := []TestItem{
		{Name: "First", Done: false},
		{Name: "Second", Done: true},
		{Name: "Third", Done: false},
	}

	var list *ListC[TestItem]

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).Ref(func(l *ListC[TestItem]) { list = l })

	// Build and execute template to initialize len
	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Verify initial selection is 0
	if list.Index() != 0 {
		t.Errorf("Expected initial index 0, got %d", list.Index())
	}

	// Verify Selected returns correct item
	if list.Selected() == nil {
		t.Fatal("Selected() returned nil")
	}
	if list.Selected().Name != "First" {
		t.Errorf("Expected 'First', got '%s'", list.Selected().Name)
	}

	// Test navigation
	list.Down(nil)
	if list.Index() != 1 {
		t.Errorf("After Down, expected index 1, got %d", list.Index())
	}
	if list.Selected().Name != "Second" {
		t.Errorf("After Down, expected 'Second', got '%s'", list.Selected().Name)
	}

	list.Down(nil)
	if list.Index() != 2 {
		t.Errorf("After second Down, expected index 2, got %d", list.Index())
	}

	// Can't go past end
	list.Down(nil)
	if list.Index() != 2 {
		t.Errorf("Should stay at 2 (end), got %d", list.Index())
	}

	list.Up(nil)
	if list.Index() != 1 {
		t.Errorf("After Up, expected index 1, got %d", list.Index())
	}

	// Test First/Last
	list.Last(nil)
	if list.Index() != 2 {
		t.Errorf("After Last, expected index 2, got %d", list.Index())
	}

	list.First(nil)
	if list.Index() != 0 {
		t.Errorf("After First, expected index 0, got %d", list.Index())
	}
}

func TestListCRendersText(t *testing.T) {
	items := []TestItem{
		{Name: "Apple", Done: false},
		{Name: "Banana", Done: true},
	}

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	})

	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 5)
	tmpl.Execute(buf, 40, 5)

	// Check that text renders correctly
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)

	// Should see marker and text
	if line0 == "" {
		t.Error("Line 0 is empty")
	}
	if !strings.Contains(line0, "Apple") {
		t.Errorf("Line 0 should contain 'Apple', got: %q", line0)
	}
	if !strings.Contains(line1, "Banana") {
		t.Errorf("Line 1 should contain 'Banana', got: %q", line1)
	}
}

func TestListCOnSelect(t *testing.T) {
	items := []TestItem{
		{Name: "First"},
		{Name: "Second"},
		{Name: "Third"},
	}

	var list *ListC[TestItem]
	var selected string
	callCount := 0

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).OnSelect(func(item *TestItem) {
		selected = item.Name
		callCount++
	}).Ref(func(l *ListC[TestItem]) { list = l })

	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// move down, should fire OnSelect
	list.Down(nil)
	if selected != "Second" {
		t.Errorf("OnSelect should receive 'Second', got %q", selected)
	}
	if callCount != 1 {
		t.Errorf("OnSelect should fire once, fired %d times", callCount)
	}

	// move down again
	list.Down(nil)
	if selected != "Third" {
		t.Errorf("OnSelect should receive 'Third', got %q", selected)
	}

	// move down at end, should NOT fire (no change)
	callCount = 0
	list.Down(nil)
	if callCount != 0 {
		t.Errorf("OnSelect should not fire when selection doesn't change, fired %d", callCount)
	}

	// move up
	callCount = 0
	list.Up(nil)
	if selected != "Second" {
		t.Errorf("OnSelect should receive 'Second', got %q", selected)
	}
	if callCount != 1 {
		t.Errorf("OnSelect should fire once on Up, fired %d", callCount)
	}

	// First/Last
	callCount = 0
	list.Last(nil)
	if selected != "Third" {
		t.Errorf("OnSelect should receive 'Third' after Last, got %q", selected)
	}
	list.First(nil)
	if selected != "First" {
		t.Errorf("OnSelect should receive 'First' after First, got %q", selected)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (Last+First), got %d", callCount)
	}
}

func TestListCDelete(t *testing.T) {
	items := []TestItem{
		{Name: "First", Done: false},
		{Name: "Second", Done: true},
		{Name: "Third", Done: false},
	}

	var list *ListC[TestItem]

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).Ref(func(l *ListC[TestItem]) { list = l })

	// Need to compile/execute to set len
	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Delete first item
	list.Delete()
	if len(items) != 2 {
		t.Errorf("After delete, expected 2 items, got %d", len(items))
	}
	if items[0].Name != "Second" {
		t.Errorf("First item should now be 'Second', got '%s'", items[0].Name)
	}
	if list.Index() != 0 {
		t.Errorf("Selection should stay at 0, got %d", list.Index())
	}

	// Move to end and delete
	list.Down(nil)
	if list.Index() != 1 {
		t.Errorf("Expected index 1, got %d", list.Index())
	}
	list.Delete()
	if len(items) != 1 {
		t.Errorf("After second delete, expected 1 item, got %d", len(items))
	}
	// Selection should adjust to stay in bounds
	if list.Index() != 0 {
		t.Errorf("After deleting last item, selection should be 0, got %d", list.Index())
	}
}

// TestListScrollConvergesInOneFrame guards against the one-frame-stale
// blank-bottom: a height-windowed variable-height List must produce its final
// window in a SINGLE render pass after a scroll changes the offset. Previously
// the render-phase scroll-down adjustment set endIdx=selected+1 and left the
// viewport's bottom rows unfilled until the next frame re-rendered from the
// persisted offset (a visible blank flash on every j past the bottom edge).
func TestListScrollConvergesInOneFrame(t *testing.T) {
	const W, H = 24, 14
	type Item struct{ Lines []string }
	items := make([]Item, 18)
	for i := range items {
		n := (i % 4) + 1 // variable heights 1..4 so the window has a partial edge
		ls := make([]string, n)
		for j := range ls {
			ls[j] = strings.Repeat(string(rune('A'+i%26)), 4)
		}
		items[i] = Item{Lines: ls}
	}
	sel := 0
	list := List(&items).MaxVisible(8).Selection(&sel).Render(func(it *Item) Component {
		return VBox(ForEach(&it.Lines, func(s *string) Component { return Text(s) }))
	})
	tmpl := Build(VBox.Height(H)(list.Build()))

	nonblank := func(b *Buffer) int {
		c := 0
		for y := 0; y < H; y++ {
			if strings.TrimRight(extractLine(b, y, W), " \x00") != "" {
				c++
			}
		}
		return c
	}

	// at every step a SINGLE render must already equal a second render of the
	// same state (converged) — in both scroll directions.
	assertConverged := func(label string, s int) {
		first := NewBuffer(W, H)
		tmpl.Execute(first, W, H)
		second := NewBuffer(W, H)
		tmpl.Execute(second, W, H)
		for y := 0; y < H; y++ {
			a := strings.TrimRight(extractLine(first, y, W), " \x00")
			b := strings.TrimRight(extractLine(second, y, W), " \x00")
			if a != b {
				t.Errorf("%s step %d row %d: first render %q != converged render %q (one-frame-stale)", label, s, y, a, b)
			}
		}
		if nb1, nb2 := nonblank(first), nonblank(second); nb1 != nb2 {
			t.Errorf("%s step %d: first render filled %d rows, converged fills %d", label, s, nb1, nb2)
		}
	}

	for s := 0; s < 16; s++ {
		list.cached.Down(nil)
		assertConverged("down", s)
	}
	for s := 0; s < 16; s++ {
		list.cached.Up(nil)
		assertConverged("up", s)
	}
}

func BenchmarkSelectionListScrollRender(b *testing.B) {
	const W, H = 24, 14
	type Item struct{ Lines []string }
	items := make([]Item, 60)
	for i := range items {
		n := (i % 4) + 1
		ls := make([]string, n)
		for j := range ls {
			ls[j] = strings.Repeat(string(rune('A'+i%26)), 4)
		}
		items[i] = Item{Lines: ls}
	}
	sel := 0
	list := List(&items).MaxVisible(8).Selection(&sel).Render(func(it *Item) Component {
		return VBox(ForEach(&it.Lines, func(s *string) Component { return Text(s) }))
	})
	tmpl := Build(VBox.Height(H)(list.Build()))
	buf := NewBuffer(W, H)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, W, H)
		list.cached.Down(nil)
		if sel >= len(items)-1 {
			sel = 0
			list.cached.offset = 0
		}
	}
}
