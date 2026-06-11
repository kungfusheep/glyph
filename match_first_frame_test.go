package glyph

import (
	"fmt"
	"testing"
)

func TestMatchBranchStyledOnFirstFrame(t *testing.T) {
	type row struct {
		Rule  string
		Color Color
	}
	rows := []row{{Rule: "─", Color: RGB(95, 91, 85)}}

	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox.Height(1).Width(20)(
				Match(&r.Rule,
					Eq("─", HRule().Char('─').FG(&r.Color)),
					Eq("·", HRule().Char('·').FG(&r.Color)),
				).Default(Text("")),
			)
		}),
	))

	for frame := 1; frame <= 2; frame++ {
		buf := NewBuffer(20, 3)
		tmpl.Execute(buf, 20, 3)
		c := buf.Get(0, 0)
		t.Log(fmt.Sprintf("frame%d rune=%q fg=%v", frame, c.Rune, c.Style.FG))
		if frame == 1 && !c.Style.FG.Equal(RGB(95, 91, 85)) {
			t.Errorf("frame1 fg=%v, want rule colour from the first frame", c.Style.FG)
		}
	}
}
