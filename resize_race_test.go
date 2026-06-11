package glyph

import (
	"sync"
	"testing"
)

// pins the resize/render exclusion: applyResize swaps buffer storage under the
// render lock, so a render-locked Execute never indexes a torn buffer (the
// 'index out of range' FillRect crash from a terminal resize landing mid-render).
// Replacing applyResize with a bare pool.Resize reproduces the crash under -race.
func TestApplyResizeSerialisedAgainstRender(t *testing.T) {
	type Row struct{ Label string }
	rows := []Row{{"a"}, {"b"}, {"c"}}
	mode := "x"

	app := &App{pool: NewBufferPool(40, 12)}
	tmpl := Build(VBox(
		Match(&mode, Eq("x", VBox.Fill(Cyan)(
			ForEach(&rows, func(r *Row) Component {
				return HBox.Fill(Blue)(Text(&r.Label))
			}),
		))).Default(Text("")),
	))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			app.applyResize(40, 12+(i%40))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			app.renderMu.Lock()
			buf := app.pool.Current()
			tmpl.Execute(buf, int16(buf.Width()), int16(buf.Height()))
			app.renderMu.Unlock()
		}
	}()
	wg.Wait()
}
