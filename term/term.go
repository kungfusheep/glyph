package term

import (
	"io"
	"os"
	"sync"

	glyph "github.com/kungfusheep/glyph"
)

// TermC is an embeddable terminal: it runs a shell on a pty and renders the
// shell's screen as glyph cells. Drop it into any layout like any other
// component — it is Layer-backed (like Log and TextView), so it flex-grows to
// fill its box, and it drives the pty size from the box it is given.
//
// Input is content-blind: feed key events with HandleKey (wire it to a router's
// HandleUnmatched) or raw bytes with Write. The component does not own focus —
// the host decides which pane's HandleKey is armed and which is Focus(true), so
// a tmux-style prefix key can sit above it on the router stack.
type TermC struct {
	shell    string
	env      []string
	grow     float32
	onExit   func(error)
	onTitle  func(string)
	onUpdate func() // repaint request (wire to app.RequestRender)

	layer   *glyph.Layer
	scr     *screen
	started sync.Once

	// the blit target, reused across frames — a fresh buffer per painted frame
	// is garbage at pty output rate. Reallocated only when the viewport resizes.
	buf *glyph.Buffer

	mu         sync.Mutex
	pty        *pty
	curW, curH int
	focused    bool
}

// New creates a terminal component running $SHELL (or /bin/sh). The pty starts
// on the first frame, sized to the cell box the layout hands the component.
//
// New (not Term) is the constructor: the package name already carries the noun,
// so term.New() reads without stutter, the way list.New() does.
func New() *TermC {
	t := &TermC{
		shell: defaultShell(),
		env:   os.Environ(),
		grow:  1, // terminals fill their box by default
		layer: glyph.NewLayer(),
	}
	// syncFrame runs every frame the layer needs rendering (output arrived or
	// the viewport changed). The framework has already set the viewport, so it
	// is the natural place to size the pty and blit the grid.
	t.layer.Render = t.syncFrame
	return t
}

func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// Shell sets the program to run on the pty. Default is $SHELL or /bin/sh.
func (t *TermC) Shell(path string) *TermC { t.shell = path; return t }

// Env sets the child environment. Default is the parent's os.Environ().
func (t *TermC) Env(env ...string) *TermC { t.env = env; return t }

// Grow sets the flex grow factor. Default is 1 (fill the box). Set 0 for a
// fixed-size terminal sized by an explicit Height/Width on the layout.
func (t *TermC) Grow(g float32) *TermC { t.grow = g; return t }

// OnExit registers a callback fired when the shell process exits.
//
// It fires on the pty reader goroutine, NOT the render goroutine. Do not touch
// bound state from it: the template reads bound state during Execute with no
// host lock, so a mutex on the write side cannot make that read safe. Marshal
// the change with App.Apply, which runs it at frame top before Execute.
// RequestRender is safe to call directly.
func (t *TermC) OnExit(fn func(error)) *TermC { t.onExit = fn; return t }

// OnTitle registers a callback fired when the shell sets the window title
// (OSC 0/2). It fires on the pty reader goroutine; see OnExit for the rule on
// touching bound state.
func (t *TermC) OnTitle(fn func(string)) *TermC { t.onTitle = fn; return t }

// OnUpdate wires the repaint request. Pass app.RequestRender so shell output
// triggers a frame. It fires on the pty reader goroutine; see OnExit for the
// rule on touching bound state.
func (t *TermC) OnUpdate(fn func()) *TermC { t.onUpdate = fn; return t }

// Focus controls whether this terminal draws the cursor. The host sets it on
// the active pane so only the focused terminal shows a cursor.
func (t *TermC) Focus(focused bool) *TermC {
	t.mu.Lock()
	t.focused = focused
	t.mu.Unlock()
	t.layer.Invalidate() // redraw so the cursor appears/disappears
	return t
}

// Ref calls f with this TermC and returns it for chaining.
func (t *TermC) Ref(f func(*TermC)) *TermC { f(t); return t }

// Write sends raw bytes to the pty (the shell's stdin).
func (t *TermC) Write(p []byte) (int, error) {
	t.mu.Lock()
	pty := t.pty
	t.mu.Unlock()
	if pty == nil {
		return 0, io.ErrClosedPipe
	}
	return pty.master.Write(p)
}

// Close tears down the pty and reaps the shell.
func (t *TermC) Close() error {
	t.mu.Lock()
	p := t.pty
	t.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.close()
}

// Build implements glyph.Component: the terminal renders through a Layer, so it
// composes as a LayerView and inherits its flex-grow and viewport handling.
func (t *TermC) Build() glyph.Component {
	return glyph.LayerView(t.layer).Grow(t.grow)
}

// syncFrame is the Layer's per-frame callback. The viewport is already set, so
// it lazily starts the pty at that size, resizes on change, and blits the grid.
func (t *TermC) syncFrame() {
	w, h := t.layer.ViewportWidth(), t.layer.ViewportHeight()
	if w <= 0 || h <= 0 {
		return
	}
	t.started.Do(func() { t.startAt(w, h) })
	if t.scr == nil {
		return // start failed
	}
	t.resizeIfNeeded(w, h)
	t.blitToLayer(w, h)
}

// startAt opens the pty at the given cell geometry and starts the reader.
func (t *TermC) startAt(w, h int) {
	t.scr = newScreen(h, w)
	t.scr.onTitle = t.onTitle

	p, err := startPTY(t.shell, t.env, uint16(h), uint16(w))
	if err != nil {
		if t.onExit != nil {
			t.onExit(err)
		}
		return
	}
	t.mu.Lock()
	t.pty = p
	t.curW, t.curH = w, h
	t.mu.Unlock()

	go t.readLoop(p)
}

// resizeIfNeeded reshapes the grid and pty when the viewport changes.
func (t *TermC) resizeIfNeeded(w, h int) {
	t.mu.Lock()
	changed := w != t.curW || h != t.curH
	p := t.pty
	if changed {
		t.curW, t.curH = w, h
	}
	t.mu.Unlock()
	if !changed || p == nil {
		return
	}
	t.scr.resize(h, w)
	p.resize(uint16(h), uint16(w))
}

// blitToLayer copies the grid into the layer's buffer, then mirrors the pty
// cursor when focused.
func (t *TermC) blitToLayer(w, h int) {
	if t.buf == nil || t.buf.Width() != w || t.buf.Height() != h {
		t.buf = glyph.NewBuffer(w, h)
	}
	buf := t.buf
	t.scr.mu.Lock()
	rows, cols := t.scr.rows, t.scr.cols
	if rows < h || cols < w {
		buf.Clear() // the grid does not cover the box; stale cells must not show
	}
	for y := 0; y < h && y < rows; y++ {
		for x := 0; x < w && x < cols; x++ {
			buf.Set(x, y, *t.scr.cellAt(x, y))
		}
	}
	cx, cy, vis := t.scr.cx, t.scr.cy, t.scr.cursorVisible
	t.scr.mu.Unlock()

	t.layer.SetBuffer(buf)

	t.mu.Lock()
	focused := t.focused
	t.mu.Unlock()
	if vis && focused && cx < w && cy < h {
		t.layer.SetCursor(cx, cy)
		t.layer.ShowCursor()
	} else {
		t.layer.HideCursor()
	}
}

// readLoop pumps pty output into the screen and requests repaints. It exits on
// EOF/error (the shell closed), firing OnExit.
func (t *TermC) readLoop(p *pty) {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.master.Read(buf)
		if n > 0 {
			t.scr.write(buf[:n])
			t.layer.Invalidate()
			if t.onUpdate != nil {
				t.onUpdate()
			}
		}
		if err != nil {
			if t.onExit != nil {
				t.onExit(err)
			}
			if t.onUpdate != nil {
				t.onUpdate()
			}
			return
		}
	}
}
