package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	uid := flag.Int64("uid", -1, "uid to run process as")
	gid := flag.Int64("gid", -1, "gid to run process as")
	flag.Parse()
	if *uid == -1 {
		fmt.Println("please pass the --uid flag")
		os.Exit(1)
	}
	if *gid == -1 {
		fmt.Println("please pass the --gid flag")
		os.Exit(1)
	}

	cmdArgv := flag.Args()
	cmd := exec.Command(cmdArgv[0], cmdArgv[1:]...)
	unprivilegedUser := &syscall.Credential{Uid: uint32(*uid), Gid: uint32(*gid)}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: unprivilegedUser}
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	must("run", err)
	panic("unreachable")
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s\n", action, err.Error())
		os.Exit(1)
	}
}
