//go:build darwin

package term

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openPTY opens a Darwin pseudo-terminal and returns the master file plus the
// slave device path. Darwin grants and unlocks the slave through ioctls on the
// master (no grantpt(3)/ptsname(3) libc calls), then names it with TIOCPTYGNAME.
func openPTY() (master *os.File, slavePath string, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, "", err
	}
	fd := int(m.Fd())

	if err := ioctlNoArg(fd, unix.TIOCPTYGRANT); err != nil {
		m.Close()
		return nil, "", err
	}
	if err := ioctlNoArg(fd, unix.TIOCPTYUNLK); err != nil {
		m.Close()
		return nil, "", err
	}

	// TIOCPTYGNAME writes the slave device path into a caller-supplied buffer.
	var buf [128]byte
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&buf[0]))); e != 0 {
		m.Close()
		return nil, "", e
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return m, string(buf[:n]), nil
}

// ioctlNoArg issues an ioctl that takes no argument (the _IO grant/unlock
// requests), surfacing a non-zero errno as an error.
func ioctlNoArg(fd int, req uint) error {
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), 0); e != 0 {
		return e
	}
	return nil
}
