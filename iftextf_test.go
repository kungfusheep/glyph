package glyph

import (
	"fmt"
	"testing"
)

// a rich-text If branch must report its natural width — it measured zero,
// so the branch collapsed and flex siblings swallowed the row.
func TestIfTextfLabelSwitches(t *testing.T) {
	label := ""
	tint := RGB(40, 40, 40)

	view := VBox(
		HBox.Width(36).Fill(tint)(
			HBox.Width(23)(
				Jump(
					If(&label).
						Then(Textf(" ", Styled(&label, Style{Attr: AttrBold}))).
						Else(Text(" select calendar")),
					func() {},
				),
				Space(),
			),
			Space(),
		),
	)
	tmpl := Build(view)

	buf := NewBuffer(40, 3)
	tmpl.Execute(buf, 40, 3)
	t.Log(fmt.Sprintf("empty: %q", buf.GetLine(0)))

	label = "Family"
	buf2 := NewBuffer(40, 3)
	tmpl.Execute(buf2, 40, 3)
	t.Log(fmt.Sprintf("set:   %q", buf2.GetLine(0)))
	if got := buf2.GetLine(0); got != "Family" {
		t.Errorf("line = %q, want label rendered", got)
	}
}
