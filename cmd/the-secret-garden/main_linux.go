package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Register("namespaced", namespaced)

	if reexec.Init() {
		os.Exit(0)
	}
}

func namespaced() {
	dataDir := os.Args[1]

	cmd := exec.Command(os.Args[4], os.Args[5:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	mustRun(exec.Command("mount", "--make-slave", dataDir))

	if err := cmd.Run(); err != nil {
		fmt.Printf("%s: %s\n", cmd.Path, err)
		os.Exit(1)
	}
}

func main() {
	dataDir := os.Args[1]
	realGraphDir := os.Args[2]
	graphDir := os.Args[3]

	mustBindMountOnce(dataDir, dataDir)
	mustRun(exec.Command("mount", "--make-shared", dataDir))

	mustBindMountOnce(realGraphDir, graphDir)

	reexecInNamespace(os.Args[1:]...)
}

func reexecInNamespace(args ...string) {
	reexecArgs := append([]string{"namespaced"}, args...)
	cmd := reexec.Command(reexecArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("exec secret garden: %s\n", err)
		os.Exit(1)
	}
}

func mustBindMountOnce(srcDir, dstDir string) {
	mounts := mustRun(exec.Command("mount"))
	alreadyMounted := strings.Contains(mounts, fmt.Sprintf("%s on %s", srcDir, dstDir))

	if !alreadyMounted {
		mustRun(exec.Command("mount", "--bind", srcDir, dstDir))
	}
}

func run(cmd *exec.Cmd) error {
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %s", cmd.Path, err, string(out))
	}

	return nil
}

func mustRun(cmd *exec.Cmd) string {
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("%s: %s: %s", cmd.Path, err, string(out)))
	}
	return string(out)
}
