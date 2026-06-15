package glyph

import (
	"strings"
	"testing"
)

func TestForEachPerItemBorderFG(t *testing.T) {
	type msg struct {
		Who   string
		Color Color
	}
	msgs := []msg{{"A", Red}, {"B", Blue}}
	tmpl := Build(VBox(ForEach(&msgs, func(m *msg) Component {
		return VBox.Border(BorderRounded).BorderFG(&m.Color)(Text(&m.Who))
	})))
	buf := NewBuffer(20, 8)
	tmpl.Execute(buf, 20, 8)

	out := buf.String()
	if !strings.ContainsAny(out, "╭╮╰╯│─") {
		t.Fatalf("border dropped:\n%s", out)
	}
	// item A border (row 0) must be Red; item B border (row 3) must be Blue —
	// proving the per-item pointer rebinds, not a single frozen colour.
	topA := buf.Get(0, 0)
	topB := buf.Get(0, 3)
	if topA.Rune != '╭' || topA.Style.FG != Red {
		t.Errorf("item A border = %q FG %+v, want ╭ Red", topA.Rune, topA.Style.FG)
	}
	if topB.Rune != '╭' || topB.Style.FG != Blue {
		t.Errorf("item B border = %q FG %+v, want ╭ Blue", topB.Rune, topB.Style.FG)
	}
}

func TestStableBorderFGPointerStillRenders(t *testing.T) {
	bc := Green
	title := "x"
	tmpl := Build(VBox.Border(BorderRounded).BorderFG(&bc)(Text(&title)))
	buf := NewBuffer(12, 4)
	tmpl.Execute(buf, 12, 4)
	if c := buf.Get(0, 0); c.Rune != '╭' || c.Style.FG != Green {
		t.Errorf("stable BorderFG pointer: got %q FG %+v, want ╭ Green", c.Rune, c.Style.FG)
	}
}
