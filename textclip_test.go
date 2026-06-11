package glyph

import (
	"strings"
	"testing"
)

// a rich text node inside a fixed-height container must not paint past the
// container's bottom edge, even when its content wraps to more lines.
func TestRichText_ClippedByFixedHeightContainer(t *testing.T) {
	buf := NewBuffer(20, 5)
	title := "a very long event title that wraps"
	tmpl := Build(VBox(
		HBox.Height(1)(
			Textf("09:00 ", &title),
		),
		Text("below"),
	))
	tmpl.Execute(buf, 20, 5)

	line0 := buf.GetLine(0)
	if !strings.HasPrefix(line0, "09:00 a very") {
		t.Errorf("line 0: got %q, want prefix %q", line0, "09:00 a very")
	}
	if got := buf.GetLine(1); got != "below" {
		t.Errorf("line 1: got %q, want %q (rich text spilled past its row)", got, "below")
	}
}

// same clip must hold inside a ForEach render func — each Height(1) row owns
// exactly one buffer line, with no bleed into the following row.
func TestRichText_ClippedInForEachRow(t *testing.T) {
	type row struct {
		Time  string
		Title string
	}
	rows := []row{
		{Time: "09:00", Title: "standup meeting with the whole team"},
		{Time: "11:30", Title: "lunch"},
	}

	buf := NewBuffer(20, 5)
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox.Height(1)(
				Textf(&r.Time, " ", &r.Title),
			)
		}),
	))
	tmpl.Execute(buf, 20, 5)

	if got := buf.GetLine(0); !strings.HasPrefix(got, "09:00 standup") {
		t.Errorf("line 0: got %q, want prefix %q", got, "09:00 standup")
	}
	if got := buf.GetLine(1); !strings.HasPrefix(got, "11:30 lunch") {
		t.Errorf("line 1: got %q, want prefix %q (row 0 spilled into row 1)", got, "11:30 lunch")
	}
}

// an If branch must size offset-bound text from the item's real content and
// never paint past its parent's edge — inside ForEach the placeholder width
// (textWidth with nil base) used to cap branches at 10 cells, and outside it
// the branch's own intrinsic width let text bleed past fixed-width boxes.
func TestIfBranchText_ClampedToParentWidth(t *testing.T) {
	type row struct {
		Time  string
		Title string
	}
	rows := []row{{Time: "", Title: "Antidisestablishmentarianism Review"}}

	buf := NewBuffer(40, 2)
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox.Height(1)(
				Text("|").Width(1),
				HBox.Width(19)(
					If(&r.Time).
						Then(Textf(&r.Time, " ", &r.Title).CharWrap()).
						Else(Text(&r.Title)),
				),
			)
		}),
	))
	tmpl.Execute(buf, 40, 2)

	if got := buf.GetLine(0); got != "|Antidisestablishmen" {
		t.Errorf("foreach: got %q, want %q", got, "|Antidisestablishmen")
	}

	buf2 := NewBuffer(40, 2)
	title := "Antidisestablishmentarianism Review"
	empty := ""
	tmpl2 := Build(VBox(
		HBox.Height(1)(
			Text("|").Width(1),
			HBox.Width(19)(
				If(&empty).
					Then(Textf(&empty, " ", &title).CharWrap()).
					Else(Text(&title)),
			),
		),
	))
	tmpl2.Execute(buf2, 40, 2)

	if got := buf2.GetLine(0); got != "|Antidisestablishmen" {
		t.Errorf("plain: got %q, want %q", got, "|Antidisestablishmen")
	}
}

// CharWrap on Textf fills the single visible line character-exact instead of
// stopping at the last word boundary that fits.
func TestTextf_CharWrapTruncatesAtWidth(t *testing.T) {
	buf := NewBuffer(12, 3)
	tmpl := Build(VBox(
		HBox.Height(1)(
			Textf("meeting with marketing").CharWrap(),
		),
	))
	tmpl.Execute(buf, 12, 3)

	if got := buf.GetLine(0); got != "meeting with" {
		t.Errorf("line 0: got %q, want %q", got, "meeting with")
	}
	if got := buf.GetLine(1); got != "" {
		t.Errorf("line 1: got %q, want empty (char wrap spilled past its row)", got)
	}
}
