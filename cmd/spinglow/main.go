package main

import (
	"log"
	"time"

	. "github.com/kungfusheep/glyph"
)

// spinglow demo — two views:
//
//	hero    the big swirling install-pill, keyboard-tweakable Radius,
//	        Falloff and Strength. tab → subtle.
//	subtle  four dialed-down use cases that hint at where this effect
//	        could slot into real UI. tab → hero.
func main() {
	app := NewApp()

	pink := RGB(255, 80, 120)
	violet := RGB(150, 100, 255)
	amber := RGB(255, 170, 70)
	teal := RGB(80, 220, 200)
	fillBG := RGB(0x13, 0x13, 0x11)

	box := func(ref *NodeRef, text string) Component {
		return VBox.PaddingVH(0, 1).FitContent().Border(BorderSoft).
			BorderFG(DefaultColor()).Fill(fillBG).NodeRef(ref)(
			Text(text).FG(amber),
		)
	}

	// ── hero view ────────────────────────────────────────────────────────

	var swirlRef NodeRef
	targetFalloff := 4.0
	targetRadius := int16(29)
	targetStrength := 0.7
	smooth := Animate.Duration(800 * time.Millisecond).Ease(EaseOutCubic)

	heroTpl := HBox(
		Space(),
		VBox.Grow(1).Gap(4).Margin(2).FitContent()(
			Text("SESpinGlow — hero").Bold(),
			Text("-/=  falloff   [/]  radius   ,/.  strength   tab  subtle   q  quit").FG(BrightBlack),
			Space(),
			box(&swirlRef, "$ go get github.com/kungfusheep/glyph@latest"),
			Space().Grow(1),
			ScreenEffect(
				SESpinGlow(&swirlRef, pink, amber, violet, teal).
					Rim(true).
					Falloff(smooth(&targetFalloff)).
					Radius(smooth(&targetRadius)).
					Strength(smooth(&targetStrength)),
			),
		),
		Space(),
	)

	// ── subtle view ──────────────────────────────────────────────────────

	// 1. Tab focus — soft rim marks the focused tab. j/k to move.
	focused := 0
	var tabRefs [4]NodeRef
	tabLabels := []string{"Dashboard", "Logs", "Settings", "About"}
	smoothFocus := Animate.Duration(220 * time.Millisecond).Ease(EaseOutCubic)

	tab := func(ref *NodeRef, label string) Component {
		return VBox.PaddingVH(0, 3).FitContent().Fill(fillBG).NodeRef(ref)(
			Text(label).FG(amber),
		)
	}

	// 2. Processing marker — larger panel with a rotating cool halo.
	var procRef NodeRef

	// 3. Success banner — full-width pulse on keypress.
	var okRef NodeRef
	okStrength := 0.0
	smoothFlash := Animate.Duration(700 * time.Millisecond).Ease(EaseOutQuad)

	section := func(title string, child Component) Component {
		return VBox.Gap(1).FitContent()(
			Text(title).FG(BrightBlack),
			child,
		)
	}

	subtleTpl := HBox(
		Space(),
		VBox.Grow(1).Gap(3).Margin(2).FitContent()(
			Text("SESpinGlow — subtle").Bold(),
			Text("j/k  focus    s  flash    tab  hero    q  quit").FG(BrightBlack),
			Space(),

			section("Focus indicator — rim-only marks the current tab",
				HBox.Gap(2)(
					tab(&tabRefs[0], tabLabels[0]),
					tab(&tabRefs[1], tabLabels[1]),
					tab(&tabRefs[2], tabLabels[2]),
					tab(&tabRefs[3], tabLabels[3]),
				),
			),

			section("Processing marker — rotating cool halo signals activity",
				VBox.PaddingVH(1, 4).FitContent().Fill(fillBG).NodeRef(&procRef)(
					Text("fetching remote config").FG(amber),
					Text("this may take a moment…").FG(BrightBlack),
				),
			),

			section("Attention flash — press s to trigger a brief success pulse",
				VBox.PaddingVH(1, 3).FitContent().Fill(fillBG).NodeRef(&okRef)(
					Text("✓ Message sent").FG(RGB(140, 230, 160)).Bold(),
				),
			),

			Space().Grow(1),

			ScreenEffect(
				// 1. Focus tabs — one effect per tab, halo only (no rim).
				//    Strength is conditional on `focused` and wrapped in a
				//    tween so focus SLIDES between tabs as you press j/k.
				SESpinGlow(&tabRefs[0], pink, violet).Radius(4).Falloff(1).
					Strength(smoothFocus(If(&focused).Eq(0).Then(0.7).Else(0.0))),
				SESpinGlow(&tabRefs[1], pink, violet).Radius(4).Falloff(1).
					Strength(smoothFocus(If(&focused).Eq(1).Then(0.7).Else(0.0))),
				SESpinGlow(&tabRefs[2], pink, violet).Radius(4).Falloff(1).
					Strength(smoothFocus(If(&focused).Eq(2).Then(0.7).Else(0.0))),
				SESpinGlow(&tabRefs[3], pink, violet).Radius(4).Falloff(1).
					Strength(smoothFocus(If(&focused).Eq(3).Then(0.7).Else(0.0))),

				// 2. Processing — medium rotation, soft cool palette.
				SESpinGlow(&procRef, violet, teal, violet).
					Strength(0.55).Radius(5).Speed(1.5).Falloff(1),

				// 3. Success flash — Strength pulses via a tween triggered
				//    by `s`. Pair palette with a green Fill so the chip
				//    reads as a success banner even when halo is at rest.
				SESpinGlow(&okRef, RGB(140, 230, 160), RGB(80, 200, 120)).
					Radius(8).Falloff(1).
					Strength(smoothFlash(&okStrength)),
			),
		),
		Space(),
	)

	// ── views + routing ──────────────────────────────────────────────────

	// each view has its own router. q and <Tab> are registered on both
	// views so they work everywhere; per-view controls stay on their view.
	app.View("hero", heroTpl).
		Handle("q", app.Stop).
		Handle("<Tab>", func() { app.Go("subtle") }).
		Handle("-", func() { targetFalloff += 2 }).
		Handle("=", func() {
			if targetFalloff > 0 {
				targetFalloff -= 2
			}
		}).
		Handle("]", func() { targetRadius += 3 }).
		Handle("[", func() {
			if targetRadius > 4 {
				targetRadius -= 3
			}
		}).
		Handle(".", func() { targetStrength += 0.1 }).
		Handle(",", func() {
			if targetStrength > 0.1 {
				targetStrength -= 0.1
			}
		})

	app.View("subtle", subtleTpl).
		Handle("q", app.Stop).
		Handle("<Tab>", func() { app.Go("hero") }).
		Handle("j", func() {
			if focused < len(tabLabels)-1 {
				focused++
			}
		}).
		Handle("k", func() {
			if focused > 0 {
				focused--
			}
		}).
		Handle("s", func() {
			// flash: snap to 1, let the tween ease it back to 0.
			okStrength = 1.0
			go func() {
				time.Sleep(50 * time.Millisecond)
				okStrength = 0.0
			}()
		})

	// 60fps render heartbeat for the animated effects.
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			app.RequestRender()
		}
	}()

	if err := app.RunFrom("hero"); err != nil {
		log.Fatal(err)
	}
}
