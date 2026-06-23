package glyph_test

import (
	"testing"

	. "github.com/kungfusheep/glyph"
)

// An If branch is laid out at content height (layout(0)) and only its ROOT used to be
// stretched to the row height afterwards — the branch's INTERNAL flex never re-ran, so
// Grow children and height-0 stretch elements (scrollbars, vrules) inside an If-wrapped
// pane kept the content-sized pass and collapsed (e.g. a scrollbar track rendering
// only 2 lines high). stretchIfContent now redistributes the branch's flex against the
// stretched height, mirroring propagateFlexToIf on the VBox flex path.
func TestIfBranchStretchRedistributesInternalFlex(t *testing.T) {
	show := true
	items := []string{"a", "b", "c"}
	sel := 0
	var off, vis, tot int

	const W, H = 60, 20
	view := VBox.Height(H).Width(W)(
		HBox.Grow(1).Gap(2)(
			VBox.Grow(1)(Text("LEFT")),
			// the If wrapper is the load-bearing part: without the redistribute the
			// scrollbar in this branch lays out against a content-sized (tiny) row.
			If(&show).Then(
				VBox.Grow(1)(
					Text("header"),
					HBox.Grow(1)(
						VBox.Grow(1)(
							List(&items).
								Selection(&sel).
								Render(func(s *string) Component { return Text(s) }).
								ScrollState(&off, &vis, &tot),
						),
						ScrollbarDyn(&tot, &vis, &off),
					),
				),
			),
		),
	)

	buf := NewBuffer(W, H)
	Build(view).Execute(buf, W, H)

	track := 0
	for y := 0; y < H; y++ {
		if r := buf.Get(W-1, y).Rune; r == '│' || r == '█' || (r >= '▁' && r <= '▇') {
			track++
		}
	}
	// the row below the header should fill the rest of the pane: track ≈ H-1.
	// Before the fix this was 0 (branch internals never saw the stretched height).
	if track < H-4 {
		t.Fatalf("scrollbar track inside the If branch is %d rows of a %d-row pane — branch flex not redistributed after stretch", track, H)
	}
}
