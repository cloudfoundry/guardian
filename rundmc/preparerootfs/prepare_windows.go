package preparerootfs

import (
	"os"
	"os/exec"
)

func Command(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd {
	return &exec.Cmd{}
}
