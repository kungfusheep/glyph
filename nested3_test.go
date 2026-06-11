package glyph

import (
	"fmt"
	"testing"
)

func TestTripleNestedForEachRendersInnerItems(t *testing.T) {
	type ev struct {
		Time  string
		Title string
	}
	type cell struct {
		Label    string
		Selected bool
		Max      int
		More     int
		Items    []ev
	}
	type week struct{ Cells []cell }

	weeks := []week{{Cells: []cell{{
		Label: "5",
		Max:   2,
		Items: []ev{{"09:00", "alpha"}, {"", "beta"}, {"10:00", "gamma"}},
	}}}}

	buf := NewBuffer(30, 8)
	tmpl := Build(VBox(
		ForEach(&weeks, func(w *week) Component {
			return HBox.Height(6)(
				ForEach(&w.Cells, func(c *cell) Component {
					bg := If(&c.Selected).Then(RGB(50, 50, 80)).Else(RGB(20, 20, 20))
					return VBox.Width(12).Height(6).Fill(bg)(
						Text(&c.Label),
						ForEach(&c.Items).Limit(&c.Max).Remaining(&c.More)(func(e *ev) Component {
							return HBox.Height(1)(
								Text("|").Width(1).BG(bg),
								HBox.Width(10)(
									If(&e.Time).
										Then(Textf(&e.Time, " ", &e.Title).CharWrap()).
										Else(Text(&e.Title)),
								),
							)
						}),
						IfOrd(&c.More).Gt(0).Then(
							HBox.Height(1)(Text(&c.More), Text(" more")),
						),
					)
				}),
			)
		}),
	))
	tmpl.Execute(buf, 30, 8)

	for y := 0; y < 6; y++ {
		t.Log(fmt.Sprintf("y=%d %q", y, buf.GetLine(y)))
	}
	if got := buf.GetLine(1); got != "|09:00 alph" {
		t.Errorf("line1 = %q, want timed alpha row", got)
	}
	if got := buf.GetLine(2); got != "|beta" {
		t.Errorf("line2 = %q, want plain beta row", got)
	}
	// the overflow count is an item-field *int: it must render the live value
	// written by the inner ForEach, not a snapshot of the compile placeholder
	if got := buf.GetLine(3); got != "1 more" {
		t.Errorf("line3 = %q, want live overflow count", got)
	}
	if weeks[0].Cells[0].More != 1 {
		t.Errorf("more = %d, want 1", weeks[0].Cells[0].More)
	}
}
