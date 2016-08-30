package dadoo

import (
	"os"
	"syscall"
	"unsafe"

	"code.cloudfoundry.org/garden"
)

func SetWinSize(f *os.File, ws garden.WindowSize) error {
	// set defaults
	if ws.Columns == 0 {
		ws.Columns = 80
	}
	if ws.Rows == 0 {
		ws.Rows = 24
	}

	_, _, e := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct {
			Rows uint16
			Cols uint16
		}{Rows: uint16(ws.Rows), Cols: uint16(ws.Columns)})),
		0, 0, 0,
	)

	if e != 0 {
		return syscall.ENOTTY
	}

	return nil
}
