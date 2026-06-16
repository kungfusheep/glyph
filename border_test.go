package glyph

import (
	"strings"
	"testing"
)

// TestIfWithGrowBorder tests that a bordered container inside an If with Grow
// has its bottom border drawn at the correct position.
func TestIfWithGrowBorder(t *testing.T) {
	showProcs := true

	// Mimic the dashboard structure:
	// VBoxNode{Grow(1)} containing:
	//   - VBoxNode{Border, Grow(1)} "Timing"
	//   - If(showProcs).Then(VBoxNode{Border, Grow(2)} "Processes")
	view := VBox.Grow(1)(
		VBox.Border(BorderSingle).BorderFG(Yellow).Title("Timing").Grow(1)(
			Text("Line 1"),
			Text("Line 2"),
			Text("Line 3"),
		),

		If(&showProcs).Eq(true).Then(VBox.Border(BorderSingle).BorderFG(BrightBlue).Title("Processes").Grow(2)(
			Text("Process 1"),
			Text("Process 2"),
			Text("Process 3"),
		)),
	)

	// Build template
	tmpl := Build(view)

	// Create a buffer with specific dimensions
	buf := NewBuffer(80, 30)

	// Execute template - this does width distribution, layout, and render
	tmpl.Execute(buf, 80, 30)

	// Print the buffer to see the output
	t.Log("Buffer contents:")
	for y := 0; y < 30; y++ {
		line := buf.GetLine(y)
		if line != "" {
			t.Logf("Line %2d: %s", y, line)
		}
	}

	// Check that Timing box bottom border exists
	timingBottomFound := false
	processesBottomFound := false

	for y := 0; y < 30; y++ {
		for x := 0; x < 80; x++ {
			cell := buf.Get(x, y)
			// Check for bottom-left corner of a box
			if cell.Rune == BorderSingle.BottomLeft {
				// Look at the next few chars to see if it's a border
				if x+1 < 80 {
					nextCell := buf.Get(x+1, y)
					if nextCell.Rune == BorderSingle.Horizontal {
						// This is a bottom border
						// Check color to distinguish Timing (Yellow) vs Processes (BrightBlue)
						if cell.Style.FG == Yellow {
							timingBottomFound = true
							t.Logf("Found Timing bottom border at y=%d", y)
						} else if cell.Style.FG == BrightBlue {
							processesBottomFound = true
							t.Logf("Found Processes bottom border at y=%d", y)
						}
					}
				}
			}
		}
	}

	if !timingBottomFound {
		t.Error("Timing box bottom border not found")
	}
	if !processesBottomFound {
		t.Error("Processes box bottom border not found")
	}
}

// TestDashboardLayoutBorders tests the full dashboard-like structure with
// nested grows and conditionals.
func TestDashboardLayoutBorders(t *testing.T) {
	showProcs := true
	showGraph := true

	// Full dashboard structure (simplified):
	// VBoxNode{Children: [
	//   Text "Header"
	//   Text "Progress bars"
	//   HBoxNode{Children: [Left.Grow(1), Right.Grow(2)]}  // Horizontal flex
	//   VBoxNode{Children: [
	//     Col "Timing".Grow(1)
	//     If.Then(Col "Processes".Grow(2))
	//   ]}.Grow(1)  // The OUTER Col also has Grow!
	//   Text "Footer"
	// ]}
	view := VBox(
		// Fixed header
		Text("Dashboard Header"),
		Text("CPU: [████████████________] 60%"),

		// Main content row (horizontal flex)
		HBox.Gap(1)(
			VBox.Border(BorderSingle).BorderFG(Cyan).Title("Stats").Grow(1)(
				Text("Tasks: 100"),
				Text("Memory: 4GB"),
			),
			VBox.Border(BorderRounded).BorderFG(Green).Title("Load").Grow(2)(
				If(&showGraph).Eq(true).Then(
					Text("Graph: ▁▂▃▄▅▆▇█"),
				),
			),
		),

		// Middle section with vertical flex - THIS IS THE KEY PART
		// The outer Col has Grow(1), inner children have Grow(1) and Grow(2)
		VBox.Grow(1)( // <-- OUTER COL HAS GROW!
			VBox.Border(BorderDouble).BorderFG(Yellow).Title("Timing").Grow(1)(
				Text("Render: 100µs"),
				Text("Flush: 50µs"),
			),

			If(&showProcs).Eq(true).Then(VBox.Border(BorderSingle).BorderFG(BrightBlue).Title("Processes").Grow(2)(
				Text("PID    NAME     CPU"),
				Text("1001   nginx    2.5%"),
				Text("1002   node     5.2%"),
			)),
		),

		// Fixed footer
		Text("Press q to quit"),
	)

	tmpl := Build(view)
	buf := NewBuffer(80, 40)
	tmpl.Execute(buf, 80, 40)

	t.Log("Buffer contents:")
	for y := 0; y < 40; y++ {
		line := buf.GetLine(y)
		if line != "" {
			t.Logf("Line %2d: %s", y, line)
		}
	}

	// Check for Processes bottom border
	processesBottomFound := false
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			cell := buf.Get(x, y)
			if cell.Rune == BorderSingle.BottomLeft && cell.Style.FG == BrightBlue {
				if x+1 < 80 {
					nextCell := buf.Get(x+1, y)
					if nextCell.Rune == BorderSingle.Horizontal {
						processesBottomFound = true
						t.Logf("Found Processes bottom border at y=%d", y)
					}
				}
			}
		}
	}

	if !processesBottomFound {
		t.Error("Processes box bottom border not found - this is the bug we're debugging!")
	}
}

// TestHBoxWithBorderedChildren tests that borders inside HBox flex children
// are drawn correctly.
func TestHBoxWithBorderedChildren(t *testing.T) {
	view := HBox.Gap(1)(
		// Left panel
		VBox.Grow(1)(
			VBox.Border(BorderSingle).BorderFG(Cyan).Title("Stats")(
				Text("Tasks: 142"),
				Text("Sleeping: 138"),
			),
			VBox.Border(BorderRounded).BorderFG(Green).Title("Load")(
				Text("1.17, 0.69, 0.85"),
			),
		),

		// Right panel
		VBox.Grow(2)(
			VBox.Border(BorderSingle).BorderFG(Magenta).Title("Info")(
				Text("Line 1"),
				Text("Line 2"),
				Text("Line 3"),
			),
		),
	)

	tmpl := Build(view)
	buf := NewBuffer(60, 15)
	tmpl.Execute(buf, 60, 15)

	t.Log("Op geometries:")
	for i, op := range tmpl.ops {
		g := tmpl.geom[i]
		name := ""
		if op.Title != "" {
			name = op.Title
		} else if op.Kind == OpContainer {
			if op.IsRow {
				name = "Row"
			} else {
				name = "Col"
			}
		}
		t.Logf("  [%d] %s: LocalX=%d LocalY=%d W=%d H=%d", i, name, g.LocalX, g.LocalY, g.W, g.H)
	}

	t.Log("Buffer contents:")
	for y := 0; y < 15; y++ {
		line := buf.GetLine(y)
		t.Logf("Line %2d: %s", y, line)
	}

	statsBottomFound := false
	loadBottomFound := false

	for y := 0; y < 15; y++ {
		for x := 0; x < 60; x++ {
			cell := buf.Get(x, y)
			if cell.Rune == BorderSingle.BottomLeft && cell.Style.FG == Cyan {
				statsBottomFound = true
				t.Logf("Found Stats bottom border at y=%d", y)
			}
			if cell.Rune == BorderRounded.BottomLeft && cell.Style.FG == Green {
				loadBottomFound = true
				t.Logf("Found Load bottom border at y=%d", y)
			}
		}
	}

	if !statsBottomFound {
		t.Error("Stats box bottom border not found!")
	}
	if !loadBottomFound {
		t.Error("Load box bottom border not found!")
	}
}

func TestForEachPerItemBorderFG(t *testing.T) {
	type msg struct {
		Who   string
		Color Color
	}
	msgs := []msg{{"A", Red}, {"B", Blue}}
	tmpl := Build(VBox(ForEach(&msgs, func(m *msg) Component {
		return VBox.Border(BorderRounded).BorderFG(&m.Color)(Text(&m.Who))
	})))
	buf := NewBuffer(20, 8)
	tmpl.Execute(buf, 20, 8)

	out := buf.String()
	if !strings.ContainsAny(out, "╭╮╰╯│─") {
		t.Fatalf("border dropped:\n%s", out)
	}
	// item A border (row 0) must be Red; item B border (row 3) must be Blue —
	// proving the per-item pointer rebinds, not a single frozen colour.
	topA := buf.Get(0, 0)
	topB := buf.Get(0, 3)
	if topA.Rune != '╭' || topA.Style.FG != Red {
		t.Errorf("item A border = %q FG %+v, want ╭ Red", topA.Rune, topA.Style.FG)
	}
	if topB.Rune != '╭' || topB.Style.FG != Blue {
		t.Errorf("item B border = %q FG %+v, want ╭ Blue", topB.Rune, topB.Style.FG)
	}
}

func TestStableBorderFGPointerStillRenders(t *testing.T) {
	bc := Green
	title := "x"
	tmpl := Build(VBox.Border(BorderRounded).BorderFG(&bc)(Text(&title)))
	buf := NewBuffer(12, 4)
	tmpl.Execute(buf, 12, 4)
	if c := buf.Get(0, 0); c.Rune != '╭' || c.Style.FG != Green {
		t.Errorf("stable BorderFG pointer: got %q FG %+v, want ╭ Green", c.Rune, c.Style.FG)
	}
}
