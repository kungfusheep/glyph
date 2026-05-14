package glyph

import (
	"strings"
	"testing"
)

func parityLayer(text string) *Layer {
	layer := NewLayer()
	layer.EnsureSize(12, 1)
	layer.SetLineString(0, text, DefaultStyle())
	return layer
}

func renderParityLines(c Component, w, h int16) (*Template, *Buffer, []string) {
	tmpl := Build(c)
	buf := NewBuffer(int(w), int(h))
	tmpl.Execute(buf, w, h)
	lines := make([]string, h)
	for y := int16(0); y < h; y++ {
		lines[y] = buf.GetLine(int(y))
	}
	return tmpl, buf, lines
}

func TestRenderPathAB_LayerViewInheritedFill(t *testing.T) {
	fill := RGB(10, 20, 30)

	_, directBuf, directLines := renderParityLines(
		VBox.Fill(fill)(
			LayerView(parityLayer("direct")).ViewHeight(1),
		),
		20, 2,
	)
	_, foreachBuf, foreachLines := renderParityLines(
		VBox.Fill(fill)(
			ForEach(&[]string{"item"}, func(item *string) Component {
				return LayerView(parityLayer("foreach")).ViewHeight(1)
			}),
		),
		20, 2,
	)

	if !strings.Contains(directLines[0], "direct") {
		t.Fatalf("direct layer did not render: %q", directLines[0])
	}
	if !strings.Contains(foreachLines[0], "foreach") {
		t.Fatalf("foreach layer did not render: %q", foreachLines[0])
	}

	directBG := directBuf.Get(0, 0).Style.BG
	foreachBG := foreachBuf.Get(0, 0).Style.BG
	if directBG != fill {
		t.Fatalf("direct LayerView should inherit container fill, got %#v", directBG)
	}
	if foreachBG != fill {
		t.Fatalf("sub-template LayerView should inherit container fill; direct=%#v foreach=%#v", directBG, foreachBG)
	}
	t.Logf("direct LayerView BG=%#v, foreach LayerView BG=%#v", directBG, foreachBG)
}

func TestRenderPathAB_RuleExtendInSubTemplate(t *testing.T) {
	ruleView := func(label string) Component {
		return VBox.Border(BorderSingle).FitContent()(
			HRule().Extend(),
			HBox.MarginVH(0, 1).Gap(1)(
				VBox.Width(6)(
					Text(label),
					HRule().Extend(),
					Text("B"),
				),
				VRule().Extend(),
				VBox.Width(6)(
					Text("C"),
					HRule().Extend(),
					Text("D"),
				),
			),
		)
	}

	_, directBuf, _ := renderParityLines(ruleView("A"), 30, 8)
	_, foreachBuf, _ := renderParityLines(
		ForEach(&[]string{"A"}, func(item *string) Component {
			return ruleView(*item)
		}),
		30, 8,
	)

	countRune := func(buf *Buffer, r rune) int {
		count := 0
		for y := 0; y < buf.Height(); y++ {
			for x := 0; x < buf.Width(); x++ {
				if buf.Get(x, y).Rune == r {
					count++
				}
			}
		}
		return count
	}

	directCrosses := countRune(directBuf, '\u253c')
	foreachCrosses := countRune(foreachBuf, '\u253c')
	if directCrosses == 0 {
		t.Fatal("direct rule view should produce extend junctions")
	}
	if foreachCrosses >= directCrosses {
		t.Fatalf("current sub-template rule extend unexpectedly matched direct path: direct=%d foreach=%d", directCrosses, foreachCrosses)
	}
	t.Logf("direct rule junctions=%d, foreach rule junctions=%d", directCrosses, foreachCrosses)
}

func TestRenderPathAB_AutoTableInSubTemplate(t *testing.T) {
	type row struct {
		Name string
		Age  int
	}
	rows := []row{{Name: "Ada", Age: 36}}
	items := []string{"table"}

	_, _, directLines := renderParityLines(AutoTable(rows), 30, 4)
	_, _, foreachLines := renderParityLines(
		ForEach(&items, func(item *string) Component {
			return AutoTable(rows)
		}),
		30, 4,
	)

	direct := strings.Join(directLines, "\n")
	foreach := strings.Join(foreachLines, "\n")
	if !strings.Contains(direct, "Ada") {
		t.Fatalf("direct AutoTable did not render row: %q", direct)
	}
	if strings.Contains(foreach, "Ada") {
		t.Fatalf("current sub-template AutoTable unexpectedly rendered row: %q", foreach)
	}
	t.Logf("direct AutoTable output=%q, foreach AutoTable output=%q", direct, foreach)
}

func TestRenderPathAB_OverlayAndScreenEffectCollection(t *testing.T) {
	effect := funcEffect(func(buf *Buffer, ctx PostContext) {})
	items := []string{"item"}

	directTmpl, _, directLines := renderParityLines(
		VBox(
			Text("base"),
			Overlay.At(0, 0)(Text("overlay")),
			ScreenEffect(effect),
		),
		20, 3,
	)
	foreachTmpl, _, foreachLines := renderParityLines(
		VBox(
			ForEach(&items, func(item *string) Component {
				return VBox(
					Text("base"),
					Overlay.At(0, 0)(Text("overlay")),
					ScreenEffect(effect),
				)
			}),
		),
		20, 3,
	)

	direct := strings.Join(directLines, "\n")
	foreach := strings.Join(foreachLines, "\n")
	t.Logf("direct overlay/effect output=%q effects=%d", direct, len(directTmpl.ScreenEffects()))
	t.Logf("foreach overlay/effect output=%q effects=%d", foreach, len(foreachTmpl.ScreenEffects()))
	if !strings.Contains(direct, "overlay") {
		t.Fatalf("direct overlay did not render: %q", direct)
	}
	if !strings.Contains(foreach, "base") {
		t.Fatalf("foreach sub-template body did not render: %q", foreach)
	}
	if !strings.Contains(foreach, "overlay") {
		t.Fatalf("sub-template overlay did not render: %q", foreach)
	}
	if got := len(directTmpl.ScreenEffects()); got != 1 {
		t.Fatalf("direct screen effect count = %d, want 1", got)
	}
	if got := len(foreachTmpl.ScreenEffects()); got != 1 {
		t.Fatalf("sub-template screen effect count = %d, want 1", got)
	}
	t.Logf("direct effects=%d foreach effects=%d", len(directTmpl.ScreenEffects()), len(foreachTmpl.ScreenEffects()))
}
