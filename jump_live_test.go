package glyph

import (
	"testing"

	"github.com/kungfusheep/riffkey"
)

// TestJumpFeedbackLiveMultiCharInput drives the REAL input path: with >27
// targets (two-char labels), typing the first key must accumulate into
// jumpMode.Input so the feedback engages — the live bug was that riffkey
// buffered the first key as a pending sequence prefix and Input never updated.
func TestJumpFeedbackLiveMultiCharInput(t *testing.T) {
	items := make([]string, 30) // >27 → two-char labels
	for i := range items {
		items[i] = "x"
	}
	selected := -1
	app := NewApp()
	// 3-column grid so all 30 targets are visible at once (a single column would
	// clip past the test viewport height and fall back to single-char labels).
	const cols = 3
	rows := (len(items) + cols - 1) / cols
	columns := make([]Component, cols)
	for c := 0; c < cols; c++ {
		cells := make([]Component, 0, rows)
		for r := 0; r < rows; r++ {
			i := r*cols + c
			if i >= len(items) {
				break
			}
			idx := i
			cells = append(cells, Jump(Text(&items[idx]).Width(6), func() { selected = idx }))
		}
		columns[c] = VBox(cells...)
	}
	app.SetView(HBox.Gap(2)(columns...))
	app.RenderNow()
	app.EnterJumpMode()
	if !app.JumpModeActive() {
		t.Fatal("not in jump mode")
	}
	first := app.JumpMode().Targets[0].Label
	if len(first) < 2 {
		t.Fatalf("expected two-char labels, got %q", first)
	}

	// type the first char: Input must accumulate (the bug: it stayed "")
	app.Input().Dispatch(riffkey.Key{Rune: rune(first[0])})
	if got := app.JumpMode().Input; got != first[:1] {
		t.Fatalf("after first key, Input = %q, want %q — feedback never engaged live", got, first[:1])
	}
	if !app.JumpModeActive() {
		t.Fatal("exited jump mode on a partial match")
	}

	// type the second char: completes the label, selects, exits
	app.Input().Dispatch(riffkey.Key{Rune: rune(first[1])})
	if app.JumpModeActive() {
		t.Fatal("still in jump mode after a full label")
	}
	if selected != 0 {
		t.Fatalf("selected = %d, want 0 (first target)", selected)
	}
}
