package main

import (
	"log"

	. "github.com/kungfusheep/glyph"
)

// preservebgdemo is the visual harness for ADR 4: PreserveBG. The left column
// stamps decorations the ordinary way — each glyph carries its own background,
// punching a hole in whatever sits beneath. The right column adds .PreserveBG()
// to the same decorations: rune and foreground land, the background underneath
// shows through. The two columns are otherwise identical.
//
// keys: q quit.
func main() {
	app := NewApp()

	bg := Hex(0x1E1E2E)
	stripeA := Hex(0x313244)
	stripeB := Hex(0x45475A)
	banner := Hex(0x7C3AED)
	accent := Hex(0xF9E2AF)
	mark := Hex(0xA6E3A1)
	dim := Hex(0x9399B2)
	// chipBG is a background the decoration carries from wherever it was
	// styled — the realistic case PreserveBG exists for. Plain writes stamp
	// it (a hole the wrong colour on every surface); PreserveBG drops it.
	chipBG := Hex(0x11111B)

	// a striped list: alternating row fills, the classic varied background
	stripes := func(decorate bool) Component {
		rows := make([]Component, 0, 6)
		labels := []string{"deploy", "rollback", "scale", "drain", "restart", "purge"}
		for i, name := range labels {
			fill := stripeA
			if i%2 == 1 {
				fill = stripeB
			}
			name := name
			badge := Text(" ●done ").FG(mark).BG(chipBG)
			if decorate {
				badge = Text(" ●done ").FG(mark).BG(chipBG).PreserveBG()
			}
			rows = append(rows, HBox.Height(1).Fill(fill)(
				Text("  "+name).FG(accent),
				Space(),
				badge,
			))
		}
		return VBox(rows...)
	}

	// a banner with a glyph drawn over it
	bannerRow := func(decorate bool) Component {
		glyph := Text(" ★ featured ★ ").FG(accent).BG(chipBG)
		if decorate {
			glyph = Text(" ★ featured ★ ").FG(accent).BG(chipBG).PreserveBG()
		}
		return HBox.Height(1).Fill(banner)(
			Text("  release 0.8.0").FG(Hex(0xFFFFFF)),
			Space(),
			glyph,
		)
	}

	// an underline drawn across a two-tone split, the focus-bar case
	underline := func(decorate bool) Component {
		u := Text("▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁").FG(mark).BG(chipBG)
		if decorate {
			u = Text("▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁").FG(mark).BG(chipBG).PreserveBG()
		}
		return VBox(
			HBox.Height(1)(
				HBox.Width(10).Fill(stripeA)(Text("  left").FG(dim)),
				HBox.Width(10).Fill(banner)(Text("  right").FG(dim)),
			),
			u,
		)
	}

	column := func(title string, decorate bool) Component {
		return VBox.Gap(1).WidthPct(0.5).PaddingVH(0, 1)(
			Text(title).Bold().FG(Hex(0xFFFFFF)),
			Text("striped list — badge over alternating fills").FG(dim),
			stripes(decorate),
			Text("banner — glyph over a solid bar").FG(dim),
			bannerRow(decorate),
			Text("underline — bar across a two-tone split").FG(dim),
			underline(decorate),
		)
	}

	app.SetView(
		VBox.Gap(1).PaddingVH(1, 2).Fill(bg).CascadeStyle(&Style{BG: bg})(
			HBox.Gap(2)(
				Text("preservebgdemo").Bold().FG(accent),
				Text("ADR 4 — same decorations, left plain, right .PreserveBG()").FG(dim),
				Space(),
				Text("[q] quit").FG(dim),
			),
			HBox.Gap(2)(
				column("PLAIN — own background", false),
				column("PRESERVEBG — background shows through", true),
			),
		),
	)

	app.Handle("q", app.Stop)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
