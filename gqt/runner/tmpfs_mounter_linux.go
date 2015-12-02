package runner

import (
	"os"
	"os/exec"
	"syscall"
)

func MustMountTmpfs(destination string) {
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		must(os.MkdirAll(destination, 0755))
		must(syscall.Mount("tmpfs", destination, "tmpfs", 0, ""))
	}
}

func MustUnmountTmpfs(destination string) {
	if out, err := exec.Command("umount", destination).CombinedOutput(); err != nil {
		panic(string(out))
	}

	must(os.Remove(destination))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
