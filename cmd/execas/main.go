package main

import (
	"flag"
	"fmt"
	"os"
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

	if err := syscall.Setgroups([]int{}); err != nil {
		must("setgroups", err)
	}
	_, _, err := syscall.Syscall(syscall.SYS_SETGID, uintptr(*gid), 0, 0)
	if err != 0 {
		must("setgid", err)
	}
	_, _, err = syscall.Syscall(syscall.SYS_SETUID, uintptr(*uid), 0, 0)
	if err != 0 {
		must("setuid", err)
	}

	cmdArgv := flag.Args()
	must("exec", syscall.Exec(cmdArgv[0], cmdArgv, os.Environ()))
	panic("unreachable")
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s\n", action, err)
		os.Exit(1)
	}
}
