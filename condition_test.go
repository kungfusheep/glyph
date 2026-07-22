package glyph

import (
	"strings"
	"testing"
)

func TestConditionEq(t *testing.T) {
	t.Run("If comparable Eq true", func(t *testing.T) {
		val := 5
		cond := If(&val).Eq(5)
		if !cond.evaluate() {
			t.Error("expected condition to be true when val == 5")
		}
	})

	t.Run("If comparable Ne", func(t *testing.T) {
		val := 5
		cond := If(&val).Ne(10)
		if !cond.evaluate() {
			t.Error("expected condition to be true when val != 10")
		}
	})
}

func TestOrdCondition(t *testing.T) {
	t.Run("Gt true", func(t *testing.T) {
		val := 10
		cond := IfOrd(&val).Gt(5)
		if !cond.evaluate() {
			t.Error("expected 10 > 5 to be true")
		}
	})

	t.Run("Gt false", func(t *testing.T) {
		val := 3
		cond := IfOrd(&val).Gt(5)
		if cond.evaluate() {
			t.Error("expected 3 > 5 to be false")
		}
	})

	t.Run("Lt true", func(t *testing.T) {
		val := 3
		cond := IfOrd(&val).Lt(5)
		if !cond.evaluate() {
			t.Error("expected 3 < 5 to be true")
		}
	})

	t.Run("Gte", func(t *testing.T) {
		val := 5
		cond := IfOrd(&val).Gte(5)
		if !cond.evaluate() {
			t.Error("expected 5 >= 5 to be true")
		}
	})

	t.Run("Lte", func(t *testing.T) {
		val := 5
		cond := IfOrd(&val).Lte(5)
		if !cond.evaluate() {
			t.Error("expected 5 <= 5 to be true")
		}
	})
}

func TestConditionThenElse(t *testing.T) {
	t.Run("Then branch accessible", func(t *testing.T) {
		val := true
		cond := If(&val).Eq(true).Then("yes").Else("no")
		if cond.getThen() != "yes" {
			t.Error("expected then to be 'yes'")
		}
		if cond.getElse() != "no" {
			t.Error("expected else to be 'no'")
		}
	})

	t.Run("Evaluates dynamically", func(t *testing.T) {
		val := 0
		cond := IfOrd(&val).Eq(0)

		if !cond.evaluate() {
			t.Error("expected true when val == 0")
		}

		val = 1
		if cond.evaluate() {
			t.Error("expected false when val == 1")
		}

		val = 0
		if !cond.evaluate() {
			t.Error("expected true again when val == 0")
		}
	})
}

func TestConditionInSerialTemplate(t *testing.T) {
	t.Run("If renders correct branch", func(t *testing.T) {
		activeLayer := 0

		view := VBox(
			IfOrd(&activeLayer).Eq(0).Then(Text("LAYER0")).Else(Text("OTHER")),
			IfOrd(&activeLayer).Eq(1).Then(Text("LAYER1")).Else(Text("OTHER")),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		// Check first line has "LAYER0"
		line0 := extractLine(buf, 0, 10)
		if line0 != "LAYER0    " {
			t.Errorf("expected 'LAYER0    ', got %q", line0)
		}

		// Check second line has "OTHER" (since activeLayer != 1)
		line1 := extractLine(buf, 1, 10)
		if line1 != "OTHER     " {
			t.Errorf("expected 'OTHER     ', got %q", line1)
		}

		// Now change activeLayer and re-render
		activeLayer = 1
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		// Check first line now has "OTHER"
		line0 = extractLine(buf, 0, 10)
		if line0 != "OTHER     " {
			t.Errorf("after change: expected 'OTHER     ', got %q", line0)
		}

		// Check second line now has "LAYER1"
		line1 = extractLine(buf, 1, 10)
		if line1 != "LAYER1    " {
			t.Errorf("after change: expected 'LAYER1    ', got %q", line1)
		}
	})
}

// extractLine returns the text content from a buffer row
func extractLine(buf *Buffer, row, width int) string {
	result := make([]rune, width)
	for x := 0; x < width; x++ {
		result[x] = buf.Get(x, row).Rune
	}
	return string(result)
}

func TestConditionInForEachWithPointerElements(t *testing.T) {
	type item struct {
		Name   string
		Active bool
	}
	items := []*item{
		{"alpha", true},
		{"beta", false},
	}

	view := VBox(
		ForEach(&items, func(e **item) Component {
			return HBox(
				Text(&(*e).Name),
				If(&(*e).Active).Then(Text(" YES")).Else(Text(" NO!")),
			)
		}),
	)

	tmpl := Build(view)
	buf := NewBuffer(40, 5)
	tmpl.Execute(buf, 40, 5)

	line0 := extractLine(buf, 0, 20)
	if !strings.Contains(line0, "YES") {
		t.Errorf("expected line 0 to contain 'YES', got %q", line0)
	}
	line1 := extractLine(buf, 1, 20)
	if !strings.Contains(line1, "NO!") {
		t.Errorf("expected line 1 to contain 'NO!', got %q", line1)
	}

	// mutate source and re-render — should reflect without Refresh
	items[0].Active = false
	items[1].Active = true

	buf.Clear()
	tmpl.Execute(buf, 40, 5)

	line0 = extractLine(buf, 0, 20)
	if !strings.Contains(line0, "NO!") {
		t.Errorf("after mutation: expected line 0 to contain 'NO!', got %q", line0)
	}
	line1 = extractLine(buf, 1, 20)
	if !strings.Contains(line1, "YES") {
		t.Errorf("after mutation: expected line 1 to contain 'YES', got %q", line1)
	}
}

func TestConditionInFilterListRender(t *testing.T) {
	type item struct {
		Name   string
		Active bool
	}
	items := []item{
		{"alpha", true},
		{"beta", false},
	}

	fl := FilterList(&items, func(e *item) string { return e.Name }).
		MaxVisible(5).
		Render(func(e *item) Component {
			return HBox(
				Text(&e.Name),
				If(&e.Active).Then(Text(" YES")).Else(Text(" NO!")),
			)
		})

	view := VBox(fl)
	tmpl := Build(view)
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// dump all lines for debugging
	for row := 0; row < 8; row++ {
		t.Logf("row %d: %q", row, extractLine(buf, row, 30))
	}

	// items render after the input+counter, find them
	found := false
	for row := 0; row < 8; row++ {
		line := extractLine(buf, row, 30)
		if strings.Contains(line, "alpha") {
			found = true
			if !strings.Contains(line, "YES") {
				t.Errorf("alpha row should contain YES, got %q", line)
			}
			break
		}
	}
	if !found {
		t.Fatal("could not find 'alpha' in any row")
	}

	// mutate source and re-render
	items[0].Active = false
	items[1].Active = true

	buf.Clear()
	tmpl.Execute(buf, 40, 10)

	for row := 0; row < 8; row++ {
		line := extractLine(buf, row, 30)
		if strings.Contains(line, "alpha") {
			if !strings.Contains(line, "NO!") {
				t.Errorf("after mutation: alpha row should contain NO!, got %q", line)
			}
			break
		}
	}
}

func TestSwitch(t *testing.T) {
	t.Run("Switch string matches case", func(t *testing.T) {
		tab := "home"
		sw := Switch(&tab).
			Case("home", Text("HOME_VIEW")).
			Case("settings", Text("SETTINGS_VIEW")).
			Default(Text("DEFAULT_VIEW"))

		if sw.getMatchIndex() != 0 {
			t.Errorf("expected match index 0, got %d", sw.getMatchIndex())
		}
		if sw.evaluateSwitch() == nil {
			t.Errorf("expected HOME_VIEW component, got nil")
		}
	})

	t.Run("Switch string matches second case", func(t *testing.T) {
		tab := "settings"
		sw := Switch(&tab).
			Case("home", Text("HOME_VIEW")).
			Case("settings", Text("SETTINGS_VIEW")).
			Default(Text("DEFAULT_VIEW"))

		if sw.getMatchIndex() != 1 {
			t.Errorf("expected match index 1, got %d", sw.getMatchIndex())
		}
		if sw.evaluateSwitch() == nil {
			t.Errorf("expected SETTINGS_VIEW component, got nil")
		}
	})

	t.Run("Switch falls through to default", func(t *testing.T) {
		tab := "unknown"
		sw := Switch(&tab).
			Case("home", Text("HOME_VIEW")).
			Case("settings", Text("SETTINGS_VIEW")).
			Default(Text("DEFAULT_VIEW"))

		if sw.getMatchIndex() != -1 {
			t.Errorf("expected match index -1, got %d", sw.getMatchIndex())
		}
		if sw.evaluateSwitch() == nil {
			t.Errorf("expected DEFAULT_VIEW component, got nil")
		}
	})

	t.Run("Switch int type", func(t *testing.T) {
		mode := 2
		sw := Switch(&mode).
			Case(1, Text("MODE_ONE")).
			Case(2, Text("MODE_TWO")).
			Default(Text("MODE_DEFAULT"))

		if sw.getMatchIndex() != 1 {
			t.Errorf("expected match index 1, got %d", sw.getMatchIndex())
		}
	})

	t.Run("Switch evaluates dynamically", func(t *testing.T) {
		tab := "home"
		sw := Switch(&tab).
			Case("home", Text("HOME")).
			Case("settings", Text("SETTINGS")).
			Default(Text("DEFAULT"))

		if sw.getMatchIndex() != 0 {
			t.Errorf("expected match index 0, got %d", sw.getMatchIndex())
		}

		tab = "settings"
		if sw.getMatchIndex() != 1 {
			t.Errorf("expected match index 1 after change, got %d", sw.getMatchIndex())
		}

		tab = "other"
		if sw.getMatchIndex() != -1 {
			t.Errorf("expected default index for unknown, got %d", sw.getMatchIndex())
		}
	})
}

func TestSwitchInSerialTemplate(t *testing.T) {
	t.Run("Switch renders correct case", func(t *testing.T) {
		tab := "home"

		view := VBox(
			Switch(&tab).
				Case("home", Text("HOME_CONTENT")).
				Case("settings", Text("SETTINGS_CONTENT")).
				Default(Text("DEFAULT_CONTENT")),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 0, 15)
		if line != "HOME_CONTENT   " {
			t.Errorf("expected 'HOME_CONTENT   ', got %q", line)
		}

		// Change tab and re-render
		tab = "settings"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		line = extractLine(buf, 0, 18)
		if line != "SETTINGS_CONTENT  " {
			t.Errorf("expected 'SETTINGS_CONTENT  ', got %q", line)
		}

		// Change to unknown tab
		tab = "unknown"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		line = extractLine(buf, 0, 17)
		if line != "DEFAULT_CONTENT  " {
			t.Errorf("expected 'DEFAULT_CONTENT  ', got %q", line)
		}
	})

	t.Run("Switch renders mixed component branches", func(t *testing.T) {
		tab := "details"

		view := VBox(
			Text("HEADER"),
			Switch(&tab).
				Case("summary", Text("SUMMARY")).
				Case("details", HBox(Text("DETAIL"), Text("S"))).
				Default(VBox(Text("FALL"), Text("BACK"))),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 1, 10)
		if line != "DETAILS   " {
			t.Errorf("expected HBox branch to render DETAILS, got %q", line)
		}

		tab = "other"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		line = extractLine(buf, 1, 10)
		if line != "FALL      " {
			t.Errorf("expected default VBox first row, got %q", line)
		}
		line = extractLine(buf, 2, 10)
		if line != "BACK      " {
			t.Errorf("expected default VBox second row, got %q", line)
		}
	})

	t.Run("Switch scalar branches drive height", func(t *testing.T) {
		mode := "short"

		tmpl := Build(VBox.Height(
			Switch(&mode).
				Case("short", int16(1)).
				Case("tall", int16(3)).
				Default(int16(2)),
		)(
			Text("one"),
			Text("two"),
			Text("three"),
		))
		buf := NewBuffer(20, 5)

		tmpl.Execute(buf, 20, 5)
		if got := tmpl.geom[0].H; got != 1 {
			t.Errorf("mode=short: got H=%d, want 1", got)
		}

		mode = "tall"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)
		if got := tmpl.geom[0].H; got != 3 {
			t.Errorf("mode=tall: got H=%d, want 3", got)
		}

		mode = "other"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)
		if got := tmpl.geom[0].H; got != 2 {
			t.Errorf("mode=other: got H=%d, want 2", got)
		}
	})
}

func TestSelectionList(t *testing.T) {
	type Item struct {
		Name string
	}

	t.Run("renders items with selection marker", func(t *testing.T) {
		items := []Item{{Name: "One"}, {Name: "Two"}, {Name: "Three"}}
		selected := 1

		list := &selectionList{
			Items:    &items,
			Selected: &selected,
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Row 0: "  One" (not selected)
		// Row 1: "> Two" (selected)
		// Row 2: "  Three" (not selected)
		line0 := extractLine(buf, 0, 7)
		line1 := extractLine(buf, 1, 7)
		line2 := extractLine(buf, 2, 9)

		if line0 != "  One  " {
			t.Errorf("row 0: expected '  One  ', got %q", line0)
		}
		if line1 != "> Two  " {
			t.Errorf("row 1: expected '> Two  ', got %q", line1)
		}
		if line2 != "  Three  " {
			t.Errorf("row 2: expected '  Three  ', got %q", line2)
		}
	})

	t.Run("selection changes dynamically", func(t *testing.T) {
		items := []Item{{Name: "A"}, {Name: "B"}}
		selected := 0

		list := &selectionList{
			Items:    &items,
			Selected: &selected,
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		line0 := extractLine(buf, 0, 4)
		line1 := extractLine(buf, 1, 4)
		if line0 != "> A " {
			t.Errorf("before: row 0 expected '> A ', got %q", line0)
		}
		if line1 != "  B " {
			t.Errorf("before: row 1 expected '  B ', got %q", line1)
		}

		// Change selection
		selected = 1
		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		line0 = extractLine(buf, 0, 4)
		line1 = extractLine(buf, 1, 4)
		if line0 != "  A " {
			t.Errorf("after: row 0 expected '  A ', got %q", line0)
		}
		if line1 != "> B " {
			t.Errorf("after: row 1 expected '> B ', got %q", line1)
		}
	})

	t.Run("custom marker", func(t *testing.T) {
		items := []Item{{Name: "X"}}
		selected := 0

		list := &selectionList{
			Items:    &items,
			Selected: &selected,
			Marker:   "→ ",
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// "→ " is 4 bytes but 2 display chars - for now we just check it renders
		line0 := extractLine(buf, 0, 6)
		// The arrow character might take different width, just check it's not "> "
		if line0 == "> X   " {
			t.Errorf("custom marker not applied, got default")
		}
	})

	t.Run("helper methods", func(t *testing.T) {
		items := []Item{{Name: "A"}, {Name: "B"}, {Name: "C"}}
		selected := 0

		list := &selectionList{
			Items:    &items,
			Selected: &selected,
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		// Need to render once to populate len
		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Test Down
		list.Down(nil)
		if selected != 1 {
			t.Errorf("Down: expected selected=1, got %d", selected)
		}

		// Test Down again
		list.Down(nil)
		if selected != 2 {
			t.Errorf("Down again: expected selected=2, got %d", selected)
		}

		// Test Down at end (should stay at 2)
		list.Down(nil)
		if selected != 2 {
			t.Errorf("Down at end: expected selected=2, got %d", selected)
		}

		// Test Up
		list.Up(nil)
		if selected != 1 {
			t.Errorf("Up: expected selected=1, got %d", selected)
		}

		// Test First
		list.First(nil)
		if selected != 0 {
			t.Errorf("First: expected selected=0, got %d", selected)
		}

		// Test Last
		list.Last(nil)
		if selected != 2 {
			t.Errorf("Last: expected selected=2, got %d", selected)
		}

		// Test Up at start
		selected = 0
		list.Up(nil)
		if selected != 0 {
			t.Errorf("Up at start: expected selected=0, got %d", selected)
		}
	})

	t.Run("MaxVisible limits displayed items", func(t *testing.T) {
		items := []Item{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
		selected := 0

		list := &selectionList{
			Items:      &items,
			Selected:   &selected,
			MaxVisible: 3, // Only show 3 items at a time
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Should show items 0-2 (A, B, C)
		line0 := extractLine(buf, 0, 4)
		line1 := extractLine(buf, 1, 4)
		line2 := extractLine(buf, 2, 4)
		line3 := extractLine(buf, 3, 4) // Should be empty

		if line0 != "> A " {
			t.Errorf("row 0: expected '> A ', got %q", line0)
		}
		if line1 != "  B " {
			t.Errorf("row 1: expected '  B ', got %q", line1)
		}
		if line2 != "  C " {
			t.Errorf("row 2: expected '  C ', got %q", line2)
		}
		if line3 != "    " {
			t.Errorf("row 3: expected empty, got %q", line3)
		}
	})

	t.Run("viewport scrolls with selection", func(t *testing.T) {
		items := []Item{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
		selected := 0

		list := &selectionList{
			Items:      &items,
			Selected:   &selected,
			MaxVisible: 3,
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Move down past visible window
		list.Down(nil) // 0 -> 1
		list.Down(nil) // 1 -> 2
		list.Down(nil) // 2 -> 3 (should scroll)

		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		// Now viewport should show B, C, D (items 1-3) with D selected
		line0 := extractLine(buf, 0, 4)
		line1 := extractLine(buf, 1, 4)
		line2 := extractLine(buf, 2, 4)

		if line0 != "  B " {
			t.Errorf("after scroll: row 0 expected '  B ', got %q", line0)
		}
		if line1 != "  C " {
			t.Errorf("after scroll: row 1 expected '  C ', got %q", line1)
		}
		if line2 != "> D " {
			t.Errorf("after scroll: row 2 expected '> D ', got %q", line2)
		}
	})

	t.Run("viewport scrolls up", func(t *testing.T) {
		items := []Item{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
		selected := 4 // Start at end

		list := &selectionList{
			Items:      &items,
			Selected:   &selected,
			MaxVisible: 3,
			Render: func(item *Item) Component {
				return Text(&item.Name)
			},
		}

		view := VBox(list)
		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Should show C, D, E (items 2-4) with E selected
		line2 := extractLine(buf, 2, 4)
		if line2 != "> E " {
			t.Errorf("initial: row 2 expected '> E ', got %q", line2)
		}

		// Move up to beginning
		list.First(nil)
		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		// Now viewport should show A, B, C (items 0-2) with A selected
		line0 := extractLine(buf, 0, 4)
		if line0 != "> A " {
			t.Errorf("after First: row 0 expected '> A ', got %q", line0)
		}
	})
}

func TestConditionInsideForEach(t *testing.T) {
	t.Run("If evaluates per element in ForEach", func(t *testing.T) {
		type Item struct {
			Name     string
			Selected bool
		}
		items := []Item{
			{Name: "Alpha", Selected: false},
			{Name: "Beta", Selected: true},
			{Name: "Gamma", Selected: false},
		}

		view := VBox(
			ForEach(&items, func(item *Item) Component {
				return If(&item.Selected).Eq(true).
					Then(Text(&item.Name).Bold()).
					Else(Text(&item.Name))
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// All items should render correctly
		line0 := extractLine(buf, 0, 7)
		line1 := extractLine(buf, 1, 7)
		line2 := extractLine(buf, 2, 7)

		if line0 != "Alpha  " {
			t.Errorf("row 0: expected 'Alpha  ', got %q", line0)
		}
		if line1 != "Beta   " {
			t.Errorf("row 1: expected 'Beta   ', got %q", line1)
		}
		if line2 != "Gamma  " {
			t.Errorf("row 2: expected 'Gamma  ', got %q", line2)
		}

		// Change selection and re-render
		items[0].Selected = true
		items[1].Selected = false
		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		// Items should still render (just with different styles)
		line0 = extractLine(buf, 0, 7)
		line1 = extractLine(buf, 1, 7)
		if line0 != "Alpha  " {
			t.Errorf("after change: row 0 expected 'Alpha  ', got %q", line0)
		}
		if line1 != "Beta   " {
			t.Errorf("after change: row 1 expected 'Beta   ', got %q", line1)
		}
	})

	t.Run("If with same component different style", func(t *testing.T) {
		type Item struct {
			Text     string
			IsActive bool
		}
		items := []Item{
			{Text: "Inactive", IsActive: false},
			{Text: "Active", IsActive: true},
		}

		view := VBox(
			ForEach(&items, func(item *Item) Component {
				return If(&item.IsActive).Eq(true).
					Then(Text(&item.Text).Bold()).
					Else(Text(&item.Text).Dim())
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Both items should render their text correctly
		line0 := extractLine(buf, 0, 10)
		line1 := extractLine(buf, 1, 10)

		if line0 != "Inactive  " {
			t.Errorf("row 0: expected 'Inactive  ', got %q", line0)
		}
		if line1 != "Active    " {
			t.Errorf("row 1: expected 'Active    ', got %q", line1)
		}

		// Flip the active states
		items[0].IsActive = true
		items[1].IsActive = false
		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		// Content should be the same (styles would differ if we checked them)
		line0 = extractLine(buf, 0, 10)
		line1 = extractLine(buf, 1, 10)
		if line0 != "Inactive  " {
			t.Errorf("after flip: row 0 expected 'Inactive  ', got %q", line0)
		}
		if line1 != "Active    " {
			t.Errorf("after flip: row 1 expected 'Active    ', got %q", line1)
		}
	})
}

// The headline leak case: a per-item If inside a ForEach over an append/realloc slice.
// When the slice's backing array reallocates, every element address (the opIf.itemBranches
// key) changes; the bounded perItemCache must (a) not leak the orphaned selectors and
// (b) keep each item rendering its OWN branch at the new addresses — never bleeding a stale
// selector into a different item.
func TestIfInForEachSurvivesSliceRealloc(t *testing.T) {
	type Row struct {
		Name string
		On   bool
	}
	// start small so the first appends are guaranteed to reallocate the backing array.
	rows := make([]Row, 0, 2)
	rows = append(rows, Row{Name: "r0", On: true}, Row{Name: "r1", On: false})

	view := VBox(
		ForEach(&rows, func(r *Row) Component {
			return If(&r.On).Eq(true).
				Then(Text(&r.Name).Bold()).
				Else(Text(&r.Name))
		}),
	)
	tmpl := Build(view)
	buf := NewBuffer(12, 64)
	tmpl.Execute(buf, 12, 64)

	// locate the opIf so we can assert its per-item cache stays bounded under churn.
	// the per-item If lives in the ForEach's iterTmpl, not the top-level ops.
	var findIf func(tt *Template) *opIf
	findIf = func(tt *Template) *opIf {
		if tt == nil {
			return nil
		}
		for i := range tt.ops {
			switch e := tt.ops[i].Ext.(type) {
			case *opIf:
				return e
			case *opForEach:
				if c := findIf(e.iterTmpl); c != nil {
					return c
				}
			}
		}
		return nil
	}
	theIf := findIf(tmpl)
	if theIf == nil {
		t.Fatal("no opIf found in compiled template")
	}

	// churn: append many rows over many frames, forcing repeated reallocation. Each new row
	// alternates On so its own branch is checkable; re-render every frame so live items stay
	// stamped and orphans accumulate then evict.
	const total = perItemCacheCap + 200
	for i := 2; i < total; i++ {
		rows = append(rows, Row{Name: "x", On: i%2 == 0})
		tmpl.Execute(buf, 12, 64)
	}

	// (a) bounded: the orphaned selectors from every realloc must have been swept.
	if n := len(theIf.itemBranches.m); n > perItemCacheCap {
		t.Fatalf("opIf.itemBranches leaked: %d entries (cap %d) after churning %d rows",
			n, perItemCacheCap, total)
	}

	// (b) correct own-values: the first two rows keep their distinct names and branches at
	// whatever addresses the final backing array gave them.
	tmpl.Execute(buf, 12, 64)
	if got := extractLine(buf, 0, 2); got != "r0" {
		t.Errorf("row 0 name after realloc = %q, want r0", got)
	}
	if got := extractLine(buf, 1, 2); got != "r1" {
		t.Errorf("row 1 name after realloc = %q, want r1", got)
	}
	// r0.On=true -> bold; r1.On=false -> not bold. A bled-in stale selector would cross these.
	if buf.Get(0, 0).Style.Attr&AttrBold == 0 {
		t.Error("row 0 (On=true) should be bold — its own Then branch")
	}
	if buf.Get(0, 1).Style.Attr&AttrBold != 0 {
		t.Error("row 1 (On=false) must NOT be bold — stale selector bled across items")
	}
}

func TestBareForEachAsConditionBranch(t *testing.T) {
	// a bare ForEach used directly as an If branch (no wrapping container) must
	// lay out and render per outer element. the inner slice is a field offset on
	// a value-slice element, so it rebases per row (a per-row fold shape).
	type Comment struct {
		Open    bool
		Body    []string
		Preview []string
	}
	cs := []Comment{
		{Open: true, Body: []string{"b1", "b2"}, Preview: []string{"p1"}},
		{Open: false, Body: []string{"x1", "x2"}, Preview: []string{"prev2"}},
	}
	view := VBox(ForEach(&cs, func(c *Comment) Component {
		return If(&c.Open).Eq(true).
			Then(ForEach(&c.Body, func(s *string) Component { return Text(s) })).
			Else(ForEach(&c.Preview, func(s *string) Component { return Text(s) }))
	}))
	tmpl := Build(view)
	buf := NewBuffer(20, 10)
	tmpl.Execute(buf, 20, 10)

	// row 0 is open -> its Body (b1,b2); the closed row shows its Preview (prev2)
	want := []string{"b1", "b2", "prev2"}
	for i, w := range want {
		got := strings.TrimRight(extractLine(buf, i, 8), " \x00")
		if got != w {
			t.Errorf("row %d: expected %q, got %q", i, w, got)
		}
	}

	// flip fold state and re-render: each row's own sub-slice follows
	cs[0].Open = false
	cs[1].Open = true
	buf.Clear()
	tmpl.Execute(buf, 20, 10)
	want = []string{"p1", "x1", "x2"}
	for i, w := range want {
		got := strings.TrimRight(extractLine(buf, i, 8), " \x00")
		if got != w {
			t.Errorf("after flip row %d: expected %q, got %q", i, w, got)
		}
	}
}

func TestForEachInIfInsideListRow(t *testing.T) {
	// per-row fold inside a List, two sibling Ifs each
	// wrapping a ForEach over a per-row sub-slice. List has its own iter path,
	// so this guards that the root-level ForEach branch layout reaches it too.
	type Row struct{ Spans []Span }
	type RowVM struct {
		Open        bool
		Closed      bool
		BodyRows    []Row
		PreviewRows []Row
	}
	mk := func(s string) []Span { return []Span{{Text: s}} }
	vms := []RowVM{
		{Open: true, Closed: false,
			BodyRows:    []Row{{mk("bodyA")}, {mk("bodyB")}},
			PreviewRows: []Row{{mk("prevA")}}},
		{Open: false, Closed: true,
			BodyRows:    []Row{{mk("bodyX")}},
			PreviewRows: []Row{{mk("prevX")}}},
	}
	view := VBox(
		List(&vms).Render(func(c *RowVM) Component {
			return VBox(
				If(&c.Open).Then(ForEach(&c.BodyRows, func(r *Row) Component { return Rich(&r.Spans).CharWrap() })),
				If(&c.Closed).Then(ForEach(&c.PreviewRows, func(r *Row) Component { return Rich(&r.Spans).CharWrap() })),
			)
		}),
	)
	tmpl := Build(view)
	buf := NewBuffer(20, 10)
	tmpl.Execute(buf, 20, 10)

	// open row -> its BodyRows; closed row -> its PreviewRows. each rebases.
	// row 0 carries the List selection marker "> ".
	checks := []struct {
		row  int
		want string
	}{{0, "bodyA"}, {1, "bodyB"}, {2, "prevX"}}
	for _, c := range checks {
		got := strings.TrimRight(extractLine(buf, c.row, 12), " \x00")
		if !strings.Contains(got, c.want) {
			t.Errorf("row %d: expected to contain %q, got %q", c.row, c.want, got)
		}
	}
}

func TestHBoxLayout(t *testing.T) {
	t.Run("HBox places children horizontally", func(t *testing.T) {
		view := HBox(
			Text("AAA"),
			Text("BBB"),
			Text("CCC"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		// All three texts should be on row 0, horizontally adjacent
		line := extractLine(buf, 0, 12)
		if line != "AAABBBCCC   " {
			t.Errorf("expected 'AAABBBCCC   ', got %q", line)
		}
	})

	t.Run("HBox with gap", func(t *testing.T) {
		view := HBox.Gap(2)(
			Text("AA"),
			Text("BB"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		// "AA" then 2 spaces gap then "BB"
		line := extractLine(buf, 0, 10)
		if line != "AA  BB    " {
			t.Errorf("expected 'AA  BB    ', got %q", line)
		}
	})

	t.Run("VBox places children vertically", func(t *testing.T) {
		view := VBox(
			Text("AAA"),
			Text("BBB"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line0 := extractLine(buf, 0, 5)
		line1 := extractLine(buf, 1, 5)
		if line0 != "AAA  " {
			t.Errorf("row 0: expected 'AAA  ', got %q", line0)
		}
		if line1 != "BBB  " {
			t.Errorf("row 1: expected 'BBB  ', got %q", line1)
		}
	})

	t.Run("Nested HBox in VBox", func(t *testing.T) {
		view := VBox(
			HBox(
				Text("A"),
				Text("B"),
			),
			Text("C"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line0 := extractLine(buf, 0, 5)
		line1 := extractLine(buf, 1, 5)
		if line0 != "AB   " {
			t.Errorf("row 0: expected 'AB   ', got %q", line0)
		}
		if line1 != "C    " {
			t.Errorf("row 1: expected 'C    ', got %q", line1)
		}
	})
}

func TestRichTextInsideForEach(t *testing.T) {
	t.Run("RichText with pointer renders per element", func(t *testing.T) {
		type DisplayLine struct {
			LineNum string
			Spans   []Span
		}
		lines := []DisplayLine{
			{LineNum: "1 ", Spans: []Span{{Text: "Hello"}}},
			{LineNum: "2 ", Spans: []Span{{Text: "World"}}},
			{LineNum: "3 ", Spans: []Span{{Text: "Test"}}},
		}

		view := VBox(
			ForEach(&lines, func(dl *DisplayLine) Component {
				return HBox(
					Text(&dl.LineNum),
					RichTextNode{Spans: &dl.Spans},
				)
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Each line should render correctly
		line0 := extractLine(buf, 0, 9)
		line1 := extractLine(buf, 1, 9)
		line2 := extractLine(buf, 2, 9)

		if line0 != "1 Hello  " {
			t.Errorf("row 0: expected '1 Hello  ', got %q", line0)
		}
		if line1 != "2 World  " {
			t.Errorf("row 1: expected '2 World  ', got %q", line1)
		}
		if line2 != "3 Test   " {
			t.Errorf("row 2: expected '3 Test   ', got %q", line2)
		}
	})

	t.Run("RichText updates dynamically", func(t *testing.T) {
		type Line struct {
			Spans []Span
		}
		lines := []Line{
			{Spans: []Span{{Text: "AAA"}}},
			{Spans: []Span{{Text: "BBB"}}},
		}

		view := VBox(
			ForEach(&lines, func(l *Line) Component {
				return RichTextNode{Spans: &l.Spans}
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		line0 := extractLine(buf, 0, 5)
		line1 := extractLine(buf, 1, 5)
		if line0 != "AAA  " {
			t.Errorf("before: row 0 expected 'AAA  ', got %q", line0)
		}
		if line1 != "BBB  " {
			t.Errorf("before: row 1 expected 'BBB  ', got %q", line1)
		}

		// Update spans
		lines[0].Spans = []Span{{Text: "XXX"}}
		lines[1].Spans = []Span{{Text: "YYY"}}
		buf.Clear()
		tmpl.Execute(buf, 20, 10)

		line0 = extractLine(buf, 0, 5)
		line1 = extractLine(buf, 1, 5)
		if line0 != "XXX  " {
			t.Errorf("after: row 0 expected 'XXX  ', got %q", line0)
		}
		if line1 != "YYY  " {
			t.Errorf("after: row 1 expected 'YYY  ', got %q", line1)
		}
	})

	t.Run("RichText with styled spans", func(t *testing.T) {
		type DisplayLine struct {
			Spans []Span
		}
		// Using styled spans like visual mode in vim
		lines := []DisplayLine{
			{Spans: []Span{
				{Text: "normal", Style: Style{}},
				{Text: "selected", Style: Style{Attr: AttrInverse}},
			}},
		}

		view := VBox(
			ForEach(&lines, func(dl *DisplayLine) Component {
				return RichTextNode{Spans: &dl.Spans}
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Both spans should render
		line0 := extractLine(buf, 0, 16)
		if line0 != "normalselected  " {
			t.Errorf("row 0: expected 'normalselected  ', got %q", line0)
		}
	})
}

func TestDynamicStylePointersInsideForEach(t *testing.T) {
	t.Run("text foreground pointer resolves per element", func(t *testing.T) {
		type Row struct {
			Label string
			Color Color
		}
		rows := []Row{
			{Label: "one", Color: Red},
			{Label: "two", Color: Green},
		}

		tmpl := Build(VBox(
			ForEach(&rows, func(row *Row) Component {
				return Text(&row.Label).FG(&row.Color)
			}),
		))
		buf := NewBuffer(10, 3)
		tmpl.Execute(buf, 10, 3)

		if got := buf.Get(0, 0).Style.FG; got != Red {
			t.Fatalf("row 0 FG = %#v, want red", got)
		}
		if got := buf.Get(0, 1).Style.FG; got != Green {
			t.Fatalf("row 1 FG = %#v, want green", got)
		}

		rows[0].Color = Blue
		rows[1].Color = Yellow
		buf.Clear()
		tmpl.Execute(buf, 10, 3)

		if got := buf.Get(0, 0).Style.FG; got != Blue {
			t.Fatalf("updated row 0 FG = %#v, want blue", got)
		}
		if got := buf.Get(0, 1).Style.FG; got != Yellow {
			t.Fatalf("updated row 1 FG = %#v, want yellow", got)
		}
	})

	t.Run("text style pointer resolves per element", func(t *testing.T) {
		type Row struct {
			Label string
			Style Style
		}
		rows := []Row{
			{Label: "one", Style: Style{FG: Red, Attr: AttrBold}},
			{Label: "two", Style: Style{FG: Green, Attr: AttrDim}},
		}

		tmpl := Build(VBox(
			ForEach(&rows, func(row *Row) Component {
				return Text(&row.Label).Style(&row.Style)
			}),
		))
		buf := NewBuffer(10, 3)
		tmpl.Execute(buf, 10, 3)

		if got := buf.Get(0, 0).Style; got.FG != Red || got.Attr&AttrBold == 0 {
			t.Fatalf("row 0 style = %#v, want red bold", got)
		}
		if got := buf.Get(0, 1).Style; got.FG != Green || got.Attr&AttrDim == 0 {
			t.Fatalf("row 1 style = %#v, want green dim", got)
		}
	})
}

func TestRichTextWrapsAcrossLines(t *testing.T) {
	t.Run("word wraps static spans", func(t *testing.T) {
		view := VBox.Width(12)(
			RichTextNode{Spans: []Span{
				{Text: "hello", Style: Style{Attr: AttrBold}},
				{Text: " world again", Style: Style{Attr: AttrDim}},
			}},
			Text("after"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		if line := extractLine(buf, 0, 12); line != "hello world " {
			t.Errorf("row 0 expected wrapped prefix, got %q", line)
		}
		if line := extractLine(buf, 1, 12); line != "again       " {
			t.Errorf("row 1 expected wrapped suffix, got %q", line)
		}
		if line := extractLine(buf, 2, 12); line != "after       " {
			t.Errorf("row 2 expected following content after rich text height, got %q", line)
		}
	})

	t.Run("preserves span styles after wrapping", func(t *testing.T) {
		view := VBox.Width(10)(
			RichTextNode{Spans: []Span{
				{Text: "plain ", Style: Style{}},
				{Text: "selected", Style: Style{Attr: AttrInverse}},
			}},
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		if line := extractLine(buf, 0, 10); line != "plain     " {
			t.Errorf("row 0 expected first wrapped line, got %q", line)
		}
		if line := extractLine(buf, 1, 10); line != "selected  " {
			t.Errorf("row 1 expected second wrapped line, got %q", line)
		}
		if !buf.cells[1*buf.width+0].Style.Attr.Has(AttrInverse) {
			t.Errorf("expected wrapped styled span to keep inverse style")
		}
	})

	t.Run("dynamic spans inside ForEach wrap to natural height", func(t *testing.T) {
		type Line struct {
			Spans []Span
		}
		lines := []Line{
			{Spans: []Span{{Text: "one two three"}}},
			{Spans: []Span{{Text: "four"}}},
		}

		view := VBox.Width(8)(
			ForEach(&lines, func(line *Line) Component {
				return RichTextNode{Spans: &line.Spans}
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		if line := extractLine(buf, 0, 8); line != "one two " {
			t.Errorf("row 0 expected first wrapped line, got %q", line)
		}
		if line := extractLine(buf, 1, 8); line != "three   " {
			t.Errorf("row 1 expected second wrapped line, got %q", line)
		}
		if line := extractLine(buf, 2, 8); line != "four    " {
			t.Errorf("row 2 expected next ForEach item after wrapped height, got %q", line)
		}
	})
}

func TestTextf(t *testing.T) {
	t.Run("static strings compose into single line", func(t *testing.T) {
		view := VBox(
			Textf("hello ", "world"),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 0, 11)
		if line != "hello world" {
			t.Errorf("expected 'hello world', got %q", line)
		}
	})

	t.Run("styled spans via helpers", func(t *testing.T) {
		view := VBox(
			Textf("normal ", Bold("bold")),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 0, 11)
		if line != "normal bold" {
			t.Errorf("expected 'normal bold', got %q", line)
		}

		cell := buf.Get(7, 0)
		if cell.Style.Attr&AttrBold == 0 {
			t.Errorf("expected bold attr on 'b' at col 7, got %v", cell.Style.Attr)
		}
	})

	t.Run("dynamic *string updates on re-render", func(t *testing.T) {
		name := "Alice"
		view := VBox(
			Textf("hi ", &name),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 0, 8)
		if line != "hi Alice" {
			t.Errorf("first render: expected 'hi Alice', got %q", line)
		}

		name = "Bob"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		line = extractLine(buf, 0, 6)
		if line != "hi Bob" {
			t.Errorf("after update: expected 'hi Bob', got %q", line)
		}
	})

	t.Run("dynamic *string inside ForEach", func(t *testing.T) {
		type Item struct {
			Label  string
			Status string
		}
		items := []Item{
			{Label: "build", Status: "ok"},
			{Label: "test", Status: "fail"},
		}

		view := VBox(
			ForEach(&items, func(it *Item) Component {
				return Textf(&it.Label, " -> ", &it.Status)
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line0 := extractLine(buf, 0, 13)
		line1 := extractLine(buf, 1, 14)
		if line0 != "build -> ok  " {
			t.Errorf("row 0: expected 'build -> ok  ', got %q", line0)
		}
		if line1 != "test -> fail  " {
			t.Errorf("row 1: expected 'test -> fail  ', got %q", line1)
		}

		items[0].Status = "done"
		buf.Clear()
		tmpl.Execute(buf, 20, 5)

		line0 = extractLine(buf, 0, 15)
		if line0 != "build -> done  " {
			t.Errorf("after update row 0: expected 'build -> done  ', got %q", line0)
		}
	})

	t.Run("styled TextC inside ForEach", func(t *testing.T) {
		type Row struct {
			Name string
		}
		rows := []Row{{Name: "pete"}}

		view := VBox(
			ForEach(&rows, func(r *Row) Component {
				return Textf("user: ", Text(&r.Name).Bold())
			}),
		)

		tmpl := Build(view)
		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		line := extractLine(buf, 0, 10)
		if line != "user: pete" {
			t.Errorf("expected 'user: pete', got %q", line)
		}

		cell := buf.Get(6, 0)
		if cell.Style.Attr&AttrBold == 0 {
			t.Errorf("expected bold on 'p' at col 6, got %v", cell.Style.Attr)
		}
	})
}

// proposal #27: a NodeRef whose node is gated out by If(false) must report zero
// bounds, not last frame's stale rect — otherwise effect dodge / hit-testing see a
// phantom node where closed content used to be (the #379 FocusShade bright-hole).
func TestNodeRefZeroesWhenGatedOut(t *testing.T) {
	show := true
	var ref NodeRef
	tmpl := Build(VBox(
		If(&show).Then(
			VBox.Border(BorderRounded).NodeRef(&ref)(Text("overlay")),
		),
	))
	buf := NewBuffer(30, 5)
	tmpl.Execute(buf, 30, 5)
	if ref.W == 0 || ref.H == 0 {
		t.Fatalf("ref should be live while shown: W%d H%d", ref.W, ref.H)
	}

	show = false
	buf.Clear()
	tmpl.Execute(buf, 30, 5)
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("gated-out ref must zero (got W%d H%d Op%.2f) — stale rect is a phantom node", ref.W, ref.H, NodeOpacity(&ref))
	}
}

// a ForEach that empties must zero the ref on its (shared) item container — nothing
// renders, so nothing should report bounds.
func TestNodeRefZeroesWhenForEachEmpties(t *testing.T) {
	type Row struct{ Label string }
	rows := []Row{{Label: "a"}, {Label: "b"}}
	var ref NodeRef
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return VBox.NodeRef(&ref)(Text(&r.Label))
		}),
	))
	buf := NewBuffer(20, 6)
	tmpl.Execute(buf, 20, 6)
	if ref.H == 0 {
		t.Fatalf("ref should be live with items present: H%d", ref.H)
	}

	rows = rows[:0]
	buf.Clear()
	tmpl.Execute(buf, 20, 6)
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("ref must zero when the ForEach renders nothing: W%d H%d", ref.W, ref.H)
	}
}

// guards the per-frame cost of zeroing refs: a template with many refs should pay a
// negligible, allocation-free sweep. A regression (e.g. re-collecting every frame)
// shows up here.
func BenchmarkNodeRefZeroingPerFrame(b *testing.B) {
	const n = 200
	flags := make([]bool, n)
	refs := make([]NodeRef, n)
	for i := range flags {
		flags[i] = true
	}
	kids := make([]Component, n)
	for i := range kids {
		kids[i] = If(&flags[i]).Then(VBox.NodeRef(&refs[i])(Text("x")))
	}
	tmpl := Build(VBox(kids...))
	buf := NewBuffer(40, n+2)
	tmpl.Execute(buf, 40, int16(n+2))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 40, int16(n+2))
	}
}

// a real-world composition (regression for the path the first overlay-ref tests missed):
// an If-gated OVERLAY renders in phase 4, not the phase-3 walk. Its NodeRef must
// still zero when the overlay is gated out, or a screen-effect dodge keeps a phantom.
func TestOverlayRefZeroesWhenGatedOut(t *testing.T) {
	open := true
	var ref NodeRef
	tmpl := Build(VBox(
		Text("background"),
		If(&open).Then(
			Overlay.Centered()(
				VBox.Border(BorderRounded).NodeRef(&ref)(Text("help")),
			),
		),
	))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)
	if ref.W == 0 || ref.H == 0 {
		t.Fatalf("overlay ref should populate while open: W%d H%d", ref.W, ref.H)
	}
	open = false
	buf.Clear()
	tmpl.Execute(buf, 40, 10)
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("gated-out OVERLAY ref must zero (got W%d H%d) — phantom dodge region", ref.W, ref.H)
	}
}

// A string Eq bound to an element FIELD is offset-rebound per row, so each row
// compares its own value. Pins the behaviour doubted in a consumer report: the
// failure there was a pointer that wasn't in the element (see the sibling test),
// not a hole in Eq. Covers []T, []*T and a nested chain, since a chain compiles
// its Else branch as a sub-template and the offset has to survive that too.
func TestEqOnElementFieldRebindsPerRow(t *testing.T) {
	type Row struct{ Status string }

	render := func(view Component) string {
		tmpl := Build(view)
		buf := NewBuffer(16, 8)
		tmpl.Execute(buf, 16, 8)
		var out string
		for y := 0; y < 8; y++ {
			if l := strings.TrimRight(buf.GetLine(y), " "); l != "" {
				out += l + "\n"
			}
		}
		return out
	}

	vals := []Row{{"active"}, {"idle"}}
	if got := render(VBox(ForEach(&vals, func(r *Row) Component {
		return If(&r.Status).Eq("active").Then(Text("A")).Else(Text("I"))
	}))); got != "A\nI\n" {
		t.Errorf("[]T: each row must compare its own field; got %q", got)
	}

	p0, p1 := &Row{"active"}, &Row{"idle"}
	ptrs := []*Row{p0, p1}
	if got := render(VBox(ForEach(&ptrs, func(r **Row) Component {
		return If(&(*r).Status).Eq("active").Then(Text("A")).Else(Text("I"))
	}))); got != "A\nI\n" {
		t.Errorf("[]*T: each row must compare its own field; got %q", got)
	}

	chain := []Row{{"active"}, {"idle"}, {"busy"}}
	if got := render(VBox(ForEach(&chain, func(r *Row) Component {
		return If(&r.Status).Eq("active").Then(Text("A")).
			Else(If(&r.Status).Eq("idle").Then(Text("I")).
				Else(Text("B")))
	}))); got != "A\nI\nB\n" {
		t.Errorf("nested chain: offset must survive Else sub-templates; got %q", got)
	}
}

// The counterpart: a pointer that is NOT inside the element fails the compile-time
// range check, so no offset is recorded and every row evaluates the same frozen
// value. This is the actual shape behind "every branch dark" reports — binding a
// local computed in the row function rather than a field of the element.
func TestEqOnComputedLocalFreezesAcrossRows(t *testing.T) {
	type Row struct{ Status string }
	rows := []Row{{"active"}, {"idle"}}

	tmpl := Build(VBox(ForEach(&rows, func(r *Row) Component {
		s := r.Status // not a field of the element — no offset can be taken
		return If(&s).Eq("active").Then(Text("A")).Else(Text("I"))
	})))
	buf := NewBuffer(16, 4)
	tmpl.Execute(buf, 16, 4)

	var out string
	for y := 0; y < 4; y++ {
		if l := strings.TrimRight(buf.GetLine(y), " "); l != "" {
			out += l + "\n"
		}
	}
	if out == "A\nI\n" {
		t.Fatalf("a computed local unexpectedly rebound per row — if this now works, "+
			"the consumer guidance in the sibling test is stale; got %q", out)
	}
}

// A string Eq chain nested inside an If.Then branch inside ForEach still rebinds
// per row — the offset has to survive the outer If's sub-template AND an
// intervening HBox, with a separate chain instance built for each branch.
// Deeper composition than the sibling test, from a consumer report at that depth.
func TestEqChainInsideIfThenRebindsPerRow(t *testing.T) {
	type Row struct {
		Name    string
		Profile string
		Live    bool
	}
	rows := []Row{
		{Name: "r0", Profile: "active", Live: true},
		{Name: "r1", Profile: "idle", Live: false},
		{Name: "r2", Profile: "busy", Live: true},
	}

	badge := func(r *Row) Component {
		return If(&r.Profile).Eq("active").Then(Text("[A]")).
			Else(If(&r.Profile).Eq("idle").Then(Text("[I]")).
				Else(If(&r.Profile).Eq("busy").Then(Text("[B]")).
					Else(Text("[?]"))))
	}

	tmpl := Build(VBox(ForEach(&rows, func(r *Row) Component {
		return If(&r.Live).
			Then(HBox(Text(&r.Name), Text(" live "), badge(r))).
			Else(HBox(Text(&r.Name), Text(" off  "), badge(r)))
	})))
	buf := NewBuffer(40, 8)
	tmpl.Execute(buf, 40, 8)

	var out string
	for y := 0; y < 8; y++ {
		if l := strings.TrimRight(buf.GetLine(y), " "); l != "" {
			out += l + "\n"
		}
	}
	t.Logf("Eq inside If.Then inside ForEach:\n%s", out)

	for _, want := range []string{"r0 live [A]", "r1 off  [I]", "r2 live [B]"} {
		if !strings.Contains(out, want) {
			t.Errorf("REPRODUCED: missing %q\ngot:\n%s", want, out)
		}
	}
}
