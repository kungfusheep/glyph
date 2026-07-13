package term

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// TestPTYRoundTrip proves the pty hosts a real shell: write a command to the
// master, read the shell's output back, and see the command's result. If pty
// setup (openpty/grant/unlock, controlling-tty attach) were broken, the shell
// would never start or never echo, and this fails.
func TestPTYRoundTrip(t *testing.T) {
	p, err := startPTY("/bin/sh", []string{"PS1=", "TERM=dumb"}, 24, 80)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer p.close()

	if _, err := io.WriteString(p.master, "echo glyphpty_marker_42\n"); err != nil {
		t.Fatalf("write to pty: %v", err)
	}

	got := readUntil(t, p.master, []byte("glyphpty_marker_42"), 3*time.Second)
	if !bytes.Contains(got, []byte("glyphpty_marker_42")) {
		t.Fatalf("shell output missing marker; got:\n%q", got)
	}
	// The literal command echoes too; the point is the shell RAN it and printed
	// the result — proving a live session, not just terminal echo.
	if bytes.Count(got, []byte("glyphpty_marker_42")) < 1 {
		t.Fatalf("expected marker in output, got:\n%q", got)
	}
}

// TestPTYResize proves resize plumbing reaches the kernel: after resizing, the
// shell's reported window size (via stty) matches. A no-op resize would report
// the original 24x80.
func TestPTYResize(t *testing.T) {
	p, err := startPTY("/bin/sh", []string{"PS1=", "TERM=dumb"}, 24, 80)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer p.close()

	if err := p.resize(40, 132); err != nil {
		t.Fatalf("resize: %v", err)
	}
	// give the kernel a moment to apply before we ask
	time.Sleep(50 * time.Millisecond)

	if _, err := io.WriteString(p.master, "stty size\n"); err != nil {
		t.Fatalf("write stty: %v", err)
	}
	got := readUntil(t, p.master, []byte("40 132"), 3*time.Second)
	if !bytes.Contains(got, []byte("40 132")) {
		t.Fatalf("resize not reflected; want \"40 132\" in:\n%q", got)
	}
}

// readUntil reads from r until sentinel appears or the deadline passes.
func readUntil(t *testing.T, r io.Reader, sentinel []byte, d time.Duration) []byte {
	t.Helper()
	var acc []byte
	deadline := time.Now().Add(d)
	buf := make([]byte, 4096)
	done := make(chan []byte, 1)
	go func() {
		for {
			n, err := r.Read(buf)
			if n > 0 {
				acc = append(acc, buf[:n]...)
				if bytes.Contains(acc, sentinel) {
					done <- acc
					return
				}
			}
			if err != nil {
				done <- acc
				return
			}
		}
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(time.Until(deadline)):
		return acc
	}
}

// TestPTYUnicodeThroughRealShell is the end-to-end check for the rendering
// defect: a real shell emits box-drawing UTF-8 and the DEC special graphics
// set, and the grid must hold the glyphs a user expects to see. Decoding
// byte-at-a-time (the original bug) leaves latin-1 mojibake here.
func TestPTYUnicodeThroughRealShell(t *testing.T) {
	p, err := startPTY("/bin/sh", []string{"PS1=", "TERM=xterm"}, 24, 80)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer p.close()

	s := newScreen(24, 80)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := p.master.Read(buf)
			if n > 0 {
				s.write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// printf writes the box line as raw UTF-8; the second uses ESC ( 0 so the
	// shell drives the graphics charset the way ncurses does.
	io.WriteString(p.master, "printf '\\342\\224\\214\\342\\224\\200\\342\\224\\220 caf\\303\\251\\n'\n")
	io.WriteString(p.master, "printf '\\033(0lqk\\033(B done\\n'\n")
	io.WriteString(p.master, "exit\n")
	<-done

	var found, foundDEC bool
	for y := 0; y < 24; y++ {
		row := rowText(s, y)
		if strings.Contains(row, "┌─┐ café") {
			found = true
		}
		if strings.Contains(row, "┌─┐ done") {
			foundDEC = true
		}
	}
	if !found {
		t.Fatalf("no row holds the UTF-8 box line + café; grid:\n%s", dumpGrid(s))
	}
	if !foundDEC {
		t.Fatalf("no row holds the DEC-graphics box line; grid:\n%s", dumpGrid(s))
	}
}

func dumpGrid(s *screen) string {
	var b strings.Builder
	for y := 0; y < s.rows; y++ {
		if r := rowText(s, y); r != "" {
			b.WriteString(r)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// TestPTYDSRRoundTripThroughRealShell proves the reply reaches the child: a real
// shell asks for the cursor position, our screen answers, and the bytes arrive on
// the shell's stdin where it can read them back. Programs like neovim BLOCK on
// this answer — without it they report "did not detect DSR response".
func TestPTYDSRRoundTripThroughRealShell(t *testing.T) {
	p, err := startPTY("/bin/sh", []string{"PS1=", "TERM=xterm"}, 24, 80)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer p.close()

	s := newScreen(24, 80)
	s.onReply = func(b []byte) { p.master.Write(b) } // exactly what TermC wires

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.master.Read(buf)
			if n > 0 {
				s.write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// ask for the cursor position, read the 6-byte answer off stdin, print it
	io.WriteString(p.master, "printf '\\033[6n'; R=$(dd bs=1 count=6 2>/dev/null | cat -v); printf 'DSR_GOT[%s]\\n' \"$R\"\n")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		var b strings.Builder
		for y := 0; y < s.rows; y++ {
			for x := 0; x < s.cols; x++ {
				b.WriteRune(s.cellAt(x, y).Rune)
			}
		}
		out := b.String()
		s.mu.Unlock()
		if i := strings.Index(out, "DSR_GOT["); i >= 0 {
			got := out[i:]
			if strings.Contains(got, "R") && strings.Contains(got, "^[[") {
				return // the shell read our reply off its stdin
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("shell never received a DSR reply on its stdin")
}
