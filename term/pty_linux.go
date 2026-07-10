//go:build linux

package term

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openPTY opens a Linux pseudo-terminal and returns the master file plus the
// slave device path. Linux unlocks the slave with TIOCSPTLCK and names it by
// its number from TIOCGPTN, which maps to /dev/pts/N.
func openPTY() (master *os.File, slavePath string, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, "", err
	}
	fd := int(m.Fd())

	// TIOCSPTLCK with a zero value unlocks the slave for opening.
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		m.Close()
		return nil, "", err
	}

	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		m.Close()
		return nil, "", err
	}
	return m, fmt.Sprintf("/dev/pts/%d", n), nil
}
