package tui

import (
	"testing"
)

func TestDeclarativeBasic(t *testing.T) {
	// Reset test data
	demoData.Title = "Test Dashboard"
	demoData.CPULoad = 50
	demoData.ShowCPU = true
	demoData.Mode = "normal"

	frame := Execute(declarativeUI)
	frame.Layout(60, 20)

	buf := NewBuffer(60, 20)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Verify content
	if !containsString(output, "=== Dashboard ===") {
		t.Error("Missing header")
	}
	if !containsString(output, "Test Dashboard") {
		t.Error("Missing title from pointer binding")
	}
	if !containsString(output, "CPU:") {
		t.Error("Missing CPU (should be visible)")
	}
	if !containsString(output, "Mode: Normal") {
		t.Error("Missing mode from switch")
	}
	if !containsString(output, "nginx") {
		t.Error("Missing process from ForEach")
	}
}

func TestDeclarativeConditional(t *testing.T) {
	// Test with CPU hidden
	demoData.ShowCPU = false
	demoData.Mode = "debug"

	frame := Execute(declarativeUI)
	frame.Layout(60, 20)

	buf := NewBuffer(60, 20)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Output:\n%s", output)

	if containsString(output, "CPU:") {
		t.Error("CPU should be hidden")
	}
	if !containsString(output, "CPU hidden") {
		t.Error("Missing 'CPU hidden' text from Else")
	}
	if !containsString(output, "Mode: DEBUG") {
		t.Error("Missing debug mode from switch")
	}

	// Reset
	demoData.ShowCPU = true
	demoData.Mode = "normal"
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
		 len(haystack) > len(needle) && containsSubstring(haystack, needle))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestInteractiveButton(t *testing.T) {
	clicked := false

	ui := DCol{
		Children: []any{
			DButton{
				Label:   "Click Me",
				OnClick: func() { clicked = true },
			},
		},
	}

	frame := Execute(ui)
	frame.Layout(40, 10)

	// Should have 1 focusable (the button)
	if len(frame.focusables) != 1 {
		t.Errorf("Expected 1 focusable, got %d", len(frame.focusables))
	}

	// Button should be focused by default (first focusable)
	if !frame.IsFocused(frame.focusables[0]) {
		t.Error("Button should be focused")
	}

	// Activate should trigger OnClick
	frame.Activate()
	if !clicked {
		t.Error("OnClick should have been called")
	}
}

func TestInteractiveCheckbox(t *testing.T) {
	checked := false
	changed := false

	ui := DCol{
		Children: []any{
			DCheckbox{
				Checked:  &checked,
				Label:    "Toggle me",
				OnChange: func(v bool) { changed = true },
			},
		},
	}

	frame := Execute(ui)
	frame.Layout(40, 10)

	// Initial state
	if checked {
		t.Error("Should start unchecked")
	}

	// Activate toggles
	frame.Activate()
	if !checked {
		t.Error("Should be checked after activate")
	}
	if !changed {
		t.Error("OnChange should have been called")
	}

	// Re-execute to see updated checkbox
	frame = Execute(ui)
	frame.Layout(40, 10)

	buf := NewBuffer(40, 10)
	frame.Render(buf)
	output := buf.String()

	if !containsString(output, "[x]") {
		t.Error("Checkbox should show [x] when checked")
	}
}

func TestFocusNavigation(t *testing.T) {
	ui := DCol{
		Children: []any{
			DButton{Label: "First"},
			DButton{Label: "Second"},
			DButton{Label: "Third"},
		},
	}

	frame := Execute(ui)
	frame.Layout(40, 10)

	// Should have 3 focusables
	if len(frame.focusables) != 3 {
		t.Errorf("Expected 3 focusables, got %d", len(frame.focusables))
	}

	// Start at first
	if frame.focusIndex != 0 {
		t.Error("Should start at index 0")
	}

	// Tab forward
	frame.FocusNext()
	if frame.focusIndex != 1 {
		t.Errorf("After FocusNext, expected 1, got %d", frame.focusIndex)
	}

	frame.FocusNext()
	if frame.focusIndex != 2 {
		t.Errorf("After FocusNext, expected 2, got %d", frame.focusIndex)
	}

	// Wrap around
	frame.FocusNext()
	if frame.focusIndex != 0 {
		t.Errorf("After FocusNext (wrap), expected 0, got %d", frame.focusIndex)
	}

	// Tab backward
	frame.FocusPrev()
	if frame.focusIndex != 2 {
		t.Errorf("After FocusPrev (wrap), expected 2, got %d", frame.focusIndex)
	}
}
