package glyph

import (
	"strings"
	"testing"
)

// regressions for the leaf-compiler offset audit: every pointer binding on a
// leaf component must follow each ForEach element's own field, not the
// compile-time dummy's. Two rows with different values; both must render
// their own.

func TestForEachSpinnerManualFramePerItem(t *testing.T) {
	type Row struct{ Frame int }
	rows := []Row{{Frame: 0}, {Frame: 3}}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return Spinner(&r.Frame).Frames(SpinnerDots)
		}),
	))
	buf := NewBuffer(4, 3)
	tmpl.Execute(buf, 4, 3)
	if buf.Get(0, 0).Rune == buf.Get(0, 1).Rune {
		t.Fatal("both rows rendered the same spinner frame; frame binding frozen on dummy")
	}
}

func TestForEachLeaderValuePerItem(t *testing.T) {
	type Row struct {
		Val string
		N   int
	}
	rows := []Row{{Val: "alpha", N: 1}, {Val: "beta", N: 2}}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return Leader("k", &r.Val).Width(12)
		}),
	))
	buf := NewBuffer(14, 3)
	tmpl.Execute(buf, 14, 3)
	if !strings.Contains(buf.GetLine(0), "alpha") || !strings.Contains(buf.GetLine(1), "beta") {
		t.Fatalf("leader values frozen: %q / %q", buf.GetLine(0), buf.GetLine(1))
	}

	tmpl2 := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return Leader("n", &r.N).Width(8)
		}),
	))
	buf2 := NewBuffer(10, 3)
	tmpl2.Execute(buf2, 10, 3)
	if !strings.Contains(buf2.GetLine(0), "1") || !strings.Contains(buf2.GetLine(1), "2") {
		t.Fatalf("leader int values frozen: %q / %q", buf2.GetLine(0), buf2.GetLine(1))
	}
}

func TestForEachSparklinePerItem(t *testing.T) {
	type Row struct{ Data []float64 }
	rows := []Row{
		{Data: []float64{0, 0, 0, 0}},
		{Data: []float64{8, 8, 8, 8}},
	}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return Sparkline(&r.Data).Width(4).Range(0, 8)
		}),
	))
	buf := NewBuffer(6, 3)
	tmpl.Execute(buf, 6, 3)
	if buf.Get(0, 0).Rune == buf.Get(0, 1).Rune {
		t.Fatalf("sparkline rows identical (%q); values binding frozen on dummy", string(buf.Get(0, 0).Rune))
	}
}

func TestForEachTabsSelectedPerItem(t *testing.T) {
	type Row struct{ Sel int }
	rows := []Row{{Sel: 0}, {Sel: 1}}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return Tabs([]string{"aa", "bb"}, &r.Sel).Kind(TabsStyleBracket).
				ActiveStyle(Style{Attr: AttrBold})
		}),
	))
	buf := NewBuffer(16, 3)
	tmpl.Execute(buf, 16, 3)
	// selection shows via ActiveStyle: row 0 selects "aa" (bold at x=1),
	// row 1 selects "bb" — the first label must differ between rows
	if buf.Get(1, 0).Style.Attr&AttrBold == 0 {
		t.Fatal("row 0 first label not bold; selection lost")
	}
	if buf.Get(1, 1).Style.Attr&AttrBold != 0 {
		t.Fatal("row 1 first label bold; selection frozen on dummy value")
	}
}

func TestForEachScrollbarPosPerItem(t *testing.T) {
	type Row struct{ Pos int }
	rows := []Row{{Pos: 0}, {Pos: 90}}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return HBox.Height(1)(Scrollbar(100, 10, &r.Pos).Horizontal())
		}),
	))
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)
	if buf.GetLine(0) == buf.GetLine(1) {
		t.Fatalf("scrollbar thumb identical on both rows (%q); pos binding frozen", buf.GetLine(0))
	}
}
