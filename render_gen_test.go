package glyph

import "testing"

// pins the request/done generation protocol behind the debounce skip: a render
// request stamped after a completed render must never read as covered, even
// though an input-driven flush happened in between (the "preview one step
// behind" race).
func TestRenderGenerationsCoverAsyncRequestAfterFlush(t *testing.T) {
	app := &App{}

	app.RequestRender()
	app.render()
	if app.reqGen.Load() != app.doneGen.Load() {
		t.Fatalf("echo request should read covered after render: req=%d done=%d",
			app.reqGen.Load(), app.doneGen.Load())
	}

	app.render() // input-driven flush of old state
	app.RequestRender() // async publish lands after the flush
	if app.reqGen.Load() == app.doneGen.Load() {
		t.Fatal("request stamped after the flush reads as covered; debounce would discard it")
	}

	app.render()
	if app.reqGen.Load() != app.doneGen.Load() {
		t.Fatalf("follow-up render should cover the request: req=%d done=%d",
			app.reqGen.Load(), app.doneGen.Load())
	}
}
