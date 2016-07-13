package dadoo

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/cloudfoundry-incubator/garden"
)

func SetWinSize(f *os.File, ws garden.WindowSize) error {
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
