package preparerootfs

import (
	"os"
	"os/exec"
)

const name = "prepare-rootfs"

func init() {
}

func Command(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd {
	return &exec.Cmd{}
}

func prepare() {
}

func rmdir(path string) {
}

func mkdir(path string, uid, gid int, mode os.FileMode) {
}

func isSymlink(stat os.FileInfo) bool {
	return false
}

func mustReadSymlink(path string) string {
	return ""
}
