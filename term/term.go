package term

import (
	"errors"
	"io"
	"os"
	"sync"

	glyph "github.com/kungfusheep/glyph"
)

// maxQueuedWriteBytes caps the bytes waiting for the writer goroutine. A peer that
// stops draining (a wedged socket to a long-lived host) leaves the writer parked
// inside rw.Write with nobody draining the queue; without a ceiling it grows for
// every keystroke. 1 MiB never triggers while the peer keeps up — the writer drains
// faster than a human types — and bounds the leak when it does not.
const maxQueuedWriteBytes = 1 << 20

// errWriteStalled is what Write returns once the queue overflowed because the far
// side stopped draining. A terminal that cannot deliver keys is broken whether the
// peer errored or hung; this makes the second case observable too.
var errWriteStalled = errors.New("term: write stream stalled, far side stopped draining")

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

	mu sync.Mutex
	// rw is the byte stream the terminal drives: the master side of a pty we
	// forked (New), or a stream handed to us by a caller whose process lives
	// elsewhere (Stream). pty is non-nil ONLY when we forked it, which is what
	// decides whether Close may kill anything.
	rw         io.ReadWriter
	pty        *pty
	onResize   func(rows, cols uint16)
	curW, curH int
	focused    bool
	closing    bool

	// writes are funnelled through one goroutine. The reader answers terminal
	// queries (cursor position, device attributes) by writing back, and a write
	// that blocks on the reader's own goroutine stops it draining the far side.
	// A pty master only blocks once the child stops reading, but a socket blocks
	// whenever the peer is slow, so the reader must never write directly.
	wmu     sync.Mutex
	wcond   *sync.Cond
	wq      [][]byte
	wqBytes int  // bytes currently queued, bounded by maxQueuedWriteBytes
	wclose  bool
	werr    error // the write error that shut the loop, nil if we closed it ourselves
}

// New creates a terminal component running $SHELL (or /bin/sh). The pty starts
// on the first frame, sized to the cell box the layout hands the component.
//
// New (not Term) is the constructor: the package name already carries the noun,
// so term.New() reads without stutter, the way list.New() does.
func New() *TermC {
	t := newTermC()
	t.shell = defaultShell()
	t.env = os.Environ()
	return t
}

// Stream drives the terminal from an existing byte stream instead of forking a
// shell. Output is read from rw and keys are written to it, so the process on the
// far side can live in another process entirely — behind a socket, say — and
// outlive the component.
//
// onResize is called with the new cell geometry whenever the layout box changes,
// in place of the TIOCSWINSZ the component issues when it owns the pty. It is how
// the far side learns its size.
//
// onResize FIRES ON THE RENDER GOROUTINE, inside Execute, and it must not block:
// enqueue the resize and return. A caller that writes it to a socket inline stalls
// the frame whenever the peer is slow, and a window drag calls this on consecutive
// frames, not once.
//
// The component does not own the far-side process, so Close closes the stream and
// leaves that process running. Use New for a terminal that forks and owns its own
// shell.
func Stream(rw io.ReadWriter, onResize func(rows, cols uint16)) *TermC {
	t := newTermC()
	t.rw = rw
	t.onResize = onResize
	return t
}

func newTermC() *TermC {
	t := &TermC{
		grow:  1, // terminals fill their box by default
		layer: glyph.NewLayer(),
	}
	t.wcond = sync.NewCond(&t.wmu)
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

// OnExit registers a callback fired when the FAR SIDE goes away: the shell exits,
// or the stream reaches EOF or fails to read. It carries that error.
//
// Close does NOT fire it. The distinction is the point: a host that closes a pane
// and a far side that dies both surface as a read error on the same goroutine, so
// without this a detach could not be told from a crash. OnExit means the process
// went away on its own; a silent teardown means you closed it.
//
// It fires on the reader goroutine, NOT the render goroutine. Do not touch bound
// state from it: the template reads bound state during Execute with no host lock,
// so a mutex on the write side cannot make that read safe. Marshal the change with
// App.Apply, which runs it at frame top before Execute. RequestRender is safe to
// call directly.
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

// Write sends raw bytes to the terminal as input (the far side's stdin). The
// bytes are queued for the writer goroutine, so this never blocks on a slow peer.
func (t *TermC) Write(p []byte) (int, error) {
	t.mu.Lock()
	rw := t.rw
	t.mu.Unlock()
	if rw == nil {
		return 0, io.ErrClosedPipe
	}
	if !t.enqueue(p) {
		t.wmu.Lock()
		err := t.werr
		t.wmu.Unlock()
		if err == nil {
			err = io.ErrClosedPipe // the write side was closed by us, not by a failure
		}
		return 0, err
	}
	return len(p), nil
}

// Close tears down the terminal.
//
// It kills the far-side process ONLY if this component forked it (New). A
// stream-backed terminal (Stream) does not own the process on the other end, so
// Close closes the stream and stops the reader, and that process keeps running —
// which is what lets a host detach from a long-lived agent instead of killing it.
func (t *TermC) Close() error {
	t.mu.Lock()
	if t.closing {
		t.mu.Unlock()
		return nil
	}
	t.closing = true
	p, rw := t.pty, t.rw
	t.mu.Unlock()

	t.wmu.Lock()
	t.wclose = true
	t.wcond.Broadcast()
	t.wmu.Unlock()

	if p != nil {
		return p.close() // we forked it, so we reap it
	}
	if c, ok := rw.(io.Closer); ok {
		return c.Close() // hand the stream back; the far side lives on
	}
	return nil
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

// startAt brings the terminal up at the given cell geometry: it forks a pty when
// it owns one, sizes the far side, and starts the reader and writer.
func (t *TermC) startAt(w, h int) {
	t.scr = newScreen(h, w)
	t.scr.onTitle = t.onTitle
	// Terminal queries are answered as terminal input: programs block on the
	// reply. The reply is queued, never written inline, so the reader's only job
	// stays reading.
	t.scr.onReply = func(b []byte) { t.enqueue(b) }

	t.mu.Lock()
	rw := t.rw
	t.mu.Unlock()

	if rw == nil { // New: we fork the shell and own it
		p, err := startPTY(t.shell, t.env, uint16(h), uint16(w))
		if err != nil {
			if t.onExit != nil {
				t.onExit(err)
			}
			return
		}
		t.mu.Lock()
		t.pty = p
		t.rw = p.master
		// TIOCSWINSZ on a pty we own: local, and it cannot meaningfully block.
		t.onResize = func(rows, cols uint16) { p.resize(rows, cols) }
		rw = p.master
		t.mu.Unlock()
	}

	t.mu.Lock()
	t.curW, t.curH = w, h
	onResize := t.onResize
	t.mu.Unlock()

	// a stream-backed terminal has to be told its size before the far side paints
	// its first frame; the forked pty was already opened at this geometry.
	if t.pty == nil && onResize != nil {
		onResize(uint16(h), uint16(w))
	}

	go t.writeLoop(rw)
	go t.readLoop(rw)
}

// enqueue hands bytes to the writer goroutine. It never blocks the caller, so it
// is safe from the reader goroutine (terminal query replies) and from the render
// goroutine. It returns false once the write side is gone, so the queue cannot
// grow without a writer to drain it and the caller can report the failure.
//
// Two things close the write side: the writer goroutine hitting a write error, and
// the queue overflowing maxQueuedWriteBytes because the far side stopped draining.
// The second is why the ceiling lives here rather than only on the error path — a
// wedged peer never errors, it just never returns from rw.Write.
func (t *TermC) enqueue(b []byte) bool {
	cp := make([]byte, len(b))
	copy(cp, b)
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if t.wclose {
		return false
	}
	if t.wqBytes+len(cp) > maxQueuedWriteBytes {
		t.wclose = true
		t.werr = errWriteStalled
		t.wq = nil
		t.wqBytes = 0
		t.wcond.Broadcast()
		return false
	}
	t.wq = append(t.wq, cp)
	t.wqBytes += len(cp)
	t.wcond.Signal()
	return true
}

// writeLoop is the only writer to the stream.
func (t *TermC) writeLoop(rw io.ReadWriter) {
	for {
		t.wmu.Lock()
		for len(t.wq) == 0 && !t.wclose {
			t.wcond.Wait()
		}
		if t.wclose {
			t.wmu.Unlock()
			return
		}
		batch := t.wq
		t.wq = nil
		t.wqBytes = 0
		t.wmu.Unlock()

		for _, b := range batch {
			if _, err := rw.Write(b); err != nil {
				// the write side is gone. On a half-open stream the reader can
				// still be blocked with nothing to report, so the death has to
				// be recorded here: close the queue so enqueue stops accepting
				// and Write returns the real error instead of a silent success.
				t.wmu.Lock()
				t.wclose = true
				t.werr = err
				t.wcond.Broadcast()
				t.wmu.Unlock()
				return
			}
		}
	}
}

// resizeIfNeeded reshapes the grid and tells the far side, when the viewport
// changes. This runs on the render goroutine, which is why onResize must not
// block (see Stream).
func (t *TermC) resizeIfNeeded(w, h int) {
	t.mu.Lock()
	changed := w != t.curW || h != t.curH
	onResize := t.onResize
	if changed {
		t.curW, t.curH = w, h
	}
	t.mu.Unlock()
	if !changed || onResize == nil {
		return
	}
	t.scr.resize(h, w)
	onResize(uint16(h), uint16(w))
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

// readLoop pumps the far side's output into the screen and requests repaints. It
// exits on EOF/error (the far side went away), firing OnExit.
func (t *TermC) readLoop(rw io.ReadWriter) {
	buf := make([]byte, 32*1024)
	for {
		n, err := rw.Read(buf)
		if n > 0 {
			t.scr.write(buf[:n])
			t.layer.Invalidate()
			if t.onUpdate != nil {
				t.onUpdate()
			}
		}
		if err != nil {
			// A teardown we performed ourselves arrives here as a read error too,
			// so it has to be told apart from the far side going away — otherwise a
			// host cannot distinguish a process that died from a pane it closed.
			// Close is silent; only the far side fires OnExit.
			t.mu.Lock()
			closing := t.closing
			t.mu.Unlock()
			if !closing && t.onExit != nil {
				t.onExit(err)
			}
			if t.onUpdate != nil {
				t.onUpdate()
			}
			return
		}
	}
}
