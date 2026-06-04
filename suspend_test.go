package glyph

import "testing"

// Suspend gates rendering (so handing the terminal to $EDITOR draws nothing over it),
// and Resume re-enables it and forces a full repaint (the external program scribbled
// over the screen, so a diff against the stale front buffer wouldn't be enough).
func TestSuspendResume(t *testing.T) {
	a := &App{}
	if a.suspended.Load() {
		t.Fatal("a fresh app should not be suspended")
	}

	a.Suspend()
	if !a.suspended.Load() {
		t.Fatal("Suspend should gate rendering")
	}

	a.forceFullFlush = false
	a.Resume()
	if a.suspended.Load() {
		t.Fatal("Resume should re-enable rendering")
	}
	if !a.forceFullFlush {
		t.Fatal("Resume should force a full repaint")
	}
}
