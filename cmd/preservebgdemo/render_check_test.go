package main

import (
	"testing"

	. "github.com/kungfusheep/glyph"
)

// the demo's point made testable: a decoration carrying its OWN background,
// dropped onto a stripe. Plain stamps the chip bg (a hole); PreserveBG keeps
// the stripe (seamless).
func TestDemoBadgePreserveVsPlain(t *testing.T) {
	stripe := Hex(0x313244)
	mark := Hex(0xA6E3A1)
	chipBG := Hex(0x11111B)

	build := func(preserve bool) Cell {
		badge := Text("●").FG(mark).BG(chipBG)
		if preserve {
			badge = Text("●").FG(mark).BG(chipBG).PreserveBG()
		}
		tmpl := Build(HBox.Height(1).Fill(stripe)(Text("x"), Space(), badge))
		b := NewBuffer(10, 1)
		tmpl.Execute(b, 10, 1)
		return b.Get(9, 0)
	}

	plain := build(false)
	pres := build(true)
	if plain.Style.BG != chipBG {
		t.Fatalf("plain badge BG = %+v, want chip bg punched through", plain.Style.BG)
	}
	if pres.Style.BG != stripe {
		t.Fatalf("preserve badge BG = %+v, want stripe shown through", pres.Style.BG)
	}
	if plain.Style.BG == pres.Style.BG {
		t.Fatal("columns identical — the demo would show no difference")
	}
}
