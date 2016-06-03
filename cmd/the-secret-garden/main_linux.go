package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

	mustRun(exec.Command("mount", "--make-slave", dataDir))
	mustRun(exec.Command("chmod", "go-x", realGraphDir))

	mustBindMountOnce(graphDir, secretGraphDir)

	programPath, err := exec.LookPath(os.Args[4])
	if err != nil {
		fmt.Printf("failed to look path in namespace : %s\n", err)
		os.Exit(1)
	}

	if err := syscall.Exec(programPath, os.Args[4:], os.Environ()); err != nil {
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
		Pdeathsig:  syscall.SIGKILL,
	}
	forwardSignals(cmd, syscall.SIGTERM)

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
	alreadyMounted := strings.Contains(mounts, fmt.Sprintf("%s on %s", srcDir, dstDir))

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

func forwardSignals(cmd *exec.Cmd, signals ...os.Signal) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		cmd.Process.Signal(<-c)
	}()
}
