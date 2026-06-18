package glyph

import (
	"testing"
	"time"
)

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

// pins the adaptive render pacing (m793): when the terminal drains slowly, the
// coalescing window widens toward the drain rate (clamped) so we never enqueue
// frames faster than the screen can absorb them — the cause of the held-key
// tearing where a frame took seconds to appear.
func TestPacingIntervalAdaptsToWriteDuration(t *testing.T) {
	app := &App{}

	// terminal keeping up (no recorded write / fast write) → resting window.
	if got := app.pacingInterval(); got != baseDebounce {
		t.Fatalf("idle terminal should use base debounce: got %v want %v", got, baseDebounce)
	}
	app.lastWriteNs.Store(int64(2 * time.Millisecond))
	if got := app.pacingInterval(); got != baseDebounce {
		t.Fatalf("fast write should clamp up to base debounce: got %v want %v", got, baseDebounce)
	}

	// terminal behind by a measurable amount → window tracks the drain rate.
	app.lastWriteNs.Store(int64(40 * time.Millisecond))
	if got := app.pacingInterval(); got != 40*time.Millisecond {
		t.Fatalf("slow write should widen the window to drain rate: got %v want 40ms", got)
	}

	// pathologically slow write → clamped so rendering can't stall forever.
	app.lastWriteNs.Store(int64(500 * time.Millisecond))
	if got := app.pacingInterval(); got != maxDebounce {
		t.Fatalf("very slow write should clamp to max debounce: got %v want %v", got, maxDebounce)
	}
}

// the input callback coalesces instead of forcing a synchronous render once the
// terminal falls behind: the behindThreshold boundary is what flips per-key
// synchronous flushes into debounced drop-to-latest.
func TestBehindThresholdBoundary(t *testing.T) {
	app := &App{}

	// at/under threshold the terminal is considered caught up.
	app.lastWriteNs.Store(int64(behindThreshold))
	if time.Duration(app.lastWriteNs.Load()) > behindThreshold {
		t.Fatal("write equal to threshold must not count as behind")
	}
	// just over → behind, input render should defer to the debounced goroutine.
	app.lastWriteNs.Store(int64(behindThreshold + time.Millisecond))
	if time.Duration(app.lastWriteNs.Load()) <= behindThreshold {
		t.Fatal("write past threshold must count as behind")
	}
}
