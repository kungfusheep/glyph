package glyph

import "testing"

// a Fill bound to a ForEach item field must resolve against the element being
// rendered — a raw pointer into the compile-time placeholder painted nothing.
func TestFillBoundToForEachItemField(t *testing.T) {
	type slot struct {
		Kind  string
		Title string
		BG    Color
	}
	type row struct {
		Slots []slot
	}
	tintA := RGB(57, 66, 86)
	tintB := RGB(86, 57, 66)
	rows := []row{
		{Slots: []slot{{Kind: "event", Title: "alpha", BG: tintA}}},
		{Slots: []slot{{Kind: "event", Title: "alex", BG: tintB}}},
	}
	slotW := int16(8)
	targetW := int16(5)

	buf := NewBuffer(20, 3)
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox.Height(1)(
				ForEach(&r.Slots, func(s *slot) Component {
					return HBox.Width(&slotW).Height(1).Fill(&s.BG)(
						Match(&s.Kind,
							Eq("event",
								HBox.Fill(&s.BG)(
									Text("|").Width(1),
									Text("M").FG(&s.BG).BG(&s.BG),
									Text(" ").BG(&s.BG),
									HBox.Width(&targetW)(
										JumpItemRef(Textf(FG(&s.Title, White)), func(s *slot, ref NodeRef) {}),
									),
								),
							),
						).Default(Text("-")),
					)
				}),
			)
		}),
	))
	tmpl.Execute(buf, 20, 3)

	for x := 1; x < 8; x++ {
		if got := buf.Get(x, 0).Style.BG; !got.Equal(tintA) {
			t.Errorf("row 0 x=%d bg=%v, want item A fill across the slot", x, got)
		}
		if got := buf.Get(x, 1).Style.BG; !got.Equal(tintB) {
			t.Errorf("row 1 x=%d bg=%v, want item B fill across the slot", x, got)
		}
	}
}
