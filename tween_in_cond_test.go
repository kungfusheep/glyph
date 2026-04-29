package glyph

import (
	"testing"
	"time"
)

func TestConditionFillInsideForEach(t *testing.T) {
	type Item struct {
		Selected bool
		Label    string
	}

	items := []Item{
		{Selected: true, Label: "first"},
		{Selected: false, Label: "second"},
	}

	selBG := Hex(0x2e2e2e)
	defBG := Hex(0x1a1a1a)

	buf := NewBuffer(20, 4)
	tmpl := Build(VBox(
		ForEach(&items, func(item *Item) Component {
			bg := If(&item.Selected).Then(selBG).Else(defBG)
			return VBox.Fill(bg)(
				Text(&item.Label),
			)
		}),
	))

	tmpl.Execute(buf, 20, 4)

	cell0 := buf.Get(0, 0)
	if cell0.Style.BG != selBG {
		t.Errorf("item 0 BG = %v, want selBG %v", cell0.Style.BG, selBG)
	}

	cell1 := buf.Get(0, 1)
	if cell1.Style.BG != defBG {
		t.Errorf("item 1 BG = %v, want defBG %v", cell1.Style.BG, defBG)
	}
}

func TestTweenInsideConditionElse(t *testing.T) {
	type Item struct {
		Selected bool
		Label    string
	}

	items := []Item{
		{Selected: true, Label: "first"},
		{Selected: false, Label: "second"},
	}

	selBG := Hex(0x2e2e2e)
	defBG := Hex(0x1a1a1a)
	fade := Animate.Duration(400 * time.Millisecond).Ease(EaseOutCubic)

	buf := NewBuffer(20, 4)
	tmpl := Build(VBox(
		ForEach(&items, func(item *Item) Component {
			bg := If(&item.Selected).Then(selBG).Else(fade(defBG))
			return VBox.Fill(bg)(
				Text(&item.Label),
			)
		}),
	))

	tmpl.Execute(buf, 20, 4)

	// selected item should have selBG (snap — Then branch)
	cell0 := buf.Get(0, 0)
	if cell0.Style.BG != selBG {
		t.Errorf("item 0 BG = %v, want selBG %v", cell0.Style.BG, selBG)
	}

	// unselected item should have defBG (tween settled)
	cell1 := buf.Get(0, 1)
	if cell1.Style.BG != defBG {
		t.Errorf("item 1 BG = %v, want defBG %v", cell1.Style.BG, defBG)
	}

	// swap selection
	items[0].Selected = false
	items[1].Selected = true

	buf.Clear()
	tmpl.Execute(buf, 20, 4)

	// item 1 should snap to selBG
	cell1 = buf.Get(0, 1)
	if cell1.Style.BG != selBG {
		t.Errorf("after swap: item 1 BG = %v, want selBG %v", cell1.Style.BG, selBG)
	}

	// item 0 should be at selBG (start of fade) — NOT already at defBG
	cell0 = buf.Get(0, 0)
	if cell0.Style.BG == defBG {
		t.Errorf("after swap: item 0 should be starting fade from selBG, already at defBG %v", cell0.Style.BG)
	}

	// after time passes, item 0 should settle to defBG
	time.Sleep(500 * time.Millisecond)
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	cell0 = buf.Get(2, 0) // offset for marker
	if cell0.Style.BG != defBG {
		t.Errorf("after animation: item 0 should be defBG %v, got %v", defBG, cell0.Style.BG)
	}
}
