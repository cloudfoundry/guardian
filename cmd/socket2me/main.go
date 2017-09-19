package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
}

func main() {
	socketPath := flag.String("socket-path", "", "Unix socket path. Can already exist.")
	uid := flag.Int("uid", -1, "uid to run process as")
	gid := flag.Int("gid", -1, "gid to run process as")
	flag.Parse()
	if *socketPath == "" {
		fmt.Println("please pass the --socket-path flag")
		os.Exit(1)
	}
	if *uid == -1 {
		fmt.Println("please pass the --uid flag")
		os.Exit(1)
	}
	if *gid == -1 {
		fmt.Println("please pass the --gid flag")
		os.Exit(1)
	}

	socketFD := createSocket(*socketPath)

	_, _, err := syscall.RawSyscall(syscall.SYS_SETGID, uintptr(*gid), 0, 0)
	if err != 0 {
		must("setgid", err)
	}
	_, _, err = syscall.RawSyscall(syscall.SYS_SETUID, uintptr(*uid), 0, 0)
	if err != 0 {
		must("setuid", err)
	}

	cmdArgv := flag.Args()
	environment := os.Environ()
	envWithFD := append(environment, fmt.Sprintf("SOCKET2ME_FD=%d", socketFD))
	must("exec", syscall.Exec(cmdArgv[0], cmdArgv, envWithFD))
	panic("unreachable")
}

func createSocket(socketPath string) int {
	_, err := os.Stat(socketPath)
	if err == nil {
		must("remove socket", os.Remove(socketPath))
	}
	if err != nil && !os.IsNotExist(err) {
		must("stat socket", err)
	}
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	must("create socket", err)
	must("bind socket", syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
	return fd
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s\n", action, err)
		os.Exit(1)
	}
}
