package glyph

import "testing"

type noopEffect struct{}

func (noopEffect) Apply(*Buffer, PostContext) {}

// A SIGWINCH updates the screen size on the signal goroutine; the pool only
// grows later in applyResize, behind renderMu. A frame that starts between the
// two must render at the buffer's size, not the screen's, or the effects pass
// indexes off the end of the old buffer (the resize crash seen in mail).
func TestFrameRendersAtBufferSizeWhenScreenGrewFirst(t *testing.T) {
	app := NewApp()
	app.SetView(VBox(
		ScreenEffect(noopEffect{}),
		Text("resize me"),
	))
	app.pool = NewBufferPool(40, 12)
	// the signal landed (handleSignals resizes the screen under its lock);
	// applyResize, which grows the pool, has not run yet
	app.screen.width, app.screen.height = 85, 52
	app.screen.front.Resize(85, 52)
	app.screen.back.Resize(85, 52)

	app.render() // must not panic

	if w, h := app.cleanBuf.Width(), app.cleanBuf.Height(); w != 40 || h != 12 {
		t.Fatalf("frame rendered at %dx%d, want the buffer's 40x12", w, h)
	}
}
