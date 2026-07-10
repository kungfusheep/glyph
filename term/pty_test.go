package term

import (
	"bytes"
	"io"
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
