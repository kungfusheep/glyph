package glyph

import "testing"

// ADR 4: a PreserveBG write keeps the destination cell's background even when
// the source declares its own BG — only the rune and FG land. Direct buffer
// write so the destination is unambiguous (not cascade-inherited).
func TestPreserveBGKeepsDestinationBG(t *testing.T) {
	red := RGB(200, 0, 0)
	green := RGB(0, 200, 0)
	accent := RGB(0, 0, 240)

	buf := NewBuffer(4, 1)
	// paint the destination red
	buf.SetFast(0, 0, Cell{Rune: ' ', Style: Style{BG: red}})
	// write a glyph that declares green BG + PreserveBG: green must be ignored
	buf.SetFast(0, 0, Cell{Rune: '▁', Style: Style{FG: accent, BG: green, Attr: AttrPreserveBG}})

	c := buf.Get(0, 0)
	if c.Rune != '▁' || c.Style.FG != accent {
		t.Fatalf("rune/FG = %q/%+v, want underline/accent", string(c.Rune), c.Style.FG)
	}
	if c.Style.BG != red {
		t.Fatalf("BG = %+v, want destination red (source green ignored)", c.Style.BG)
	}
	if c.Style.Attr.Has(AttrPreserveBG) {
		t.Fatal("AttrPreserveBG persisted on the cell; must be stripped at write")
	}
}

// the control: without the flag, the source BG wins.
func TestPlainWriteOverwritesBG(t *testing.T) {
	red := RGB(200, 0, 0)
	green := RGB(0, 200, 0)
	buf := NewBuffer(4, 1)
	buf.SetFast(0, 0, Cell{Rune: ' ', Style: Style{BG: red}})
	buf.SetFast(0, 0, Cell{Rune: 'X', Style: Style{BG: green}})
	if buf.Get(0, 0).Style.BG != green {
		t.Fatal("plain write should stamp its own BG over the destination")
	}
}

// the text render path (WriteStringFast) carries the flag end to end. Text
// declares its own BG; over a container fill the fill must show through.
func TestPreserveBGTextRenderPath(t *testing.T) {
	red := RGB(200, 0, 0)
	tmpl := Build(VBox.Fill(red)(
		Text("ab").FG(RGB(255, 255, 255)).BG(RGB(0, 200, 0)).PreserveBG(),
	))
	buf := NewBuffer(6, 2)
	tmpl.Execute(buf, 6, 2)
	c := buf.Get(0, 0)
	if c.Rune != 'a' {
		t.Fatalf("rune = %q, want a", string(c.Rune))
	}
	if c.Style.BG != red {
		t.Fatalf("BG = %+v, want fill red — the text's own green BG must be dropped", c.Style.BG)
	}
}

// the span path (Rich/Textf) honours node-level PreserveBG for static and
// live spans alike.
func TestPreserveBGRichKeepsDestinationBG(t *testing.T) {
	blue := RGB(0, 0, 180)
	tmpl := Build(VBox.Fill(blue)(
		Rich(Span{Text: "ab", Style: Style{FG: RGB(240, 240, 240), BG: RGB(0, 200, 0)}}).PreserveBG(),
	))
	buf := NewBuffer(6, 2)
	tmpl.Execute(buf, 6, 2)
	c := buf.Get(0, 0)
	if c.Rune != 'a' {
		t.Fatalf("rune = %q, want a", string(c.Rune))
	}
	if c.Style.BG != blue {
		t.Fatalf("span BG = %+v, want fill blue preserved", c.Style.BG)
	}
	if c.Style.Attr.Has(AttrPreserveBG) {
		t.Fatal("AttrPreserveBG persisted on a span cell")
	}
}

// the spec-required interaction: a node carrying both opacity and PreserveBG
// composites the dest-BG rule ONCE. PreserveBG resolves at write time (BG is
// already the destination's), then opacity blends the result — so the blend
// sees a normal cell, not a double-applied background.
func TestPreserveBGWithOpacityCompositesOnce(t *testing.T) {
	red := RGB(200, 0, 0)
	tmpl := Build(VBox.Fill(red)(
		Text("Z").FG(RGB(255, 255, 255)).PreserveBG().Opacity(0.5),
	))
	buf := NewBuffer(4, 2)
	tmpl.Execute(buf, 4, 2)
	c := buf.Get(0, 0)
	// the cell's BG derives from red on both the source (preserved) and the
	// backing (the fill) — so the opacity blend of BG-against-BG is still red,
	// not a half-tone of red against some other colour.
	if c.Style.BG != red {
		t.Fatalf("BG = %+v, want red — double application would shift it", c.Style.BG)
	}
	if c.Style.Attr.Has(AttrPreserveBG) {
		t.Fatal("AttrPreserveBG survived the opacity path")
	}
}
