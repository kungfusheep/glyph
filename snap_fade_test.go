package glyph

import (
	"fmt"
	"testing"
	"time"
)

// TestSnapInFadeOut verifies the exact behaviour we want:
// - selecting an item: background SNAPS to selBG immediately (no animation)
// - deselecting an item: background FADES from selBG back to defBG over time
func TestSnapInFadeOut(t *testing.T) {
	type Row struct {
		Label    string
		Selected bool
		Grouped  bool
	}

	rows := []Row{
		{Label: "first", Selected: true},
		{Label: "second", Selected: false},
		{Label: "third", Selected: false},
	}

	selBG := Hex(0x2e2e2e)
	defBG := Hex(0x1a1a1a)
	grpBG := Hex(0x242424)
	sel := 0
	dur := 400 * time.Millisecond
	fade := Animate.Duration(dur).Ease(EaseOutCubic)
	_ = grpBG

	buf := NewBuffer(30, 6)
	tmpl := Build(VBox(
		List(&rows).Selection(&sel).SelectedStyle(Style{}).Marker("").Render(func(row *Row) Component {
			// matches the mail app pattern: fade inside inner condition Else
			bg := If(&row.Selected).Then(selBG).Else(
				If(&row.Grouped).Then(grpBG).Else(fade(defBG)),
			)
			return VBox.Fill(bg)(
				Text(&row.Label),
			)
		}),
	))

	getBG := func(y int) Color {
		// scan across the row to find first cell with a non-default BG
		for x := 0; x < 30; x++ {
			c := buf.Get(x, y)
			if c.Style.BG.Mode != 0 {
				return c.Style.BG
			}
		}
		return Color{}
	}

	dump := func(label string) {
		for y := 0; y < 3; y++ {
			for x := 0; x < 20; x++ {
				c := buf.Get(x, y)
				if c.Rune > ' ' || c.Style.BG.Mode != 0 {
					fmt.Printf("  %s cell(%d,%d) r=%c bg=%v\n", label, x, y, c.Rune, c.Style.BG)
				}
			}
		}
	}

	// Frame 0: initial render. Item 0 selected, items 1,2 not.
	tmpl.Execute(buf, 30, 6)
	dump("F0")
	fmt.Printf("Frame 0: row0=%v row1=%v row2=%v\n", getBG(0), getBG(1), getBG(2))

	if getBG(0) != selBG {
		t.Errorf("frame 0: row 0 should be selBG %v, got %v", selBG, getBG(0))
	}
	if getBG(1) != defBG {
		t.Errorf("frame 0: row 1 should be defBG %v, got %v", defBG, getBG(1))
	}

	// Now move selection from 0 to 1
	rows[0].Selected = false
	rows[1].Selected = true
	sel = 1

	// Frame 1: immediately after selection change
	buf.ClearDirty()
	tmpl.Execute(buf, 30, 6)
	fmt.Printf("Frame 1: row0=%v row1=%v row2=%v\n", getBG(0), getBG(1), getBG(2))

	// Row 1 should SNAP to selBG (Then branch — no animation)
	if getBG(1) != selBG {
		t.Errorf("frame 1: row 1 should SNAP to selBG %v, got %v", selBG, getBG(1))
	}

	// Row 0 should be at selBG (start of fade) or animating — NOT at defBG yet
	bg0 := getBG(0)
	if bg0 == defBG {
		t.Errorf("frame 1: row 0 should be starting fade (not at defBG yet), got %v", bg0)
	}

	// Frame 2: tiny time later — should be between selBG and defBG
	time.Sleep(50 * time.Millisecond)
	buf.ClearDirty()
	tmpl.Execute(buf, 30, 6)
	bg0mid := getBG(0)
	fmt.Printf("Frame 2: row0=%v (should be between selBG and defBG)\n", bg0mid)
	if bg0mid == selBG {
		t.Errorf("frame 2: row 0 should have started animating, still at selBG %v", bg0mid)
	}
	if bg0mid == defBG {
		t.Errorf("frame 2: row 0 should still be animating, already at defBG %v", bg0mid)
	}

	// Row 2 should still be at defBG (never selected)
	if getBG(2) != defBG {
		t.Errorf("frame 1: row 2 should still be defBG %v, got %v", defBG, getBG(2))
	}

	// Simulate time passing (animation complete)
	time.Sleep(dur + 50*time.Millisecond)
	buf.ClearDirty()
	tmpl.Execute(buf, 30, 6)
	fmt.Printf("Frame N: row0=%v row1=%v row2=%v\n", getBG(0), getBG(1), getBG(2))

	// Row 0 should now be at defBG (animation complete)
	if getBG(0) != defBG {
		t.Errorf("frame N: row 0 should have settled to defBG %v, got %v", defBG, getBG(0))
	}
	// Row 1 still selected
	if getBG(1) != selBG {
		t.Errorf("frame N: row 1 should still be selBG %v, got %v", selBG, getBG(1))
	}
}
