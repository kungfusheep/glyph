package main

import (
	"testing"

	. "github.com/kungfusheep/glyph"
)

// the overlay must anchor correctly when the split is nested deep in the
// column layout (NodeRef is absolute, but depth is the risk). Build a column
// shaped like the real one and confirm the bar lands on the split's row with
// the two-tone preserved.
func TestColumnOverlayAnchorsWhenNested(t *testing.T) {
	stripeA := Hex(0x313244)
	banner := Hex(0x7C3AED)
	mark := Hex(0xA6E3A1)
	chipBG := Hex(0x11111B)

	var ref NodeRef
	view := VBox.Gap(1).PaddingVH(1, 2)(
		Text("header"),
		VBox.WidthPct(0.5).PaddingVH(0, 1)(
			Text("label one"),
			Text("label two"),
			HBox.Height(1).NodeRef(&ref)(
				HBox.Width(10).Fill(stripeA)(Text("L")),
				HBox.Width(10).Fill(banner)(Text("R")),
			),
			Overlay.OnTop(&ref)(Text("▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔").FG(mark).BG(chipBG).PreserveBG()),
		),
	)
	b := NewBuffer(60, 12)
	Build(view).Execute(b, 60, 12)

	// find the bar: scan for a row whose left/right halves carry the two fills
	found := false
	for y := 0; y < 12 && !found; y++ {
		for x := 0; x < 50; x++ {
			c := b.Get(x, y)
			if c.Rune == '▔' && c.Style.BG == stripeA {
				// the right pane begins 10 cols on; check it shows banner
				if b.Get(x+10, y).Style.BG == banner {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Fatal("overlaid bar did not anchor over the nested two-tone split with both fills preserved")
	}
}
