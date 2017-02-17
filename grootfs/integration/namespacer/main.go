package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
	rootfsPath := os.Args[1]
	userId, err := strconv.ParseInt(os.Args[2], 10, 32)
	if err != nil {
		panic(err)
	}

	must(syscall.Mount(rootfsPath, rootfsPath, "", syscall.MS_BIND, ""))
	must(os.MkdirAll(filepath.Join(rootfsPath, "oldrootfs"), 0700))
	must(syscall.PivotRoot(rootfsPath, filepath.Join(rootfsPath, "oldrootfs")))
	must(os.Chdir("/"))

	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(userId),
			Gid: uint32(userId),
		},
	}

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
