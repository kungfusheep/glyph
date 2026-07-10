package term

import (
	"io"
	"os"
	"sync"

	glyph "github.com/kungfusheep/glyph"
)

// TermC is an embeddable terminal: it runs a shell on a pty and renders the
// shell's screen as glyph cells. Drop it into any layout like any other
// Renderer — it grows to fill its cell box, sizes the pty from that box, and
// repaints when the shell produces output.
//
// Input is content-blind: feed key events with HandleKey (wire it to a router's
// HandleUnmatched) or raw bytes with Write. The component does not own focus —
// the host decides which pane's HandleKey is armed, so a tmux-style prefix key
// can sit above it on the router stack.
type TermC struct {
	shell   string
	env     []string
	grow    float32
	onExit  func(error)
	onTitle func(string)

	started  sync.Once
	scr      *screen
	pty      *pty
	onUpdate func() // request a repaint (wire to app.RequestRender)

	mu           sync.Mutex
	lastW, lastH int
	focused      bool
	exited       bool
}

// New creates a terminal component. It reads $SHELL (falling back to /bin/sh)
// unless overridden with Shell. The pty is not started until the first Render,
// when the component knows its cell geometry.
//
// New (not Term) is the constructor: the package name already carries the noun,
// so term.New() reads without stutter, the way list.New() does.
func New() *TermC {
	return &TermC{
		shell: defaultShell(),
		env:   os.Environ(),
	}
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

// Grow sets the flex grow factor so the terminal expands to fill its box.
func (t *TermC) Grow(g float32) *TermC { t.grow = g; return t }

// OnExit registers a callback fired when the shell process exits.
func (t *TermC) OnExit(fn func(error)) *TermC { t.onExit = fn; return t }

// OnTitle registers a callback fired when the shell sets the window title
// (OSC 0/2).
func (t *TermC) OnTitle(fn func(string)) *TermC { t.onTitle = fn; return t }

// OnUpdate wires the repaint request. Pass app.RequestRender so shell output
// triggers a frame. Without it the terminal only repaints when something else
// drives a frame.
func (t *TermC) OnUpdate(fn func()) *TermC { t.onUpdate = fn; return t }

// Focus controls whether the pty cursor is drawn. The host sets this on the
// active pane so only the focused terminal shows a cursor.
func (t *TermC) Focus(focused bool) *TermC {
	t.mu.Lock()
	t.focused = focused
	t.mu.Unlock()
	return t
}

// Ref calls f with this TermC and returns it for chaining.
func (t *TermC) Ref(f func(*TermC)) *TermC { f(t); return t }

// Write sends raw bytes to the pty (the shell's stdin). Safe once started.
func (t *TermC) Write(p []byte) (int, error) {
	t.mu.Lock()
	p2 := t.pty
	t.mu.Unlock()
	if p2 == nil {
		return 0, io.ErrClosedPipe
	}
	return p2.master.Write(p)
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

// --- Renderer interface ---

// Build implements glyph.Component.
func (t *TermC) Build() glyph.Component { return t }

// MinSize implements glyph.Renderer. A terminal needs at least one cell; it
// grows to fill whatever the layout allocates.
func (t *TermC) MinSize() (int, int) { return 1, 1 }

// Render implements glyph.Renderer. It lazily starts the pty at the first known
// geometry, resizes on change, then blits the shell's screen grid into buf.
func (t *TermC) Render(buf *glyph.Buffer, x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	t.started.Do(func() { t.startAt(w, h) })
	t.resizeTo(w, h)

	scr := t.scr
	if scr == nil {
		return
	}
	scr.mu.Lock()
	rows, cols := scr.rows, scr.cols
	for row := 0; row < h && row < rows; row++ {
		for col := 0; col < w && col < cols; col++ {
			buf.Set(x+col, y+row, *scr.cellAt(col, row))
		}
	}
	cx, cy, vis := scr.cx, scr.cy, scr.cursorVisible
	scr.mu.Unlock()

	t.mu.Lock()
	focused := t.focused
	t.mu.Unlock()
	if vis && focused && cx < w && cy < h {
		// draw the cursor as an inverse cell — self-contained, no hardware
		// cursor plumbing through the custom-renderer boundary
		c := buf.Get(x+cx, y+cy)
		c.Style.Attr = c.Style.Attr.With(glyph.AttrInverse)
		buf.Set(x+cx, y+cy, c)
	}
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
		t.exited = true
		return
	}
	t.mu.Lock()
	t.pty = p
	t.lastW, t.lastH = w, h
	t.mu.Unlock()

	go t.readLoop(p)
}

// resizeTo reshapes the grid and pty when the allocated geometry changes.
func (t *TermC) resizeTo(w, h int) {
	t.mu.Lock()
	changed := w != t.lastW || h != t.lastH
	p := t.pty
	if changed {
		t.lastW, t.lastH = w, h
	}
	t.mu.Unlock()
	if !changed || p == nil {
		return
	}
	t.scr.resize(h, w)
	p.resize(uint16(h), uint16(w))
}

// readLoop pumps pty output into the screen and requests repaints. It exits on
// EOF/error (the shell closed), firing OnExit.
func (t *TermC) readLoop(p *pty) {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.master.Read(buf)
		if n > 0 {
			t.scr.write(buf[:n])
			if t.onUpdate != nil {
				t.onUpdate()
			}
		}
		if err != nil {
			t.mu.Lock()
			t.exited = true
			t.mu.Unlock()
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
