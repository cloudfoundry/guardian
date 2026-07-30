package testhelpers

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// see https://github.com/karelzak/util-linux/blob/422f0e9f206a145c59a71333dad20d38cbbfc0c4/include/loopdev.h#L53-L67
type LoopInfo64 struct {
	Device         uint64
	Inode          uint64
	Rdevice        uint64
	Offset         uint64
	SizeLimit      uint64
	Number         uint32
	EncryptType    uint32
	EncryptKeySize uint32
	Flags          uint32
	FileName       [64]uint8
	CryptName      [64]uint8
	EncryptKey     [32]uint8
	Init           [2]uint64
}

func IsDirectIOEnabled(loopDevPath string) (bool, error) {
	loopFd, err := os.Open(loopDevPath)
	if err != nil {
		return false, err
	}
	defer loopFd.Close()

	// see https://github.com/karelzak/util-linux/blob/422f0e9f206a145c59a71333dad20d38cbbfc0c4/include/loopdev.h#L23
	const LOOP_GET_STATUS64 = 0x4C05

	var loopInfo LoopInfo64
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, loopFd.Fd(), LOOP_GET_STATUS64, uintptr(unsafe.Pointer(&loopInfo)), 0, 0, 0)
	if errno != 0 {
		return false, fmt.Errorf("getting loop status failed: %s", errno.Error())
	}

	// see https://github.com/karelzak/util-linux/blob/422f0e9f206a145c59a71333dad20d38cbbfc0c4/include/loopdev.h#L44
	const LO_FLAGS_DIRECT_IO = 16
	return loopInfo.Flags&LO_FLAGS_DIRECT_IO != 0, nil
}
