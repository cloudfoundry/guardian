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

func Command(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd {
	flags := []string{
		name,
		"-rootfsPath", rootfsPath,
		"-uid", strconv.Itoa(uid),
		"-gid", strconv.Itoa(gid),
		"-perm", strconv.Itoa(int(mode.Perm())),
	}

	if recreate {
		flags = append(flags, "-recreate=true")
	}

	return reexec.Command(append(flags, paths...)...)
}

func prepare() {
	var rootfsPath = flag.String("rootfsPath", "", "rootfs path to chroot into")
	var uid = flag.Int("uid", 0, "uid to create directories as")
	var gid = flag.Int("gid", 0, "gid to create directories as")
	var perm = flag.Int("perm", 0755, "Mode to create the directory with")
	var recreate = flag.Bool("recreate", false, "whether to delete the directory before (re-)creating it")

	flag.Parse()

	runtime.LockOSThread()
	if err := syscall.Chroot(*rootfsPath); err != nil {
		panic(err)
	}

	if err := os.Chdir("/"); err != nil {
		panic(err)
	}

	for _, path := range flag.Args() {
		path, err := filepath.Abs(path)
		if err != nil {
			panic(err)
		}

		if *recreate {
			rmdir(path)
		}

		mkdir(path, *uid, *gid, os.FileMode(*perm))
	}
}

func rmdir(path string) {
	if err := os.RemoveAll(path); err != nil {
		panic(err)
	}
}

func mkdir(path string, uid, gid int, mode os.FileMode) {
	if stat, err := os.Lstat(path); err == nil {
		if isSymlink(stat) {
			mkdir(mustReadSymlink(path), uid, gid, mode)
		}

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

func isSymlink(stat os.FileInfo) bool {
	return stat.Mode()&os.ModeSymlink == os.ModeSymlink
}

func mustReadSymlink(path string) string {
	path, err := os.Readlink(path)
	if err != nil {
		panic(err)
	}

	return path
}
