package preparerootfs

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
)

const name = "prepare-rootfs"

func init() {
	reexec.Register(name, prepare)
}

func Command(rootfsPath string, uid, gid int, mode os.FileMode, paths ...string) *exec.Cmd {
	flags := []string{
		name,
		"-rootfsPath", rootfsPath,
		"-uid", strconv.Itoa(uid),
		"-gid", strconv.Itoa(gid),
		"-perm", strconv.Itoa(int(mode.Perm())),
	}

	return reexec.Command(append(flags, paths...)...)
}

func prepare() {
	var rootfsPath = flag.String("rootfsPath", "", "rootfs path to chroot into")
	var uid = flag.Int("uid", 0, "uid to create directories as")
	var gid = flag.Int("gid", 0, "gid to create directories as")
	// Default permission: 493 decimal is 0755 octal
	var perm = flag.Int("perm", 493, "Mode to create the directory with")

	flag.Parse()

	runtime.LockOSThread()
	if err := syscall.Chroot(*rootfsPath); err != nil {
		panic(err)
	}

	if err := os.Chdir("/"); err != nil {
		panic(err)
	}

	for _, path := range flag.Args() {
		rmdir(path)
		mkdir(path, *uid, *gid, os.FileMode(*perm))
	}
}

func rmdir(path string) {
	if err := os.RemoveAll(path); err != nil {
		panic(err)
	}
}

func mkdir(path string, uid, gid int, mode os.FileMode) {
	if _, err := os.Stat(path); err == nil {
		return
	}

	mkdir(filepath.Dir(path), uid, gid, mode)

	if err := os.Mkdir(path, mode); err != nil {
		panic(err)
	}

	if err := os.Chown(path, uid, gid); err != nil {
		panic(err)
	}
}
