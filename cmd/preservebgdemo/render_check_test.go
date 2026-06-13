package main

import (
	"testing"

	. "github.com/kungfusheep/glyph"
)

// the badge-over-stripe case: a decoration carrying its own bg, dropped onto a
// stripe. Plain stamps the chip bg; PreserveBG keeps the stripe.
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

	plain, pres := build(false), build(true)
	if plain.Style.BG != chipBG {
		t.Fatalf("plain badge BG = %+v, want chip bg punched through", plain.Style.BG)
	}
	if pres.Style.BG != stripe {
		t.Fatalf("preserve badge BG = %+v, want stripe shown through", pres.Style.BG)
	}
}

// the overlay-over-two-tone case (the panel Pete flagged): an overlaid bar
// spanning two differently-filled panes. Plain stamps chipBG across BOTH;
// PreserveBG keeps stripeA on the left half and banner on the right.
func TestDemoOverlayBarSpansTwoTone(t *testing.T) {
	stripeA := Hex(0x313244)
	banner := Hex(0x7C3AED)
	mark := Hex(0xA6E3A1)
	chipBG := Hex(0x11111B)

	build := func(preserve bool) *Buffer {
		var ref NodeRef
		bar := Text("▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔").FG(mark).BG(chipBG)
		if preserve {
			bar = bar.PreserveBG()
		}
		tmpl := Build(VBox(
			HBox.Height(1).NodeRef(&ref)(
				HBox.Width(10).Fill(stripeA)(Text("L")),
				HBox.Width(10).Fill(banner)(Text("R")),
			),
			Overlay.OnTop(&ref)(bar),
		))
		b := NewBuffer(20, 2)
		tmpl.Execute(b, 20, 2)
		return b
	}

	plain := build(false)
	pres := build(true)

	// left half (col 2) and right half (col 12) of the overlaid bar row
	if got := plain.Get(2, 0).Style.BG; got != chipBG {
		t.Fatalf("plain bar left BG = %+v, want chipBG hole", got)
	}
	if got := plain.Get(12, 0).Style.BG; got != chipBG {
		t.Fatalf("plain bar right BG = %+v, want chipBG hole", got)
	}
	if got := pres.Get(2, 0).Style.BG; got != stripeA {
		t.Fatalf("preserve bar left BG = %+v, want stripeA shown through", got)
	}
	if got := pres.Get(12, 0).Style.BG; got != banner {
		t.Fatalf("preserve bar right BG = %+v, want banner shown through", got)
	}
}
