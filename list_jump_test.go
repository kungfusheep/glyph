package glyph

import (
	"fmt"
	"testing"
)

// a Jump next to an explicit-flex sibling used to be starved to zero width
// (its content drew only by overflow, and styled rows painted over it).
func TestListRendersJumpItemChildren(t *testing.T) {
	type row struct {
		Name  string
		Color Color
	}
	rows := []row{{"alpha", Red}, {"beta", Blue}}
	sel := 0

	buf := NewBuffer(30, 5)
	tmpl := Build(VBox(
		List(&rows).
			Selection(&sel).
			Render(func(r *row) Component {
				return HBox(
					Text("|").FG(&r.Color),
					SpaceW(1),
					JumpItem(Text(&r.Name), func(r *row) {}),
					Space(),
				)
			}).
			Marker("").
			Style(Style{FG: White, BG: RGB(25, 25, 24)}),
	))
	tmpl.Execute(buf, 30, 5)
	for y := 0; y < 3; y++ {
		t.Log(fmt.Sprintf("y=%d %q", y, buf.GetLine(y)))
	}
	if got := buf.GetLine(0); got != "> | alpha" {
		t.Errorf("line0 = %q, want jump-wrapped name rendered", got)
	}
}
