// cacheeffects demonstrates ADR 19: screen effects animating over a STATIC
// dashboard. The app content never changes, so Execute is skipped and only the
// effect pipeline re-runs each frame. The footer shows the proof — full Executes
// climb slowly (a 2s heartbeat) while effect frames climb ~per-frame. Two effects
// run: a warm pulse on the text and a vignette breathing in and out at the edges.
package main

import (
	"fmt"
	"log"
	"math"
	"time"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

var (
	fullExecutes int // bumped inside a bound Text — i.e. once per real Execute
	effectFrames int // bumped inside the effect's Apply — once per frame (incl. skips)
)

// pulse is a self-animating effect: it brightens/darkens the whole buffer on a
// sine over ctx.Time and requests another frame each tick. Because it animates
// via the effect pipeline (not template state), those frames skip Execute.
type pulse struct{}

func (pulse) Apply(buf *Buffer, ctx PostContext) {
	effectFrames++
	amt := 0.35 + 0.35*math.Sin(ctx.Time.Seconds()*2.2)
	w, h := ctx.Width, ctx.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := buf.Get(x, y)
			c.Style.FG = Lerp(c.Style.FG, RGB(255, 180, 60), amt)
			buf.Set(x, y, c)
		}
	}
	ctx.RequestAnimation()
}

// oscVignette breathes a vignette in and out forever. The strength is computed
// from ctx.Time INSIDE the effect (not bound into the template), so the
// oscillation lives entirely on the effect-frame path and every breath skips
// Execute. Drive the same oscillation with a template-level Osc instead and the
// template reads as Animating(), which forces a full Execute every frame — the
// exact thing the cache path avoids. So a built-in SEVignette().Strength(Osc(…))
// would animate, but NOT skip; this is how you oscillate one over the cache.
type oscVignette struct{}

func (oscVignette) Apply(buf *Buffer, ctx PostContext) {
	effectFrames++
	// strength breathes between 0.15 (barely there) and 0.85 (heavy edges).
	strength := 0.5 + 0.35*math.Sin(ctx.Time.Seconds()*1.5)
	w, h := ctx.Width, ctx.Height
	cx, cy := float64(w)/2, float64(h)/2
	maxX := math.Max(cx, float64(w)-cx)
	maxY := math.Max(cy, float64(h)-cy) * 2
	maxDist := math.Sqrt(maxX*maxX + maxY*maxY)
	black := RGB(0, 0, 0)
	for y := 0; y < h; y++ {
		dy := (float64(y) - cy) * 2
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dist := math.Sqrt(dx*dx+dy*dy) / maxDist
			dim := dist * dist * strength // quadratic falloff, same shape as SEVignette
			if dim > 1 {
				dim = 1
			}
			c := buf.Get(x, y)
			c.Style.FG = Lerp(c.Style.FG, black, dim)
			c.Style.BG = Lerp(c.Style.BG, black, dim)
			buf.Set(x, y, c)
		}
	}
	ctx.RequestAnimation()
}

func main() {
	app := NewApp()
	app.SetDefaultStyle(Style{FG: RGB(200, 210, 220), BG: RGB(12, 14, 20)})

	rows := []string{
		"CPU   ▏███████████▏  62%",
		"MEM   ▏████████▏      48%",
		"NET   ▏██████████████ 91%",
		"DISK  ▏████▏          21%",
	}
	footer := func() string {
		fullExecutes++
		return fmt.Sprintf(" Execute()s: %d   effect frames: %d   (effect animates, Execute skipped) ",
			fullExecutes, effectFrames)
	}

	children := []Component{
		Text("  CACHE-EFFECTS DEMO  ").Bold().FG(RGB(255, 220, 120)),
		SpaceH(1),
		VBox.Border(BorderRounded).Title("system").PaddingTRBL(0, 1, 0, 1)(
			Text(&rows[0]), Text(&rows[1]), Text(&rows[2]), Text(&rows[3]),
		),
		Space(),
		Text(footer).FG(RGB(120, 200, 255)),
		// both effects animate over the SAME static dashboard, every frame
		// skipping Execute: pulse breathes the text warm, oscVignette breathes
		// the edges dark.
		ScreenEffect(pulse{}, oscVignette{}),
	}

	app.SetView(VBox.Grow(1).Fill(RGB(12, 14, 20))(children...)).
		Handle("q", func(_ riffkey.Match) { app.Stop() })

	// a slow heartbeat forces a real full render every 2s so the footer counter
	// refreshes — between beats the pulse animates via skipped frames.
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			app.RequestRender()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
