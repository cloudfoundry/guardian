package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	realGraphDir := os.Args[2]
	graphDir := filepath.Join(realGraphDir, "graph")
	secretGraphDir := os.Args[3]
	pidFile := os.Args[4]

	mustRun(exec.Command("mount", "--make-slave", dataDir))
	mustRun(exec.Command("chmod", "go-x", realGraphDir))

	mustBindMountOnce(graphDir, secretGraphDir)

	programPath, err := exec.LookPath(os.Args[5])
	if err != nil {
		fmt.Printf("failed to look path in namespace : %s\n", err)
		os.Exit(1)
	}

	pid := strconv.Itoa(os.Getpid())
	err = ioutil.WriteFile(pidFile, []byte(pid), 0644)
	if err != nil {
		fmt.Printf("failed writing pidfile: %s\n", err)
		os.Exit(1)
	}

	err = syscall.Exec(programPath, os.Args[5:], os.Environ())
	if err != nil {
		fmt.Printf("exec failed in namespace: %s\n", err)
		os.Exit(1)
	}
}

func main() {
	dataDir := os.Args[1]
	realGraphDir := os.Args[2]
	graphDir := filepath.Join(realGraphDir, "graph")

	mustRun(exec.Command("mkdir", "-p", graphDir))

	mustRBindMountOnce(dataDir, dataDir)
	mustRun(exec.Command("mount", "--make-shared", dataDir))

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
		fmt.Printf("secret garden exec failed: %s\n", err)
		os.Exit(1)
	}
}

func mustBindMountOnce(srcDir, dstDir string) {
	mustMountOnce(srcDir, dstDir, "--bind")
}

func mustRBindMountOnce(srcDir, dstDir string) {
	mustMountOnce(srcDir, dstDir, "--rbind")
}

func mustMountOnce(srcDir, dstDir, option string) {
	mounts := mustRun(exec.Command("mount"))
	alreadyMounted := strings.Contains(mounts, fmt.Sprintf("on %s type", dstDir))

	if !alreadyMounted {
		mustRun(exec.Command("mount", option, srcDir, dstDir))
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
		fmt.Printf("%s: %s: %s", cmd.Path, err, string(out))
		os.Exit(1)
	}

	return string(out)
}
