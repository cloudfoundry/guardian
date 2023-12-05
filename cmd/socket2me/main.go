//go:build !windows

package main

import (
	"flag"
	"fmt"
	"net"
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
	uid := flag.Int64("uid", -1, "uid to run process as")
	gid := flag.Int64("gid", -1, "gid to run process as")
	socketUID := flag.Int64("socket-uid", -1, "uid to chown socket to")
	socketGID := flag.Int64("socket-gid", -1, "gid to chown socket to")
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
	if *socketUID == -1 {
		fmt.Println("please pass the --socket-uid flag")
		os.Exit(1)
	}
	if *socketGID == -1 {
		fmt.Println("please pass the --socket-gid flag")
		os.Exit(1)
	}

	socketFD := createSocket(*socketPath, *socketUID, *socketGID)

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
	environment := os.Environ()
	envWithFD := append(environment, fmt.Sprintf("SOCKET2ME_FD=%d", socketFD))
	must("exec", syscall.Exec(cmdArgv[0], cmdArgv, envWithFD))
	panic("unreachable")
}

func createSocket(socketPath string, socketUID, socketGID int64) uintptr {
	_, err := os.Stat(socketPath)
	if err == nil {
		must("remove socket", os.Remove(socketPath))
	}
	if err != nil && !os.IsNotExist(err) {
		must("stat socket", err)
	}

	listener, err := net.Listen("unix", socketPath)
	must("listen", err)
	netFd, err := listener.(*net.UnixListener).File()
	must("get socket file descriptor", err)
	fd := netFd.Fd()
	_, _, errNo := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0)
	if errNo != 0 {
		must("clear cloexec flag", err)
	}

	must("chown socket", os.Chown(socketPath, int(socketUID), int(socketGID)))
	must("chmod socket", os.Chmod(socketPath, 0600))
	return fd
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s\n", action, err)
		os.Exit(1)
	}
}
