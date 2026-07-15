package term

import (
	"io"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	glyph "github.com/kungfusheep/glyph"
)

func bufContains(buf *glyph.Buffer, h int, sub string) bool {
	for y := 0; y < h; y++ {
		if strings.Contains(buf.GetLine(y), sub) {
			return true
		}
	}
	return false
}

func renderTerm(tc *TermC, w, h int) *glyph.Buffer {
	buf := glyph.NewBuffer(w, h)
	// a terminal fills its parent box; give it one (VBox) the way a real layout
	// does, so grow has a container to distribute
	glyph.Build(glyph.VBox(tc)).Execute(buf, int16(w), int16(h))
	return buf
}

// TestTermRendersShellOutput is the end-to-end proof: the component hosts a
// real shell, its output flows through the VT interpreter into the grid, and a
// render blits that grid into the buffer via the Layer path.
func TestTermRendersShellOutput(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 40, 10
	renderTerm(tc, w, h) // first render starts the pty at this geometry

	if _, err := tc.Write([]byte("echo HELLO_TERM\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bufContains(renderTerm(tc, w, h), h, "HELLO_TERM") {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("HELLO_TERM never appeared in rendered output")
}

// TestTermResizeFromGeometry proves the component sizes the pty from its
// allocated cell box: rendering into a different geometry reshapes the grid.
func TestTermResizeFromGeometry(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	renderTerm(tc, 20, 5)
	if tc.scr.rows != 5 || tc.scr.cols != 20 {
		t.Fatalf("initial grid %dx%d, want 20x5", tc.scr.cols, tc.scr.rows)
	}

	renderTerm(tc, 60, 15)
	if tc.scr.rows != 15 || tc.scr.cols != 60 {
		t.Fatalf("resized grid %dx%d, want 60x15", tc.scr.cols, tc.scr.rows)
	}
}

// TestTermCursorFollowsFocus proves the focused terminal shows the pty cursor
// on its layer and an unfocused one hides it.
func TestTermCursorFollowsFocus(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 20, 5
	renderTerm(tc, w, h)
	tc.scr.write([]byte("\x1b[2;3H")) // place cursor deterministically

	tc.Focus(true)
	renderTerm(tc, w, h)
	if cur := tc.layer.Cursor(); !cur.Visible {
		t.Fatal("focused terminal should show its cursor")
	}

	tc.Focus(false)
	renderTerm(tc, w, h)
	if cur := tc.layer.Cursor(); cur.Visible {
		t.Fatal("unfocused terminal should hide its cursor")
	}
}

// TestBlitDoesNotAllocatePerFrame pins the steady-state render path to zero
// allocations. A fresh Buffer per painted frame is garbage at pty output rate,
// which breaks glyph's zero-alloc-per-render contract.
func TestBlitDoesNotAllocatePerFrame(t *testing.T) {
	const w, h = 80, 24
	tc := blitFixture(w, h)
	tc.blitToLayer(w, h) // first frame legitimately allocates the buffer

	if got := testing.AllocsPerRun(50, func() { tc.blitToLayer(w, h) }); got != 0 {
		t.Fatalf("blitToLayer allocates %v times per frame, want 0", got)
	}
}

// TestBlitReallocatesOnResize proves the reuse is size-aware: a resized viewport
// must get a correctly sized buffer, not a stale one.
func TestBlitReallocatesOnResize(t *testing.T) {
	tc := blitFixture(80, 24)
	tc.blitToLayer(80, 24)
	tc.scr.resize(10, 40)
	tc.blitToLayer(40, 10)

	if tc.buf.Width() != 40 || tc.buf.Height() != 10 {
		t.Fatalf("buffer is %dx%d after resize, want 40x10", tc.buf.Width(), tc.buf.Height())
	}
}

// TestOnTitleNotCalledUnderScreenLock guards the lock order. The host's title
// callback runs on the pty goroutine; if it fires while write() holds the screen
// lock, a host that takes its own mutex there deadlocks against the render
// goroutine, which takes the locks the other way round in blitToLayer.
func TestOnTitleNotCalledUnderScreenLock(t *testing.T) {
	s := newScreen(24, 80)
	var held bool
	var got string
	s.onTitle = func(title string) {
		got = title
		if s.mu.TryLock() {
			s.mu.Unlock() // free to take it: the callback is outside the lock
		} else {
			held = true
		}
	}

	s.write([]byte("\x1b]0;my-title\x07"))

	if held {
		t.Fatal("onTitle fired while write() held the screen lock — a host mutex here deadlocks the renderer")
	}
	if got != "my-title" {
		t.Fatalf("onTitle got %q, want my-title", got)
	}
}

// waitFor polls cond every 20ms until it holds or the deadline passes. Real
// programs start, paint and exit on their own schedule; a fixed sleep either
// flakes or wastes the difference.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", d, what)
}

// TestFullScreenProgramInPane drives a REAL full-screen program through the pane,
// which is the only thing that proves the alternate screen: vi takes over, paints
// its own grid, and the shell's output is still underneath when it quits.
//
// The synthetic tests in vt_test.go pin the mechanism; this pins the outcome.
func TestFullScreenProgramInPane(t *testing.T) {
	if _, err := exec.LookPath("vi"); err != nil {
		t.Skip("vi not installed")
	}

	tc := New().Shell("/bin/sh").Env("PS1=$ ", "TERM=xterm", "LANG=C")
	tc.OnUpdate(func() {})
	defer tc.Close()

	const w, h = 40, 10
	renderTerm(tc, w, h) // first render starts the pty

	if _, err := tc.Write([]byte("echo MARKER_IN_SHELL\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	waitFor(t, 5*time.Second, "the shell's output", func() bool {
		return bufContains(renderTerm(tc, w, h), h, "MARKER_IN_SHELL")
	})

	// vi takes the screen: its grid is blank apart from what it paints, so the
	// shell's output must NOT be visible through it.
	tc.Write([]byte("vi\n"))
	waitFor(t, 5*time.Second, "vi to paint the alt screen", func() bool {
		return bufContains(renderTerm(tc, w, h), h, "~")
	})
	if b := renderTerm(tc, w, h); bufContains(b, h, "MARKER_IN_SHELL") {
		t.Error("the shell's output shows through the alt screen — the program is painting into the scrollback grid")
	}

	// and leaving restores what the shell had on screen
	tc.Write([]byte("\x1b:q\r"))
	waitFor(t, 5*time.Second, "the shell's output to come back", func() bool {
		return bufContains(renderTerm(tc, w, h), h, "MARKER_IN_SHELL")
	})
}

// TestStreamDoesNotOwnTheFarSide is the point of the Stream constructor: the
// component drives a process it did not fork, and closing the pane leaves that
// process running.
//
// The far side is modelled the way a host actually holds one: a real shell on a
// pty owned by someone else, reached over a socket. The terminal only ever sees
// the socket, so closing it cannot reach the process.
func TestStreamDoesNotOwnTheFarSide(t *testing.T) {
	p, err := startPTY("/bin/sh", []string{"PS1=", "TERM=dumb"}, 10, 40)
	if err != nil {
		t.Fatalf("start far side: %v", err)
	}
	defer p.close()
	pid := p.cmd.Process.Pid

	hostSide, uiSide := net.Pipe()
	go io.Copy(hostSide, p.master) // far side → socket
	go io.Copy(p.master, hostSide) // socket → far side

	var gotRows, gotCols uint16
	tc := Stream(uiSide, func(rows, cols uint16) { gotRows, gotCols = rows, cols })
	tc.OnUpdate(func() {})

	const w, h = 40, 10
	renderTerm(tc, w, h)

	// output flows in, keys flow out — over the injected stream, with no pty of ours
	tc.Write([]byte("echo STREAM_OK\n"))
	waitFor(t, 5*time.Second, "output over the injected stream", func() bool {
		return bufContains(renderTerm(tc, w, h), h, "STREAM_OK")
	})
	if gotRows != h || gotCols != w {
		t.Errorf("onResize got %dx%d, want %dx%d — the far side was never told its size", gotRows, gotCols, h, w)
	}

	// leaving the pane closes the stream...
	if err := tc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := uiSide.Write([]byte("x")); err == nil {
		t.Error("the stream is still open after Close — Close must hand the stream back")
	}

	// ...and does not reach the process behind it
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("far-side process %d died on Close (%v) — the component killed what it does not own", pid, err)
	}
}

// TestNewOwnsItsShell is the contrast: a terminal that forked its own shell still
// reaps it, so Close stays a full teardown for the case where the component IS the
// owner.
func TestNewOwnsItsShell(t *testing.T) {
	tc := New().Shell("/bin/sh").Env("PS1=", "TERM=dumb")
	tc.OnUpdate(func() {})
	renderTerm(tc, 40, 10)

	waitFor(t, 5*time.Second, "the shell to start", func() bool {
		tc.mu.Lock()
		defer tc.mu.Unlock()
		return tc.pty != nil
	})
	tc.mu.Lock()
	pid := tc.pty.cmd.Process.Pid
	tc.mu.Unlock()

	if err := tc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("shell %d still alive after Close — a terminal that forked its own shell must reap it", pid)
	}
}

// blockedStream is a stream whose Write never returns: a peer that has wedged.
// Reads block until the test closes done.
type blockedStream struct {
	done   chan struct{}
	writes chan int
}

func (b *blockedStream) Read(p []byte) (int, error) {
	<-b.done
	return 0, io.EOF
}

func (b *blockedStream) Write(p []byte) (int, error) {
	b.writes <- len(p)
	<-b.done // never returns while the peer is wedged
	return 0, io.EOF
}

// TestCloseIsSilentAndFarSideIsNot is the signal a host detaches on. Both paths
// end at a read error on the same goroutine, so they must be told apart: a pane
// the host closed says nothing, a far side that died reports it.
func TestCloseIsSilentAndFarSideIsNot(t *testing.T) {
	t.Run("close is silent", func(t *testing.T) {
		hostSide, uiSide := net.Pipe()
		defer hostSide.Close()

		exits := make(chan error, 1)
		tc := Stream(uiSide, func(rows, cols uint16) {})
		tc.OnUpdate(func() {})
		tc.OnExit(func(err error) { exits <- err })
		renderTerm(tc, 40, 10)

		tc.Close()

		select {
		case err := <-exits:
			t.Fatalf("OnExit fired with %v on a close we performed — a host cannot tell its own detach from a crash", err)
		case <-time.After(400 * time.Millisecond):
		}
	})

	t.Run("far side going away reports", func(t *testing.T) {
		hostSide, uiSide := net.Pipe()

		exits := make(chan error, 1)
		tc := Stream(uiSide, func(rows, cols uint16) {})
		tc.OnUpdate(func() {})
		tc.OnExit(func(err error) { exits <- err })
		defer tc.Close()
		renderTerm(tc, 40, 10)

		hostSide.Close() // the far side went away on its own

		select {
		case err := <-exits:
			if err == nil {
				t.Error("OnExit fired with a nil error; it must carry what happened")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the far side went away and OnExit never fired")
		}
	})
}

// TestRenderNeverBlocksOnConsumerIO: a wedged peer must not stall a frame. Writes
// are queued for the writer goroutine, so neither the render goroutine nor the
// reader ever waits on the far side.
func TestRenderNeverBlocksOnConsumerIO(t *testing.T) {
	bs := &blockedStream{done: make(chan struct{}), writes: make(chan int, 8)}
	defer close(bs.done)

	tc := Stream(bs, func(rows, cols uint16) {})
	tc.OnUpdate(func() {})
	renderTerm(tc, 40, 10)

	// a key: the writer goroutine takes it and wedges on the peer
	tc.Write([]byte("hello"))
	select {
	case <-bs.writes:
	case <-time.After(2 * time.Second):
		t.Fatal("the writer never reached the stream")
	}

	// with the writer wedged, frames must still complete — including one that
	// resizes, which is the path that calls onResize on the render goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		tc.Write([]byte("more"))
		renderTerm(tc, 40, 10)
		renderTerm(tc, 60, 20) // resize: exercises resizeIfNeeded
		renderTerm(tc, 40, 10)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a frame blocked on a wedged peer — consumer IO reached the render goroutine")
	}
}

// halfOpenStream is a peer that stopped reading without closing: writes fail,
// reads block. The read side staying open is the point — the reader never sees
// EOF, so the write side's death has to be reported on the write path itself.
type halfOpenStream struct {
	readGate chan struct{}
	writeErr error
}

func (h *halfOpenStream) Read(p []byte) (int, error) {
	<-h.readGate
	return 0, io.EOF
}

func (h *halfOpenStream) Write(p []byte) (int, error) { return 0, h.writeErr }

// TestWriteReportsDeadStream: when the writer goroutine dies on a write error,
// Write must report the failure and the queue must stop growing. A half-open peer
// (write fails, read blocks) is the case the reader cannot cover, so a naive
// "return, the reader reports it" leaves every keystroke lying about success and
// the queue accumulating forever.
func TestWriteReportsDeadStream(t *testing.T) {
	s := &halfOpenStream{readGate: make(chan struct{}), writeErr: syscall.EPIPE}
	defer close(s.readGate)

	tc := Stream(s, func(rows, cols uint16) {})
	tc.OnUpdate(func() {})
	renderTerm(tc, 40, 10)

	// the first key reaches the writer, which hits EPIPE and shuts the write side
	tc.Write([]byte("a"))

	// once the writer is dead, Write must stop returning success
	var lastErr error
	waitFor(t, 2*time.Second, "Write to report the dead stream", func() bool {
		_, lastErr = tc.Write([]byte("b"))
		return lastErr != nil
	})
	if lastErr != syscall.EPIPE {
		t.Errorf("Write returned %v, want the real write error EPIPE — the failure must not be masked", lastErr)
	}

	// and the queue must not have accumulated the rejected keys
	tc.wmu.Lock()
	n := len(tc.wq)
	tc.wmu.Unlock()
	if n > 1 {
		t.Errorf("write queue holds %d batches for a dead writer — enqueue kept appending with nobody draining", n)
	}
}

// TestWriteQueueIsBounded: a peer that WEDGES inside rw.Write (blocks, never
// errors) parks the writer goroutine there, so enqueue has no drainer. Without a
// ceiling the queue grows for as long as the human keeps typing at a pane that
// looks alive. The queue must cap and the stream must go degraded so Write can
// report it, the same observable failure as a dead stream.
func TestWriteQueueIsBounded(t *testing.T) {
	bs := &blockedStream{done: make(chan struct{}), writes: make(chan int, 8)}
	defer close(bs.done)

	tc := Stream(bs, func(rows, cols uint16) {})
	tc.OnUpdate(func() {})
	renderTerm(tc, 40, 10)

	// the first key wedges the writer inside rw.Write
	tc.Write([]byte("x"))
	select {
	case <-bs.writes:
	case <-time.After(2 * time.Second):
		t.Fatal("the writer never reached the stream")
	}

	// pump far more than any sane ceiling; with the writer stuck, this all queues
	chunk := make([]byte, 1024)
	var lastErr error
	for i := 0; i < 64*1024; i++ {
		if _, err := tc.Write(chunk); err != nil {
			lastErr = err
			break
		}
	}
	if lastErr == nil {
		t.Fatal("Write never reported the stalled stream — the queue grew without bound")
	}

	// on overflow the queue is dropped and the stream is degraded, not left holding
	tc.wmu.Lock()
	n := len(tc.wq)
	tc.wmu.Unlock()
	if n != 0 {
		t.Errorf("queue holds %d batches after degrade — overflow must drop them, not keep leaking", n)
	}
}
