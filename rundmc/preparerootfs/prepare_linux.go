package preparerootfs

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

const name = "prepare-rootfs"

func init() {
	reexec.Register(name, prepare)
}

func Command(spec specs.Spec, uid, gid int, mode os.FileMode, recreate bool, paths ...string) (*exec.Cmd, error) {
	rootfsMount, err := findRootfsMount(spec.Mounts)
	if err != nil {
		return nil, err
	}

	flags := []string{
		name,
		"-mountOptions", strings.Join(rootfsMount.Options, ","),
		"-uid", strconv.Itoa(uid),
		"-gid", strconv.Itoa(gid),
		"-perm", strconv.Itoa(int(mode.Perm())),
	}

	if recreate {
		flags = append(flags, "-recreate=true")
	}

	return reexec.Command(append(flags, paths...)...), nil
}

func findRootfsMount(mounts []specs.Mount) (specs.Mount, error) {
	for _, mount := range mounts {
		if mount.Destination == "/" {
			return mount, nil
		}
	}

	return specs.Mount{}, fmt.Errorf("no rootfs mount found in %v", mounts)
}

func prepare() {
	var mountOptions = flag.String("mountOptions", "", "rootfs mount options")
	var uid = flag.Int("uid", 0, "uid to create directories as")
	var gid = flag.Int("gid", 0, "gid to create directories as")
	var perm = flag.Int("perm", 0755, "Mode to create the directory with")
	var recreate = flag.Bool("recreate", false, "whether to delete the directory before (re-)creating it")

	flag.Parse()

	runtime.LockOSThread()

	mountPoint, err := mountRootfs(*mountOptions)
	if err != nil {
		fail("mounting-rootfs", err)
	}
	defer unmountRootfs(mountPoint)

	if err := syscall.Chroot(mountPoint); err != nil {
		fail("chroot", err)
	}

	if err := os.Chdir("/"); err != nil {
		fail("chdir", err)
	}

	for _, path := range flag.Args() {
		path, err := filepath.Abs(path)
		if err != nil {
			fail("abs-path", err)
		}

		if *recreate {
			rmdir(path)
		}

		mkdir(path, *uid, *gid, os.FileMode(*perm))
	}
}

func fail(msg string, err error) {
	panic(fmt.Errorf("%s: %v\n", msg, err))
}

func mountRootfs(mountOptions string) (string, error) {
	mountPoint, err := ioutil.TempDir("", "roootfs-")
	if err != nil {
		return "", err
	}

	if err := unix.Mount("overlay", mountPoint, "overlay", 0, mountOptions); err != nil {
		_ = os.RemoveAll(mountPoint)
		return "", err
	}

	return mountPoint, nil
}

func unmountRootfs(mountPoint string) {
	if err := unix.Unmount(mountPoint, 0); err != nil {
		fmt.Fprintf(os.Stderr, "unmount-rootfs-failed: %v", err)
		return
	}

	if err := os.RemoveAll(mountPoint); err != nil {
		fmt.Fprintf(os.Stderr, "remove-mountpoin-failed: %v", err)
	}
}

func rmdir(path string) {
	if err := os.RemoveAll(path); err != nil {
		fail(fmt.Sprintf("rmdir: %s", path), err)
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
		fail(fmt.Sprintf("mkdir %q", path), err)
	}
	fmt.Fprintf(os.Stderr, "mkdired %q\n", path)

	if err := os.Chown(path, uid, gid); err != nil {
		fail(fmt.Sprintf("chown %q", path), err)
	}
	fmt.Fprintf(os.Stderr, "chowned %q\n", path)
}

func isSymlink(stat os.FileInfo) bool {
	return stat.Mode()&os.ModeSymlink == os.ModeSymlink
}

func mustReadSymlink(path string) string {
	path, err := os.Readlink(path)
	if err != nil {
		fail(fmt.Sprintf("read-symlink %q", path), err)
	}

	return path
}
