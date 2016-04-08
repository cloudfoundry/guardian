package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/system"
)

func main() {
	var logFile string
	flag.StringVar(&logFile, "log", "dadoo.log", "dadoo log file path")
	flag.Parse()

	command := flag.Args()[0] // e.g. run
	runtime := flag.Args()[1] // e.g. runc
	bundlePath := flag.Args()[2]
	containerId := flag.Args()[3]

	if command != "run" {
		fmt.Fprintf(os.Stderr, "unknown command: %s", command)
		os.Exit(127)
	}

	fd3 := os.NewFile(3, "/proc/self/fd/3")

	signals := make(chan os.Signal, 100)
	signal.Notify(signals, syscall.SIGCHLD)

	pidFilePath := filepath.Join(bundlePath, "pidfile")

	// we need to be the subreaper so we can wait on the detached container process
	system.SetSubreaper(os.Getpid())

	runcStartCmd := exec.Command(runtime, "-debug", "-log", logFile, "start", "-d", "-pid-file", pidFilePath, containerId)
	runcStartCmd.Dir = bundlePath

	if err := runcStartCmd.Start(); err != nil {
		fd3.Write([]byte{2})
		os.Exit(2)
	}

	pid := -2
	for range signals {

		exits := make(map[int]int)
		for {
			var status syscall.WaitStatus
			var rusage syscall.Rusage
			wpid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)

			if err != nil || wpid <= 0 {
				break // wait for next SIGCHLD
			}

			if wpid == runcStartCmd.Process.Pid {
				fd3.Write([]byte{byte(status.ExitStatus())})

				if status.ExitStatus() != 0 {
					os.Exit(3) // nothing to wait for, container didn't launch
				}

				pid, err = readPid(pidFilePath)
				check(err)
			}

			if wpid == pid || pid < 0 {
				exits[wpid] = status.ExitStatus()
			}

			if status, ok := exits[pid]; ok {
				check(exec.Command(runtime, "delete", containerId).Run())
				os.Exit(status)
			}
		}
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func readPid(pidFile string) (int, error) {
	b, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(b), "%d", &pid); err != nil {
		return -1, err
	}

	return pid, nil
}
