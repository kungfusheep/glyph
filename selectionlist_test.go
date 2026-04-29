package glyph

import (
	"fmt"
	"testing"
)

func TestV2SelectionListBasic(t *testing.T) {
	items := []string{"Apple", "Banana", "Cherry"}
	selected := 1

	view := VBox(
		&SelectionList{
			Items:    &items,
			Selected: &selected,
			Marker:   "> ",
			Render: func(s *string) Component {
				return Text(s)
			},
		},
	)

	tmpl := Build(view)
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Check output
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)
	line2 := buf.GetLine(2)

	t.Logf("Line 0: %q", line0)
	t.Logf("Line 1: %q", line1)
	t.Logf("Line 2: %q", line2)

	// Apple should not have marker (selected = 1)
	if !contains(line0, "  Apple") {
		t.Errorf("Line 0 should have spaces before Apple: %q", line0)
	}

	// Banana should have marker (selected = 1)
	if !contains(line1, "> Banana") {
		t.Errorf("Line 1 should have marker before Banana: %q", line1)
	}

	// Cherry should not have marker
	if !contains(line2, "  Cherry") {
		t.Errorf("Line 2 should have spaces before Cherry: %q", line2)
	}
}

func TestV2SelectionListWithRender(t *testing.T) {
	items := []string{"First", "Second", "Third"}
	selected := 0

	view := VBox(
		&SelectionList{
			Items:    &items,
			Selected: &selected,
			Marker:   "* ",
			Render: func(s *string) Component {
				return Text(s)
			},
		},
	)

	tmpl := Build(view)
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	line0 := buf.GetLine(0)
	t.Logf("Line 0: %q", line0)

	// First should have marker
	if !contains(line0, "* First") {
		t.Errorf("Line 0 should have marker before First: %q", line0)
	}
}

func TestV2SelectionListMaxVisible(t *testing.T) {
	items := []string{"One", "Two", "Three", "Four", "Five"}
	selected := 0

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 3,
		Render: func(s *string) Component {
			return Text(s)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Should only show 3 items
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)
	line2 := buf.GetLine(2)
	line3 := buf.GetLine(3)

	t.Logf("Line 0: %q", line0)
	t.Logf("Line 1: %q", line1)
	t.Logf("Line 2: %q", line2)
	t.Logf("Line 3: %q", line3)

	if !contains(line0, "One") {
		t.Errorf("Line 0 should contain One: %q", line0)
	}
	if !contains(line2, "Three") {
		t.Errorf("Line 2 should contain Three: %q", line2)
	}
	// Line 3 should be empty (only showing 3 items)
	if contains(line3, "Four") {
		t.Errorf("Line 3 should NOT contain Four (MaxVisible=3): %q", line3)
	}
}

func TestV2SelectionListScrolling(t *testing.T) {
	items := []string{"One", "Two", "Three", "Four", "Five"}
	selected := 3 // Select "Four"

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 3,
		Render: func(s *string) Component {
			return Text(s)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// After ensureVisible, offset should be 1 so we see [Two, Three, Four]
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)
	line2 := buf.GetLine(2)

	t.Logf("Line 0: %q", line0)
	t.Logf("Line 1: %q", line1)
	t.Logf("Line 2: %q", line2)

	// "Four" should be visible and selected
	found := false
	for y := 0; y < 3; y++ {
		line := buf.GetLine(y)
		if contains(line, "> Four") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Selected item 'Four' with marker not found in visible window")
	}
}

func TestSelectionListOverflowClipped(t *testing.T) {
	// Reproduces the pprofin bug: a bordered VBox with header, Grow(1) list,
	// and footer. The list has MaxVisible(20) but the terminal is only 15 lines
	// tall. Without the fix, the list renders past the border and pushes
	// the footer off-screen.
	items := make([]string, 25)
	for i := range items {
		items[i] = fmt.Sprintf("Item %02d", i+1)
	}
	selected := 0

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 20,
		Render: func(s *string) Component {
			return Text(s)
		},
	}

	// Layout mirrors pprofin: bordered root, header, grow list, footer
	view := VBox.Border(BorderSingle).Title("test")(
		Text("header"),     // 1 line
		HRule(),            // 1 line
		VBox.Grow(1)(list), // flex fill
		HRule(),            // 1 line
		Text("footer"),     // 1 line
	)

	screenH := int16(15)
	tmpl := Build(view)
	buf := NewBuffer(40, int(screenH))
	tmpl.Execute(buf, 40, screenH)

	// Dump for debugging
	for y := 0; y < int(screenH); y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// The bottom border must be on the last line
	bottomLeft := buf.Get(0, int(screenH)-1)
	if bottomLeft.Rune != '└' {
		t.Errorf("Bottom-left corner should be └, got %c (border overwritten by list overflow)", bottomLeft.Rune)
	}

	// The footer text must be on the second-to-last line (inside border)
	footerLine := buf.GetLine(int(screenH) - 2)
	if !contains(footerLine, "footer") {
		t.Errorf("Footer should be visible on line %d, got: %q", screenH-2, footerLine)
	}

	// The header must be on line 1 (inside top border)
	headerLine := buf.GetLine(1)
	if !contains(headerLine, "header") {
		t.Errorf("Header should be on line 1, got: %q", headerLine)
	}

	// List items should NOT appear on the footer or border lines
	footerArea := buf.GetLine(int(screenH) - 2)
	if contains(footerArea, "Item") {
		t.Errorf("List items should not overflow into footer area: %q", footerArea)
	}
	borderLine := buf.GetLine(int(screenH) - 1)
	if contains(borderLine, "Item") {
		t.Errorf("List items should not overflow into border: %q", borderLine)
	}
}

func TestSelectionListVariableHeight(t *testing.T) {
	type item struct {
		Title string
		Desc  string
	}

	items := []item{
		{"Open", "Opens a file from disk"},
		{"Save", "Saves the current buffer"},
		{"Quit", "Exits the application"},
	}
	selected := 1

	list := &SelectionList{
		Items:    &items,
		Selected: &selected,
		Marker:   "> ",
		Render: func(it *item) Component {
			// 2-row item: title on first line, description on second
			return VBox(
				Text(&it.Title),
				Text(&it.Desc),
			)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	for y := 0; y < 10; y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// Item 0 ("Open") occupies lines 0-1
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)
	if !contains(line0, "Open") {
		t.Errorf("Line 0 should contain 'Open': %q", line0)
	}
	if !contains(line1, "Opens a file") {
		t.Errorf("Line 1 should contain description: %q", line1)
	}
	// non-selected: no marker
	if contains(line0, ">") {
		t.Errorf("Line 0 should NOT have marker (item 0 not selected): %q", line0)
	}

	// Item 1 ("Save") is selected, occupies lines 2-3
	line2 := buf.GetLine(2)
	line3 := buf.GetLine(3)
	if !contains(line2, "> Save") {
		t.Errorf("Line 2 should have marker + 'Save': %q", line2)
	}
	if !contains(line3, "Saves") {
		t.Errorf("Line 3 should contain description: %q", line3)
	}

	// Item 2 ("Quit") occupies lines 4-5
	line4 := buf.GetLine(4)
	line5 := buf.GetLine(5)
	if !contains(line4, "Quit") {
		t.Errorf("Line 4 should contain 'Quit': %q", line4)
	}
	if !contains(line5, "Exits") {
		t.Errorf("Line 5 should contain description: %q", line5)
	}

	// Line 6 should be empty (no more items)
	line6 := buf.GetLine(6)
	if contains(line6, "Open") || contains(line6, "Save") || contains(line6, "Quit") {
		t.Errorf("Line 6 should be empty, got: %q", line6)
	}
}

func TestSelectionListVariableHeightMaxVisible(t *testing.T) {
	type item struct {
		Title string
		Desc  string
	}

	items := []item{
		{"Alpha", "First item"},
		{"Beta", "Second item"},
		{"Gamma", "Third item"},
		{"Delta", "Fourth item"},
		{"Epsilon", "Fifth item"},
	}
	selected := 0

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 3,
		Render: func(it *item) Component {
			return VBox(
				Text(&it.Title),
				Text(&it.Desc),
			)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	for y := 0; y < 12; y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// 3 items visible x 2 rows each = 6 rows total
	line0 := buf.GetLine(0)
	line5 := buf.GetLine(5)
	line6 := buf.GetLine(6)

	if !contains(line0, "> Alpha") {
		t.Errorf("Line 0 should have marker + 'Alpha': %q", line0)
	}
	if !contains(line5, "Third item") {
		t.Errorf("Line 5 should contain 'Third item' (last visible item desc): %q", line5)
	}
	// Item 4 ("Delta") should NOT be visible
	if contains(line6, "Delta") {
		t.Errorf("Line 6 should NOT contain 'Delta' (MaxVisible=3): %q", line6)
	}
}

func TestSelectionListVariableHeightScrolling(t *testing.T) {
	type item struct {
		Title string
		Desc  string
	}

	items := []item{
		{"Alpha", "First"},
		{"Beta", "Second"},
		{"Gamma", "Third"},
		{"Delta", "Fourth"},
		{"Epsilon", "Fifth"},
	}
	selected := 4 // select last item

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 3,
		Render: func(it *item) Component {
			return VBox(
				Text(&it.Title),
				Text(&it.Desc),
			)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	for y := 0; y < 12; y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// With scrolling, the last 3 items should be visible: Gamma, Delta, Epsilon
	// Epsilon (selected) should have marker
	found := false
	for y := 0; y < 12; y++ {
		line := buf.GetLine(y)
		if contains(line, "> Epsilon") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Selected item 'Epsilon' with marker not found in rendered output")
	}

	// Alpha should NOT be visible (scrolled past)
	for y := 0; y < 12; y++ {
		line := buf.GetLine(y)
		if contains(line, "Alpha") {
			t.Errorf("'Alpha' should not be visible (scrolled past), found on line %d: %q", y, line)
			break
		}
	}
}

func TestSelectionListVariableHeightSelectedRef(t *testing.T) {
	type item struct {
		Title string
		Desc  string
	}

	items := []item{
		{"Open", "Opens file"},
		{"Save", "Saves file"},
		{"Quit", "Exits app"},
	}
	selected := 1
	ref := &NodeRef{}

	list := &SelectionList{
		Items:       &items,
		Selected:    &selected,
		Marker:      "> ",
		SelectedRef: ref,
		Render: func(it *item) Component {
			return VBox(
				Text(&it.Title),
				Text(&it.Desc),
			)
		},
	}

	view := VBox(list)

	tmpl := Build(view)
	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	// Item 0 takes rows 0-1, so item 1 starts at Y=2 with height 2
	if ref.Y != 2 {
		t.Errorf("SelectedRef.Y should be 2, got %d", ref.Y)
	}
	if ref.H != 2 {
		t.Errorf("SelectedRef.H should be 2, got %d", ref.H)
	}
}

func TestSelectionListVariableHeightClipped(t *testing.T) {
	// variable-height items inside a bordered box with fixed screen height;
	// items must not overflow beyond the border
	type item struct {
		Title string
		Desc  string
	}

	items := []item{
		{"Alpha", "First"},
		{"Beta", "Second"},
		{"Gamma", "Third"},
		{"Delta", "Fourth"},
		{"Epsilon", "Fifth"},
		{"Zeta", "Sixth"},
		{"Eta", "Seventh"},
		{"Theta", "Eighth"},
	}
	selected := 0

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 20,
		Render: func(it *item) Component {
			return VBox(
				Text(&it.Title),
				Text(&it.Desc),
			)
		},
	}

	view := VBox.Border(BorderSingle).Title("test")(
		Text("header"),
		HRule(),
		VBox.Grow(1)(list),
		HRule(),
		Text("footer"),
	)

	screenH := int16(15)
	tmpl := Build(view)
	buf := NewBuffer(40, int(screenH))
	tmpl.Execute(buf, 40, screenH)

	for y := 0; y < int(screenH); y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// bottom border must be on last line
	bottomLeft := buf.Get(0, int(screenH)-1)
	if bottomLeft.Rune != '└' {
		t.Errorf("Bottom-left corner should be └, got %c", bottomLeft.Rune)
	}

	// footer must be visible
	footerLine := buf.GetLine(int(screenH) - 2)
	if !contains(footerLine, "footer") {
		t.Errorf("Footer should be visible: %q", footerLine)
	}

	// no item content on footer or border lines
	for _, y := range []int{int(screenH) - 2, int(screenH) - 1} {
		line := buf.GetLine(y)
		if contains(line, "Alpha") || contains(line, "Beta") || contains(line, "Gamma") {
			t.Errorf("List content should not overflow into footer/border on line %d: %q", y, line)
		}
	}
}

func TestSelectionListOverflowScrolling(t *testing.T) {
	// When the list is clipped to fewer rows than MaxVisible, scrolling
	// to a selected item near the bottom should still work correctly.
	items := make([]string, 25)
	for i := range items {
		items[i] = fmt.Sprintf("Item %02d", i+1)
	}
	selected := 24 // last item

	list := &SelectionList{
		Items:      &items,
		Selected:   &selected,
		Marker:     "> ",
		MaxVisible: 20,
		Render: func(s *string) Component {
			return Text(s)
		},
	}

	view := VBox.Border(BorderSingle).Title("test")(
		Text("header"),
		HRule(),
		VBox.Grow(1)(list),
		HRule(),
		Text("footer"),
	)

	screenH := int16(15)
	tmpl := Build(view)
	buf := NewBuffer(40, int(screenH))
	tmpl.Execute(buf, 40, screenH)

	for y := 0; y < int(screenH); y++ {
		t.Logf("Line %2d: %q", y, buf.GetLine(y))
	}

	// The selected item (Item 25) should be visible somewhere in the list area
	found := false
	for y := 3; y < int(screenH)-3; y++ { // list area is between header+hrule and hrule+footer
		line := buf.GetLine(y)
		if contains(line, "> Item 25") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Selected item 'Item 25' should be visible in the clipped list area")
	}

	// Footer must still be visible
	footerLine := buf.GetLine(int(screenH) - 2)
	if !contains(footerLine, "footer") {
		t.Errorf("Footer should still be visible: %q", footerLine)
	}
}
