// +build linux

package unpacker

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

const utimeOmit int64 = ((1 << 30) - 2)
const atSymlinkNoFollow int = 0x100

func changeModTime(path string, modTime time.Time) error {
	var _path *byte
	_path, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}

	ts := []syscall.Timespec{
		syscall.Timespec{Sec: 0, Nsec: utimeOmit},
		syscall.NsecToTimespec(modTime.UnixNano()),
	}

	atFdCwd := -100
	_, _, errno := syscall.Syscall6(
		syscall.SYS_UTIMENSAT,
		uintptr(atFdCwd),
		uintptr(unsafe.Pointer(_path)),
		uintptr(unsafe.Pointer(&ts[0])),
		uintptr(atSymlinkNoFollow),
		0, 0,
	)
	if errno == syscall.ENOSYS {
		return os.Chtimes(path, time.Now(), modTime)
	}

	if errno != 0 {
		return errno
	}

	return nil
}
