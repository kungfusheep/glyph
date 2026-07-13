package glyph

import (
	"strings"
	"testing"
)

func TestSerialFlexPercentWidth(t *testing.T) {
	// Test that PercentWidth distributes space correctly in a Row
	tmpl := Build(HBox(
		VBox.WidthPct(0.5)(Text("Left")),
		VBox.WidthPct(0.5)(Text("Right")),
	))

	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	output := buf.String()
	t.Logf("PercentWidth output:\n%s", output)

	// Left should be at position 0
	if !strings.Contains(output, "Left") {
		t.Error("Output should contain 'Left'")
	}
	// Right should be at position 20 (50% of 40)
	if !strings.Contains(output, "Right") {
		t.Error("Output should contain 'Right'")
	}
}

func TestSerialFlexGrow(t *testing.T) {
	// Test that FlexGrow distributes remaining space
	tmpl := Build(VBox(
		Text("Header"), // H=1
		VBox.Grow(1)(Text("Content")),
	))

	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	output := buf.String()
	t.Logf("FlexGrow output:\n%s", output)

	// Header should be at Y=0
	if !strings.Contains(output, "Header") {
		t.Error("Output should contain 'Header'")
	}
	// Content should be somewhere below
	if !strings.Contains(output, "Content") {
		t.Error("Output should contain 'Content'")
	}
}

func TestSerialFlexBorder(t *testing.T) {
	// Test that borders are drawn correctly
	tmpl := Build(VBox.Border(BorderSingle).Title("Panel")(Text("Inside")))

	buf := NewBuffer(30, 5)
	tmpl.Execute(buf, 30, 5)

	output := buf.String()
	t.Logf("Border output:\n%s", output)

	// Check border corner
	if buf.Get(0, 0).Rune != '┌' {
		t.Errorf("Top-left should be ┌, got %c", buf.Get(0, 0).Rune)
	}

	// Check content is inside
	if !strings.Contains(output, "Inside") {
		t.Error("Output should contain 'Inside'")
	}
}

func TestSerialFlexExplicitHeight(t *testing.T) {
	// Test explicit height is respected
	tmpl := Build(VBox.Height(5)(
		Text("Line 1"),
		Text("Line 2"),
	))

	buf := NewBuffer(40, 20)
	tmpl.Execute(buf, 40, 20)

	output := buf.String()
	t.Logf("ExplicitHeight output:\n%s", output)

	if !strings.Contains(output, "Line 1") {
		t.Error("Output should contain 'Line 1'")
	}
}

func TestSerialFlexCombined(t *testing.T) {
	// Test combining PercentWidth, FlexGrow, and Border
	tmpl := Build(VBox(
		HBox(
			VBox.WidthPct(0.5).Border(BorderSingle).Title("Left")(Text("L1")),
			VBox.WidthPct(0.5).Border(BorderSingle).Title("Right")(Text("R1")),
		),
		VBox.Grow(1).Border(BorderSingle).Title("Log")(Text("Log entry")),
	))

	buf := NewBuffer(60, 20)
	tmpl.Execute(buf, 60, 20)

	output := buf.StringTrimmed()
	t.Logf("Combined layout:\n%s", output)

	// Check all content is present
	if !strings.Contains(output, "L1") {
		t.Error("Output should contain 'L1'")
	}
	if !strings.Contains(output, "R1") {
		t.Error("Output should contain 'R1'")
	}
	if !strings.Contains(output, "Log entry") {
		t.Error("Output should contain 'Log entry'")
	}
}

func TestSerialFlexWithPointerBindings(t *testing.T) {
	// Test that flex works with dynamic pointer bindings
	status := "OK"
	level := 75

	tmpl := Build(VBox(
		HBox(
			VBox.WidthPct(0.5)(Text(&status)),
			VBox.WidthPct(0.5)(Progress(&level).Width(10)),
		),
	))

	buf := NewBuffer(40, 5)

	// Initial render
	tmpl.Execute(buf, 40, 5)
	output1 := buf.String()
	t.Logf("Initial:\n%s", output1)

	if !strings.Contains(output1, "OK") {
		t.Error("Initial output should contain 'OK'")
	}

	// Update values and re-render
	status = "ERROR"
	level = 25

	buf.Clear()
	tmpl.Execute(buf, 40, 5)
	output2 := buf.String()
	t.Logf("Updated:\n%s", output2)

	if !strings.Contains(output2, "ERROR") {
		t.Error("Updated output should contain 'ERROR'")
	}
}

func TestLeaderComponent(t *testing.T) {
	t.Run("static leader renders correctly", func(t *testing.T) {
		tmpl := Build(VBox(
			Leader("CPU", "75%").Width(20),
			Leader("MEM", "4.2GB").Width(20),
		))

		buf := NewBuffer(40, 5)
		tmpl.Execute(buf, 40, 5)

		output := buf.String()
		t.Logf("Leader output:\n%s", output)

		// Should contain label and value connected by dots
		if !strings.Contains(output, "CPU") {
			t.Error("Output should contain 'CPU'")
		}
		if !strings.Contains(output, "75%") {
			t.Error("Output should contain '75%'")
		}
		if !strings.Contains(output, "...") {
			t.Error("Output should contain dots")
		}
	})

	t.Run("pointer binding updates dynamically", func(t *testing.T) {
		value := "PASS"
		tmpl := Build(VBox(
			Leader("STATUS", &value).Width(25),
		))

		buf := NewBuffer(40, 5)
		tmpl.Execute(buf, 40, 5)

		output1 := buf.String()
		t.Logf("Initial:\n%s", output1)

		if !strings.Contains(output1, "PASS") {
			t.Error("Initial output should contain 'PASS'")
		}

		// Update value and re-render
		value = "FAIL"
		buf.Clear()
		tmpl.Execute(buf, 40, 5)

		output2 := buf.String()
		t.Logf("Updated:\n%s", output2)

		if !strings.Contains(output2, "FAIL") {
			t.Error("Updated output should contain 'FAIL'")
		}
		if strings.Contains(output2, "PASS") {
			t.Error("Updated output should NOT contain 'PASS'")
		}
	})

	t.Run("custom fill character", func(t *testing.T) {
		tmpl := Build(VBox(
			Leader("ITEM", "OK").Width(15).Fill('-'),
		))

		buf := NewBuffer(40, 5)
		tmpl.Execute(buf, 40, 5)

		output := buf.String()
		t.Logf("Custom fill output:\n%s", output)

		if !strings.Contains(output, "-") {
			t.Error("Output should contain dash fill")
		}
	})

	t.Run("leader in bordered panel", func(t *testing.T) {
		tmpl := Build(VBox.Border(BorderSingle).Title("STATUS")(
			Leader("RAM", "PASS").Width(20),
			Leader("CPU", "OK").Width(20),
		))

		buf := NewBuffer(30, 6)
		tmpl.Execute(buf, 30, 6)

		output := buf.String()
		t.Logf("Bordered panel:\n%s", output)

		if !strings.Contains(output, "RAM") {
			t.Error("Output should contain 'RAM'")
		}
		if !strings.Contains(output, "STATUS") {
			t.Error("Output should contain panel title 'STATUS'")
		}
	})
}

func TestSparklineComponent(t *testing.T) {
	t.Run("basic sparkline renders", func(t *testing.T) {
		values := []float64{1, 3, 5, 7, 5, 3, 1, 2, 4, 6, 8}
		tmpl := Build(VBox(
			Text("CPU:"),
			Sparkline(values),
		))

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)

		output := buf.String()
		t.Logf("Sparkline output:\n%s", output)

		// Should contain sparkline characters
		if !strings.Contains(output, "CPU:") {
			t.Error("Output should contain 'CPU:'")
		}
		// Check for block characters (any of ▁▂▃▄▅▆▇█)
		hasBlock := false
		for _, r := range output {
			if r >= '▁' && r <= '█' {
				hasBlock = true
				break
			}
		}
		if !hasBlock {
			t.Error("Output should contain block characters")
		}
	})

	t.Run("sparkline updates dynamically", func(t *testing.T) {
		values := []float64{1, 2, 3, 4, 5}
		tmpl := Build(Sparkline(&values).Width(10))

		buf := NewBuffer(15, 3)
		tmpl.Execute(buf, 15, 3)
		output1 := buf.String()
		t.Logf("Initial:\n%s", output1)

		// Update values - reverse the trend
		values = []float64{5, 4, 3, 2, 1}
		buf.Clear()
		tmpl.Execute(buf, 15, 3)
		output2 := buf.String()
		t.Logf("Updated:\n%s", output2)

		// Just verify it renders without error
		if output1 == "" || output2 == "" {
			t.Error("Sparkline should produce output")
		}
	})

	t.Run("sparkline with fixed min/max", func(t *testing.T) {
		values := []float64{25, 50, 75}
		tmpl := Build(Sparkline(values).Range(0, 100))

		buf := NewBuffer(10, 3)
		tmpl.Execute(buf, 10, 3)
		output := buf.String()
		t.Logf("Fixed range:\n%s", output)

		// Should render without error
		if output == "" {
			t.Error("Sparkline should produce output")
		}
	})

	t.Run("multi-row sparkline height", func(t *testing.T) {
		values := []float64{0, 50, 100}
		tmpl := Build(Sparkline(values).Width(3).Height(3).Range(0, 100))

		buf := NewBuffer(3, 3)
		tmpl.Execute(buf, 3, 3)

		// row 0 (top): col 0 should be space (0%), col 2 should be full block (100%)
		top := buf.Get(0, 0)
		if top.Rune != ' ' {
			t.Errorf("top-left should be space for 0%% value, got %q", string(top.Rune))
		}
		topRight := buf.Get(2, 0)
		if topRight.Rune != '█' {
			t.Errorf("top-right should be full block for 100%% value, got %q", string(topRight.Rune))
		}

		// bottom row: all columns should have content (even 0% gets ▁ minimum from fractional)
		bot := buf.Get(0, 2)
		if bot.Rune != ' ' && (bot.Rune < '▁' || bot.Rune > '█') {
			t.Errorf("bottom-left unexpected rune: %q", string(bot.Rune))
		}
		botRight := buf.Get(2, 2)
		if botRight.Rune != '█' {
			t.Errorf("bottom-right should be full block for 100%%, got %q", string(botRight.Rune))
		}

		t.Logf("Multi-row output:\n%s", buf.String())
	})

	t.Run("multi-row sparkline via builder", func(t *testing.T) {
		values := []float64{0, 50, 100}
		tmpl := Build(Sparkline(values).Width(3).Height(4))

		buf := NewBuffer(3, 4)
		tmpl.Execute(buf, 3, 4)

		// 100% value should fill all 4 rows with full blocks
		for row := 0; row < 4; row++ {
			c := buf.Get(2, row)
			if c.Rune != '█' {
				t.Errorf("row %d col 2 should be █ for 100%%, got %q", row, string(c.Rune))
			}
		}

		// 0% value should be space everywhere
		for row := 0; row < 4; row++ {
			c := buf.Get(0, row)
			if c.Rune != ' ' {
				t.Errorf("row %d col 0 should be space for 0%%, got %q", row, string(c.Rune))
			}
		}

		t.Logf("Builder multi-row output:\n%s", buf.String())
	})
}

func TestHRuleVRuleSpacer(t *testing.T) {
	t.Run("HRule fills width", func(t *testing.T) {
		tmpl := Build(VBox(
			Text("Above"),
			HRule(),
			Text("Below"),
		))

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()
		t.Logf("HRule output:\n%s", output)

		if !strings.Contains(output, "Above") {
			t.Error("Output should contain 'Above'")
		}
		if !strings.Contains(output, "─") {
			t.Error("Output should contain horizontal line character")
		}
		if !strings.Contains(output, "Below") {
			t.Error("Output should contain 'Below'")
		}
	})

	t.Run("HRule custom character", func(t *testing.T) {
		tmpl := Build(VBox(
			HRule().Char('═'),
		))

		buf := NewBuffer(10, 3)
		tmpl.Execute(buf, 10, 3)
		output := buf.String()
		t.Logf("Custom HRule:\n%s", output)

		if !strings.Contains(output, "═") {
			t.Error("Output should contain double line character")
		}
	})

	t.Run("VRule in row with height", func(t *testing.T) {
		tmpl := Build(HBox(
			VBox.WidthPct(0.4)(
				Text("Left1"),
				Text("Left2"),
			),
			VRule(),
			VBox.WidthPct(0.5)(
				Text("Right1"),
				Text("Right2"),
			),
		))

		buf := NewBuffer(20, 3)
		tmpl.Execute(buf, 20, 3)
		output := buf.String()
		t.Logf("VRule output:\n%s", output)

		if !strings.Contains(output, "Left1") {
			t.Error("Output should contain 'Left1'")
		}
		if !strings.Contains(output, "│") {
			t.Error("Output should contain vertical line character")
		}
		if !strings.Contains(output, "Right1") {
			t.Error("Output should contain 'Right1'")
		}
	})

	t.Run("Spacer creates gap", func(t *testing.T) {
		tmpl := Build(VBox(
			Text("Line1"),
			SpaceH(2),
			Text("Line4"),
		))

		buf := NewBuffer(20, 6)
		tmpl.Execute(buf, 20, 6)

		// Line1 should be at y=0
		if got := buf.GetLine(0); got != "Line1" {
			t.Errorf("line 0: got %q, want %q", got, "Line1")
		}
		// Line4 should be at y=3 (after 2-line spacer)
		if got := buf.GetLine(3); got != "Line4" {
			t.Errorf("line 3: got %q, want %q", got, "Line4")
		}
	})
}

func TestSpinnerComponent(t *testing.T) {
	t.Run("Spinner renders current frame", func(t *testing.T) {
		frame := 0
		tmpl := Build(HBox(
			Spinner(&frame),
			Text(" Loading..."),
		))

		buf := NewBuffer(20, 1)
		tmpl.Execute(buf, 20, 1)
		output := buf.String()
		t.Logf("Spinner frame 0:\n%s", output)

		// Default spinner is SpinnerBraille, frame 0 is "⠋"
		if !strings.Contains(output, "⠋") {
			t.Error("Output should contain first braille spinner frame")
		}
		if !strings.Contains(output, "Loading") {
			t.Error("Output should contain 'Loading'")
		}
	})

	t.Run("Spinner advances frames", func(t *testing.T) {
		frame := 0
		tmpl := Build(Spinner(&frame))

		buf := NewBuffer(5, 1)

		// Frame 0
		tmpl.Execute(buf, 5, 1)
		if buf.Get(0, 0).Rune != '⠋' {
			t.Errorf("frame 0: got %c, want ⠋", buf.Get(0, 0).Rune)
		}

		// Frame 1
		frame = 1
		buf.Clear()
		tmpl.Execute(buf, 5, 1)
		if buf.Get(0, 0).Rune != '⠙' {
			t.Errorf("frame 1: got %c, want ⠙", buf.Get(0, 0).Rune)
		}
	})

	t.Run("Spinner with custom frames", func(t *testing.T) {
		frame := 0
		tmpl := Build(Spinner(&frame).Frames(SpinnerLine))

		buf := NewBuffer(5, 1)
		tmpl.Execute(buf, 5, 1)

		// SpinnerLine frame 0 is "-"
		if buf.Get(0, 0).Rune != '-' {
			t.Errorf("got %c, want -", buf.Get(0, 0).Rune)
		}
	})

	t.Run("Spinner with dots frames", func(t *testing.T) {
		frame := 0
		tmpl := Build(Spinner(&frame).Frames(SpinnerDots))

		buf := NewBuffer(5, 1)
		tmpl.Execute(buf, 5, 1)

		// SpinnerDots frame 0 is "⣾"
		if buf.Get(0, 0).Rune != '⣾' {
			t.Errorf("got %c, want ⣾", buf.Get(0, 0).Rune)
		}
	})

	t.Run("Spinner wraps frame index", func(t *testing.T) {
		frame := 10 // SpinnerBraille has 10 frames, so this should wrap to 0
		tmpl := Build(Spinner(&frame))

		buf := NewBuffer(5, 1)
		tmpl.Execute(buf, 5, 1)

		// Should wrap to frame 0
		if buf.Get(0, 0).Rune != '⠋' {
			t.Errorf("wrapped frame: got %c, want ⠋", buf.Get(0, 0).Rune)
		}
	})
}

func TestScrollbarComponent(t *testing.T) {
	t.Run("Vertical scrollbar at top", func(t *testing.T) {
		pos := 0
		tmpl := Build(HBox(
			Text("Content"),
			Scrollbar(100, 10, &pos).Length(10),
		))

		buf := NewBuffer(20, 10)
		tmpl.Execute(buf, 20, 10)

		// Thumb should be at top (position 0)
		// Default thumb char is '█', track is '│'
		if buf.Get(7, 0).Rune != '█' {
			t.Errorf("thumb at pos 0: got %c, want █", buf.Get(7, 0).Rune)
		}
		// Track should be below the thumb
		if buf.Get(7, 9).Rune != '│' {
			t.Errorf("track: got %c, want │", buf.Get(7, 9).Rune)
		}
	})

	t.Run("Vertical scrollbar at bottom", func(t *testing.T) {
		pos := 90 // scrolled to bottom
		tmpl := Build(Scrollbar(100, 10, &pos).Length(10))

		buf := NewBuffer(5, 10)
		tmpl.Execute(buf, 5, 10)

		// Thumb should be at bottom
		// At pos=90 of range 0-90, thumb should be at track end
		if buf.Get(0, 9).Rune != '█' {
			t.Errorf("thumb at bottom: got %c, want █", buf.Get(0, 9).Rune)
		}
		// Track should be above
		if buf.Get(0, 0).Rune != '│' {
			t.Errorf("track at top: got %c, want │", buf.Get(0, 0).Rune)
		}
	})

	t.Run("Horizontal scrollbar", func(t *testing.T) {
		pos := 0
		tmpl := Build(Scrollbar(100, 10, &pos).Length(10).Horizontal())

		buf := NewBuffer(10, 3)
		tmpl.Execute(buf, 10, 3)

		// Thumb should be at left (position 0)
		if buf.Get(0, 0).Rune != '█' {
			t.Errorf("thumb at left: got %c, want █", buf.Get(0, 0).Rune)
		}
		// Track should be to the right
		if buf.Get(9, 0).Rune != '─' {
			t.Errorf("track at right: got %c, want ─", buf.Get(9, 0).Rune)
		}
	})

	t.Run("Scrollbar thumb moves with position", func(t *testing.T) {
		pos := 0
		tmpl := Build(Scrollbar(100, 10, &pos).Length(10))

		buf := NewBuffer(5, 10)

		// At top
		pos = 0
		tmpl.Execute(buf, 5, 10)
		topThumbPos := findThumbPosition(buf, 0, 10, false)
		if topThumbPos != 0 {
			t.Errorf("thumb at top: got position %d, want 0", topThumbPos)
		}

		// At middle
		buf.Clear()
		pos = 45 // middle of scroll range
		tmpl.Execute(buf, 5, 10)
		midThumbPos := findThumbPosition(buf, 0, 10, false)
		if midThumbPos <= 0 || midThumbPos >= 9 {
			t.Errorf("thumb at middle: got position %d, expected between 1-8", midThumbPos)
		}

		// At bottom
		buf.Clear()
		pos = 90
		tmpl.Execute(buf, 5, 10)
		bottomThumbPos := findThumbPosition(buf, 0, 10, false)
		if bottomThumbPos != 9 {
			t.Errorf("thumb at bottom: got position %d, want 9", bottomThumbPos)
		}
	})

	t.Run("Custom scrollbar characters", func(t *testing.T) {
		pos := 0
		tmpl := Build(Scrollbar(20, 5, &pos).Length(4).TrackChar('░').ThumbChar('▓'))

		buf := NewBuffer(5, 4)
		tmpl.Execute(buf, 5, 4)

		// Check custom characters
		if buf.Get(0, 0).Rune != '▓' {
			t.Errorf("custom thumb: got %c, want ▓", buf.Get(0, 0).Rune)
		}
		if buf.Get(0, 3).Rune != '░' {
			t.Errorf("custom track: got %c, want ░", buf.Get(0, 3).Rune)
		}
	})
}

// findThumbPosition finds the Y position (for vertical) or X position (for horizontal) of the thumb
func findThumbPosition(buf *Buffer, x, length int, horizontal bool) int {
	for i := 0; i < length; i++ {
		var char rune
		if horizontal {
			char = buf.Get(i, 0).Rune
		} else {
			char = buf.Get(x, i).Rune
		}
		if char == '█' || char == '▓' || char == '▁' || char == '▂' || char == '▃' || char == '▄' || char == '▅' || char == '▆' || char == '▇' || char == '▔' || char == '🮂' || char == '🮃' || char == '▀' || char == '🮄' || char == '🮅' || char == '🮆' {
			return i
		}
	}
	return -1
}

func TestTabsComponent(t *testing.T) {
	t.Run("Tabs with underline style", func(t *testing.T) {
		selected := 0
		tmpl := Build(Tabs([]string{"Home", "Settings", "Help"}, &selected))

		buf := NewBuffer(30, 3)
		tmpl.Execute(buf, 30, 3)
		output := buf.String()
		t.Logf("Tabs underline:\n%s", output)

		// Check labels are present
		if !strings.Contains(output, "Home") {
			t.Error("Output should contain 'Home'")
		}
		if !strings.Contains(output, "Settings") {
			t.Error("Output should contain 'Settings'")
		}
		if !strings.Contains(output, "Help") {
			t.Error("Output should contain 'Help'")
		}

		// First tab should have underline attribute (active)
		cell := buf.Get(0, 0)
		if cell.Rune != 'H' {
			t.Errorf("First char: got %c, want H", cell.Rune)
		}
		if !cell.Style.Attr.Has(AttrUnderline) {
			t.Error("First tab should be underlined (active)")
		}
	})

	t.Run("Tabs selection changes", func(t *testing.T) {
		selected := 1 // Select "Settings"
		tmpl := Build(Tabs([]string{"Home", "Settings"}, &selected))

		buf := NewBuffer(20, 3)
		tmpl.Execute(buf, 20, 3)

		// "Home" should NOT be underlined (gap=2 means "Settings" starts at position 6)
		if buf.Get(0, 0).Style.Attr.Has(AttrUnderline) {
			t.Error("First tab should NOT be underlined when not selected")
		}
		// "Settings" should be underlined
		if !buf.Get(6, 0).Style.Attr.Has(AttrUnderline) {
			t.Error("Second tab should be underlined when selected")
		}
	})

	t.Run("Tabs with bracket style", func(t *testing.T) {
		selected := 0
		tmpl := Build(Tabs([]string{"Tab1", "Tab2"}, &selected).Kind(TabsStyleBracket))

		buf := NewBuffer(20, 3)
		tmpl.Execute(buf, 20, 3)
		output := buf.String()
		t.Logf("Tabs bracket:\n%s", output)

		// Check bracket characters
		if !strings.Contains(output, "[Tab1]") {
			t.Error("Output should contain '[Tab1]'")
		}
		if !strings.Contains(output, "[Tab2]") {
			t.Error("Output should contain '[Tab2]'")
		}
	})

	t.Run("Tabs with box style", func(t *testing.T) {
		selected := 0
		tmpl := Build(Tabs([]string{"One", "Two"}, &selected).Kind(TabsStyleBox))

		buf := NewBuffer(30, 5)
		tmpl.Execute(buf, 30, 5)
		output := buf.String()
		t.Logf("Tabs box:\n%s", output)

		// Check for box characters
		if !strings.Contains(output, "┌") {
			t.Error("Output should contain box corner ┌")
		}
		if !strings.Contains(output, "│") {
			t.Error("Output should contain box side │")
		}
		if !strings.Contains(output, "One") {
			t.Error("Output should contain 'One'")
		}
	})

	t.Run("Tabs with custom gap", func(t *testing.T) {
		selected := 0
		tmpl := Build(Tabs([]string{"A", "B"}, &selected).Gap(5))

		buf := NewBuffer(20, 3)
		tmpl.Execute(buf, 20, 3)

		// "A" at 0, gap of 5, "B" at 6
		if buf.Get(0, 0).Rune != 'A' {
			t.Errorf("First tab: got %c, want A", buf.Get(0, 0).Rune)
		}
		if buf.Get(6, 0).Rune != 'B' {
			t.Errorf("Second tab at pos 6: got %c, want B", buf.Get(6, 0).Rune)
		}
	})

	t.Run("Tabs with styling", func(t *testing.T) {
		selected := 0
		tmpl := Build(Tabs([]string{"Active", "Inactive"}, &selected).
			ActiveStyle(Style{FG: Green}).
			InactiveStyle(Style{FG: White}))

		buf := NewBuffer(25, 3)
		tmpl.Execute(buf, 25, 3)

		// Active tab should have green FG
		if buf.Get(0, 0).Style.FG != Green {
			t.Error("Active tab should have green foreground")
		}
	})
}

func TestTreeViewComponent(t *testing.T) {
	t.Run("TreeView renders expanded tree", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "Root",
			Expanded: true,
			Children: []*TreeNode{
				{Label: "Child 1"},
				{Label: "Child 2"},
			},
		}
		tmpl := Build(TreeView{
			Root:     tree,
			ShowRoot: true,
		})

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()
		t.Logf("TreeView:\n%s", output)

		if !strings.Contains(output, "Root") {
			t.Error("Output should contain 'Root'")
		}
		if !strings.Contains(output, "Child 1") {
			t.Error("Output should contain 'Child 1'")
		}
		if !strings.Contains(output, "Child 2") {
			t.Error("Output should contain 'Child 2'")
		}
	})

	t.Run("TreeView collapsed hides children", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "Root",
			Expanded: false, // collapsed
			Children: []*TreeNode{
				{Label: "Hidden"},
			},
		}
		tmpl := Build(TreeView{
			Root:     tree,
			ShowRoot: true,
		})

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()
		t.Logf("TreeView collapsed:\n%s", output)

		if !strings.Contains(output, "Root") {
			t.Error("Output should contain 'Root'")
		}
		if strings.Contains(output, "Hidden") {
			t.Error("Output should NOT contain 'Hidden' when collapsed")
		}
		// Should show collapsed indicator
		if !strings.Contains(output, "▶") {
			t.Error("Output should contain collapsed indicator ▶")
		}
	})

	t.Run("TreeView shows expand indicator", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "Parent",
			Expanded: true,
			Children: []*TreeNode{
				{Label: "Child"},
			},
		}
		tmpl := Build(TreeView{
			Root:     tree,
			ShowRoot: true,
		})

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()

		// Should show expanded indicator
		if !strings.Contains(output, "▼") {
			t.Error("Output should contain expanded indicator ▼")
		}
	})

	t.Run("TreeView without root", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "HiddenRoot",
			Expanded: true,
			Children: []*TreeNode{
				{Label: "Child 1"},
				{Label: "Child 2"},
			},
		}
		tmpl := Build(TreeView{
			Root:     tree,
			ShowRoot: false, // don't show root
		})

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()
		t.Logf("TreeView no root:\n%s", output)

		if strings.Contains(output, "HiddenRoot") {
			t.Error("Output should NOT contain 'HiddenRoot' when ShowRoot=false")
		}
		if !strings.Contains(output, "Child 1") {
			t.Error("Output should contain 'Child 1'")
		}
	})

	t.Run("TreeView with nested levels", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "Root",
			Expanded: true,
			Children: []*TreeNode{
				{
					Label:    "Level 1",
					Expanded: true,
					Children: []*TreeNode{
						{
							Label:    "Level 2",
							Expanded: true,
							Children: []*TreeNode{
								{Label: "Level 3"},
							},
						},
					},
				},
			},
		}
		tmpl := Build(TreeView{
			Root:     tree,
			ShowRoot: true,
			Indent:   2,
		})

		buf := NewBuffer(30, 10)
		tmpl.Execute(buf, 30, 10)
		output := buf.String()
		t.Logf("TreeView nested:\n%s", output)

		if !strings.Contains(output, "Level 3") {
			t.Error("Output should contain 'Level 3'")
		}
	})

	t.Run("TreeView with custom characters", func(t *testing.T) {
		tree := &TreeNode{
			Label:    "Root",
			Expanded: true,
			Children: []*TreeNode{
				{Label: "Leaf"},
			},
		}
		tmpl := Build(TreeView{
			Root:          tree,
			ShowRoot:      true,
			ExpandedChar:  '-',
			CollapsedChar: '+',
			LeafChar:      '*',
		})

		buf := NewBuffer(20, 5)
		tmpl.Execute(buf, 20, 5)
		output := buf.String()
		t.Logf("TreeView custom chars:\n%s", output)

		if !strings.Contains(output, "-") {
			t.Error("Output should contain expanded indicator '-'")
		}
		if !strings.Contains(output, "*") {
			t.Error("Output should contain leaf indicator '*'")
		}
	})
}

// TestSparklineInHBoxGrowPanels tests the commandcenter scenario: multiple VBox.Grow(1)
// panels inside an HBox, each containing a Sparkline. The Sparkline must not overflow
// through the top border of its parent VBox.
func TestSparklineInHBoxGrowPanels(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 4, 3, 2, 1, 2}
	label := "142 req/s"

	tmpl := Build(
		VBox(
			HBox.Gap(1)(
				VBox.Border(BorderSingle).Grow(1).Title("requests/s")(Sparkline(data), Text(label)),
				VBox.Border(BorderSingle).Grow(1).Title("p99 latency")(Sparkline(data), Text(label)),
				VBox.Border(BorderSingle).Grow(1).Title("error rate")(Sparkline(data), Text(label)),
			),
			// service table with Grow(1) in root VBox context
			VBox.Border(BorderSingle).Grow(1)(Text("services")),
		),
	)

	buf := NewBuffer(80, 20)
	tmpl.Execute(buf, 80, 20)

	output := buf.String()
	lines := strings.Split(output, "\n")

	t.Logf("output:\n%s", output)

	// line 0 is the top border of the HBox panels — must contain a corner character
	topLine := lines[0]
	if !strings.ContainsAny(topLine, "┌╔+-") {
		t.Errorf("top border of panels missing or overwritten; got: %q", topLine)
	}

	// the sparkline/label content must appear on row 1 or later (inside the border)
	contentRow := -1
	for i, l := range lines {
		if strings.Contains(l, label) {
			contentRow = i
			break
		}
	}
	if contentRow < 1 {
		t.Errorf("panel content rendered above or on top border (row %d)", contentRow)
	}
}

// TestSparklineGrowInsideBorderedVBox is a regression test for a bug where
// a Sparkline inside a bordered VBox with Grow(1) would overflow through the
// top border. distributeFlexInCol was not including op.Margin[0] when
// recalculating child positions after flex distribution.
func TestSparklineGrowInsideBorderedVBox(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 4, 3, 2, 1, 2}

	tmpl := Build(
		VBox(
			VBox.Border(BorderSingle).Grow(1)(Sparkline(data), Text("label")),
		),
	)

	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	output := buf.String()
	lines := strings.Split(output, "\n")

	t.Logf("output:\n%s", output)

	// the top border must be on line 0 and must contain a corner character —
	// if the sparkline overflows, it overwrites the border line
	if len(lines) == 0 {
		t.Fatal("no output")
	}
	topLine := lines[0]
	if !strings.ContainsAny(topLine, "┌╔+-") {
		t.Errorf("top border missing or overwritten by sparkline content; got: %q", topLine)
	}

	// label must appear inside the box, not above the top border
	labelRow := -1
	for i, l := range lines {
		if strings.Contains(l, "label") {
			labelRow = i
			break
		}
	}
	if labelRow < 1 {
		t.Errorf("label rendered above or on top border (row %d)", labelRow)
	}
}

// TestForEachPerItemFlexPointersRebind guards the per-item binding rule for the
// flex pointer surfaces: a *float32 Grow and a *int16 Width read from a ForEach
// struct field must resolve to EACH element's value, not the compile-time
// placeholder. Two rows with different values must render differently.
func TestForEachPerItemFlexPointersRebind(t *testing.T) {
	// per-item Grow: row 0 grow 0 (text left), row 1 grow 1 (text pushed right)
	type grow struct {
		lg    float32
		label string
	}
	grows := []grow{{0, "AA"}, {1, "BB"}}
	gt := Build(VBox.Width(20)(
		ForEach(&grows, func(r *grow) Component {
			return HBox.Width(20)(Space().Grow(&r.lg), Text(&r.label))
		}),
	))
	gb := NewBuffer(20, 4)
	gt.Execute(gb, 20, 4)
	if a, b := strings.Index(gb.GetLine(0), "AA"), strings.Index(gb.GetLine(1), "BB"); a == b {
		t.Errorf("per-item Grow frozen: both rows place text at col %d", a)
	}

	// per-item Width on a spacer: row 0 pad 0, row 1 pad 8
	type pad struct {
		w     int16
		label string
	}
	pads := []pad{{0, "CC"}, {8, "DD"}}
	pt := Build(VBox.Width(20)(
		ForEach(&pads, func(r *pad) Component {
			return HBox.Width(20)(Space().Width(&r.w), Text(&r.label))
		}),
	))
	pb := NewBuffer(20, 4)
	pt.Execute(pb, 20, 4)
	if a, b := strings.Index(pb.GetLine(0), "CC"), strings.Index(pb.GetLine(1), "DD"); a == b {
		t.Errorf("per-item spacer Width frozen: both rows place text at col %d", a)
	}
}

func TestMaxWidthCapsFill(t *testing.T) {
	// MaxWidth is a pure cap: with the default fill sizing, a short label still
	// fills up to the bound (no implicit hug) — hugging is FitContent's job.
	tmpl := Build(VBox.MaxWidth(20)(Text("hi")))
	buf := NewBuffer(40, 3)
	tmpl.Execute(buf, 40, 3)
	if got := tmpl.geom[0].W; got != 20 {
		t.Errorf("MaxWidth alone caps the fill at 20, got W=%d", got)
	}
}

func TestMaxWidthFitContentHugsUnderCap(t *testing.T) {
	// FitContent + MaxWidth: hug the content, capped at the bound.
	tmpl := Build(VBox.FitContent().MaxWidth(20)(Text("hi")))
	buf := NewBuffer(40, 3)
	tmpl.Execute(buf, 40, 3)
	if got := tmpl.geom[0].W; got != 2 {
		t.Errorf("FitContent under the cap should hug to 2, got W=%d", got)
	}
}

func TestMaxWidthClampsLongSingleLine(t *testing.T) {
	// a long non-wrapping line clamps at the bound
	tmpl := Build(VBox.MaxWidth(20)(Text("this is a very long line exceeding twenty")))
	buf := NewBuffer(60, 3)
	tmpl.Execute(buf, 60, 3)
	if got := tmpl.geom[0].W; got != 20 {
		t.Errorf("long single line should clamp to 20, got W=%d", got)
	}
}

func TestMaxWidthWrapsToCappedWidth(t *testing.T) {
	// MaxWidth narrows the container, so wrappable content wraps to the cap —
	// the container is the cap width (20), not the longest wrapped line.
	tmpl := Build(VBox.MaxWidth(20)(TextBlock("the quick brown fox jumps over")))
	buf := NewBuffer(60, 5)
	tmpl.Execute(buf, 60, 5)
	if got := tmpl.geom[0].W; got != 20 {
		t.Errorf("MaxWidth caps the container to 20 (text wraps to it), got W=%d", got)
	}
	// and the text actually wrapped (more than one row used)
	if buf.GetLine(1) == "" {
		t.Error("text should wrap to a second line at the capped width")
	}
}

func TestMaxWidthFitContentBorderHugsUnderCap(t *testing.T) {
	// FitContent + MaxWidth + border: hug content + border chrome, under the cap.
	tmpl := Build(VBox.FitContent().MaxWidth(20).Border(BorderSingle)(Text("hey")))
	buf := NewBuffer(40, 5)
	tmpl.Execute(buf, 40, 5)
	// content 3 + border 2 = 5, under the bound so no clamp
	if got := tmpl.geom[0].W; got != 5 {
		t.Errorf("content+border should hug to 5, got W=%d", got)
	}
}

func BenchmarkMaxWidthNonWrapping(b *testing.B) {
	tmpl := Build(VBox.MaxWidth(20)(Text("short")))
	buf := NewBuffer(40, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 40, 3)
	}
}

func BenchmarkMaxWidthWrapMeasure(b *testing.B) {
	tmpl := Build(VBox.MaxWidth(20)(TextBlock("the quick brown fox jumps over the lazy dog again")))
	buf := NewBuffer(40, 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 40, 8)
	}
}

func TestMaxWidthClampsInContentSizingParent(t *testing.T) {
	// a MaxWidth child measured by a content-sizing parent must report its
	// CLAMPED width so both measurement paths agree (intrinsic + setOpWidth).
	tmpl := Build(HBox.FitContent()(
		VBox.MaxWidth(20)(Text("this label is forty characters long ok!!")),
		Text("x"),
	))
	buf := NewBuffer(80, 3)
	tmpl.Execute(buf, 80, 3)
	// clamped child 20 + sibling 1 = 21, not the unwrapped ~41
	if got := tmpl.geom[0].W; got != 21 {
		t.Errorf("content-sizing parent should size to clamped child (21), got W=%d", got)
	}
}

func TestFixedHeightClipsContent(t *testing.T) {
	// a VBox with explicit Height(2) holding 5 lines must show only the first 2
	// and NOT let the rest escape onto the sibling below.
	tmpl := Build(VBox(
		VBox.Height(2)(
			Text("L1"), Text("L2"), Text("L3"), Text("L4"), Text("L5"),
		),
		Text("AFTER"),
	))
	buf := NewBuffer(20, 12)
	tmpl.Execute(buf, 20, 12)

	get := func(y int) string { return strings.TrimRight(extractLine(buf, y, 8), " \x00") }
	if get(0) != "L1" {
		t.Errorf("row 0: want L1, got %q", get(0))
	}
	if get(1) != "L2" {
		t.Errorf("row 1: want L2, got %q", get(1))
	}
	// row 2 is the sibling — the box owns exactly 2 rows
	if get(2) != "AFTER" {
		t.Errorf("row 2: want AFTER (sibling at declared height), got %q", get(2))
	}
	// L3/L4/L5 must NOT have escaped the box onto rows 3+
	for y := 3; y < 8; y++ {
		if g := get(y); g != "" {
			t.Errorf("row %d: content escaped fixed-height box: %q", y, g)
		}
	}
}

func TestFixedHeightBorderedClipsBetweenBorders(t *testing.T) {
	// bordered fixed-height box reserves both border rows and clips content between
	tmpl := Build(VBox(
		VBox.Height(3).Border(BorderSingle)(
			Text("A"), Text("B"), Text("C"), Text("D"),
		),
		Text("AFTER"),
	))
	buf := NewBuffer(20, 12)
	tmpl.Execute(buf, 20, 12)
	get := func(y int) string { return strings.TrimRight(extractLine(buf, y, 8), " \x00") }
	// rows: 0 top border, 1 content (A), 2 bottom border, 3 sibling
	if buf.Get(0, 0).Rune != '┌' {
		t.Errorf("row 0 should be top border, got %q", buf.Get(0, 0).Rune)
	}
	if !strings.Contains(get(1), "A") {
		t.Errorf("row 1 should show A, got %q", get(1))
	}
	if get(3) != "AFTER" {
		t.Errorf("row 3 should be the sibling, got %q", get(3))
	}
	// B/C/D must not escape below the box
	for y := 4; y < 8; y++ {
		if g := get(y); g != "" {
			t.Errorf("row %d: content escaped bordered fixed-height box: %q", y, g)
		}
	}
}

func TestFixedHeightOverflowVisibleOptOut(t *testing.T) {
	// .Overflow(OverflowVisible) restores the old escape-the-box behaviour:
	// with 5 children in a Height(2) box, L4/L5 spill past the sibling row.
	tmpl := Build(VBox(
		VBox.Height(2).Overflow(OverflowVisible)(
			Text("L1"), Text("L2"), Text("L3"), Text("L4"), Text("L5"),
		),
		Text("AFTER"),
	))
	buf := NewBuffer(20, 12)
	tmpl.Execute(buf, 20, 12)
	get := func(y int) string { return strings.TrimRight(extractLine(buf, y, 8), " \x00") }
	// with overflow visible, the overflow lines escape below the sibling
	found := false
	for y := 3; y < 8; y++ {
		if strings.Contains(get(y), "L4") || strings.Contains(get(y), "L5") {
			found = true
		}
	}
	if !found {
		t.Error("Overflow(OverflowVisible) should let content escape the box")
	}
}

func TestFixedHeightClipsLayerView(t *testing.T) {
	// a LayerView taller than its fixed-height box must clip at the box edge,
	// not bleed past it — guards that Blit honours the active clip rect.
	layer := NewLayer()
	layer.EnsureSize(12, 6)
	for i := 0; i < 6; i++ {
		layer.SetLineString(i, "LAYERROW"+string(rune('0'+i)), DefaultStyle())
	}
	tmpl := Build(VBox(
		VBox.Height(2)(LayerView(layer).ViewHeight(6)),
		Text("AFTER"),
	))
	buf := NewBuffer(20, 12)
	tmpl.Execute(buf, 20, 12)
	get := func(y int) string { return strings.TrimRight(extractLine(buf, y, 12), " \x00") }
	// only the first 2 layer rows fit; row 2 is the sibling
	if !strings.Contains(get(0), "LAYERROW0") {
		t.Errorf("row 0 should show first layer row, got %q", get(0))
	}
	if get(2) != "AFTER" {
		t.Errorf("row 2 should be the sibling, got %q", get(2))
	}
	// layer rows 2..5 must NOT bleed below the box
	for y := 3; y < 9; y++ {
		if strings.Contains(get(y), "LAYERROW") {
			t.Errorf("row %d: layer content escaped the fixed-height box: %q", y, get(y))
		}
	}
}

func TestFixedHeightClipsPerForEachItem(t *testing.T) {
	// per-item fixed-height boxes in a ForEach each clip their own content
	// (exercises the renderSubOp clip path, not just top-level renderOp).
	type row struct{ lines []string }
	rows := []row{
		{lines: []string{"a1", "a2", "a3", "a4"}},
		{lines: []string{"b1", "b2", "b3", "b4"}},
	}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return VBox.Height(1)(
				ForEach(&r.lines, func(s *string) Component { return Text(s) }),
			)
		}),
	))
	buf := NewBuffer(20, 12)
	tmpl.Execute(buf, 20, 12)
	get := func(y int) string { return strings.TrimRight(extractLine(buf, y, 8), " \x00") }
	// each Height(1) box shows only its first line; rows stack 1-high
	if get(0) != "a1" {
		t.Errorf("row 0: want a1, got %q", get(0))
	}
	if get(1) != "b1" {
		t.Errorf("row 1: want b1 (second item, clipped to 1 high), got %q", get(1))
	}
	// no a2/a3/a4 or b2.. escaping
	for y := 2; y < 8; y++ {
		if g := get(y); g != "" {
			t.Errorf("row %d: per-item content escaped Height(1) box: %q", y, g)
		}
	}
}

func BenchmarkFixedHeightClipRender(b *testing.B) {
	// fixed-height container with overflowing content (clip path active)
	tmpl := Build(VBox.Height(10)(
		ForEach(&[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}, func(i *int) Component {
			return Text("row of dashboard content here")
		}),
	))
	buf := NewBuffer(60, 30)
	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		tmpl.Execute(buf, 60, 30)
	}
}

func BenchmarkNoClipRender(b *testing.B) {
	// same content, no fixed height (no clip — the common path, must be unchanged)
	tmpl := Build(VBox(
		ForEach(&[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}, func(i *int) Component {
			return Text("row of dashboard content here")
		}),
	))
	buf := NewBuffer(60, 30)
	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		tmpl.Execute(buf, 60, 30)
	}
}

// WidthPct must FAIL LOUD on an out-of-contract arg instead of silently no-opping (which
// vanished the element). The contract is a 0.0-1.0 fraction; an int or an out-of-range
// float panics at build with a message naming the right call.
func TestWidthPctFailsLoudOnBadArg(t *testing.T) {
	mustPanic := func(name string, build func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected a build-time panic, got none (silent no-op regressed)", name)
			}
		}()
		build()
	}

	mustNotPanic := func(name string, build func()) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("%s: unexpected panic %v — a static over/under-percent is legitimate", name, r)
			}
		}()
		build()
	}

	// The real footgun: a wrong TYPE silently dropped. An int reads as a percentage but
	// the fraction contract makes it ambiguous (and an int literal was the silent no-op
	// that vanished the element) — must panic, not drop.
	mustPanic("VBox.WidthPct(42)", func() { VBox.WidthPct(42)(Text("x")) })
	mustPanic("HBox.WidthPct(42)", func() { HBox.WidthPct(42)(Text("x")) })
	// unmatched type must panic.
	mustPanic("VBox.WidthPct(string)", func() { VBox.WidthPct("42")(Text("x")) })

	// A static float OUTSIDE [0,1] is NOT an error: >100% is a deliberate over-width and
	// <0% is a valid animation start/overshoot value — these must be accepted, not panicked.
	mustNotPanic("VBox.WidthPct(1.5)", func() { VBox.WidthPct(1.5)(Text("x")) })
	mustNotPanic("VBox.WidthPct(-0.1)", func() { VBox.WidthPct(-0.1)(Text("x")) })
	mustNotPanic("HBox.WidthPct(1.2)", func() { HBox.WidthPct(1.2)(Text("x")) })

	// the valid 0-1 fraction must still lay out correctly — regression guard.
	left, right := "L", "R"
	view := HBox.Width(40)(
		VBox.WidthPct(0.25)(Text(&left)),
		VBox.Grow(1)(Text(&right)),
	)
	buf := NewBuffer(40, 1)
	Build(view).Execute(buf, 40, 1)
	// 0.25 * 40 = 10 columns for the left col; "R" lands at column 10.
	if got := buf.GetLine(0); !strings.HasPrefix(got, "L") || got[10] != 'R' {
		t.Errorf("WidthPct(0.25) layout = %q, want L in col0 and R at col10", got)
	}

	// a static >100% must actually produce an over-width column (not dropped to 0).
	wide := "W"
	wt := Build(HBox.Width(40)(VBox.WidthPct(1.5)(Text(&wide))))
	wt.Execute(NewBuffer(60, 1), 60, 1)
	// 1.5 * 40 = 60 columns requested for the column (over the 40 parent) — accepted, not 0.
	if got := findPercentColWidth(wt); got <= 40 {
		t.Errorf("WidthPct(1.5) produced width %d, want >40 (over-width accepted, not clamped/dropped)", got)
	}
}

// findPercentColWidth returns the resolved width of the first OpContainer with a percent.
func findPercentColWidth(t *Template) int16 {
	for i := range t.ops {
		if t.ops[i].Kind == OpContainer && t.ops[i].percentWidth() > 0 {
			return t.geom[i].W
		}
	}
	return -1
}

// TestForEachItemHeightIsIntrinsic pins the sizing contract for list items.
//
// Width is distributed per item, so a Space() inside an item spreads
// horizontally. Height is not: an item is as tall as its content, and a Grow on
// a container inside an item has nothing to grow into. Anyone who changes this
// is changing how lists size, which is a different decision from how boxes size.
func TestForEachItemHeightIsIntrinsic(t *testing.T) {
	type row struct{ Name, Val string }
	rows := []row{{"a", "1"}, {"b", "2"}}

	// horizontal: Space() inside an item pushes Val to the right edge
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox(Text(&r.Name), Space(), Text(&r.Val))
		}),
	))
	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)
	if got := buf.Get(19, 0).Rune; got != '1' {
		t.Errorf("width is distributed per item, so Space() should push Val to column 19; got %q", got)
	}

	// vertical: a Grow inside an item does NOT expand into the taller box
	tmpl2 := Build(VBox.Height(int16(10))(
		ForEach(&rows, func(r *row) Component {
			return VBox.Grow(1)(Text(&r.Name))
		}),
	))
	tmpl2.Execute(NewBuffer(20, 10), 20, 10)

	for _, op := range tmpl2.ops {
		if op.Kind != OpForEach {
			continue
		}
		for i, g := range op.Ext.(*opForEach).geoms {
			if g.H != 1 {
				t.Errorf("item %d has height %d, want 1 — a ForEach item is intrinsic-height, so a "+
					"Grow inside it must not expand it. If this changed deliberately, it is a "+
					"list-sizing decision and needs its own design", i, g.H)
			}
		}
	}
}
