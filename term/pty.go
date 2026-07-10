// Package term hosts a pseudo-terminal inside a glyph layout: a Term component
// runs a shell on a pty, interprets its VT output into an offscreen cell grid,
// and renders the viewport like any other glyph component. It is content-blind
// to what the shell does — bytes in, bytes out.
package term

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// pty is a hosted pseudo-terminal: the master side of a pty pair plus the child
// process running on the slave side. Bytes written to master reach the child's
// stdin; the child's stdout/stderr arrive by reading master.
type pty struct {
	master *os.File
	cmd    *exec.Cmd
}

// startPTY opens a pty, sizes it, and starts shell as a session leader with the
// slave as its controlling terminal. The parent keeps master and closes slave —
// the child owns the slave via its stdio.
func startPTY(shell string, env []string, rows, cols uint16) (*pty, error) {
	master, slavePath, err := openPTY()
	if err != nil {
		return nil, err
	}

	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer slave.Close() // the child inherits it via stdio; the parent doesn't need it

	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	setWinsize(master, rows, cols)

	cmd := exec.Command(shell)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	// Setsid makes the child a new session leader; Setctty attaches its
	// controlling terminal. Ctty is read in the child's fd space after stdio is
	// dup'd into place, so slave lands on fd 0 — the default Ctty.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	if err := cmd.Start(); err != nil {
		master.Close()
		return nil, err
	}
	return &pty{master: master, cmd: cmd}, nil
}

// resize sets the pty window size. The kernel delivers SIGWINCH to the child,
// which is how a full-screen program learns its new geometry.
func (p *pty) resize(rows, cols uint16) error {
	return setWinsize(p.master, rows, cols)
}

// close tears down the pty: closing master sends EOF to the child's read side,
// then we reap the process so it doesn't linger as a zombie.
func (p *pty) close() error {
	err := p.master.Close()
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
	return err
}

func setWinsize(f *os.File, rows, cols uint16) error {
	return unix.IoctlSetWinsize(int(f.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: rows,
		Col: cols,
	})
}
