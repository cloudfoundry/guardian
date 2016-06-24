package dadoo

import (
	"os"
	"syscall"
	"unsafe"
)

type TtySize struct {
	Rows   uint16
	Cols   uint16
	Xpixel uint16
	Ypixel uint16
}

func SetWinSize(f *os.File, ttySize *TtySize) error {
	_, _, e := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(ttySize)),
		0, 0, 0,
	)

	if e != 0 {
		return syscall.ENOTTY
	}

	return nil
}
