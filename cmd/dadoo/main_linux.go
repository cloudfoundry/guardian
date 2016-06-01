package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	"github.com/opencontainers/runc/libcontainer/system"
)

func main() {
	var logFile, stdoutPath, stdinPath, stderrPath string
	flag.StringVar(&logFile, "log", "dadoo.log", "dadoo log file path")
	flag.StringVar(&stdoutPath, "stdout", "", "path to stdout")
	flag.StringVar(&stdinPath, "stdin", "", "path to stdin")
	flag.StringVar(&stderrPath, "stderr", "", "path to stderr")

	flag.Parse()

	command := flag.Args()[0] // e.g. run
	runtime := flag.Args()[1] // e.g. runc
	bundlePath := flag.Args()[2]
	containerId := flag.Args()[3]

	fd3 := os.NewFile(3, "/proc/self/fd/3")

	signals := make(chan os.Signal, 100)
	signal.Notify(signals, syscall.SIGCHLD)

	pidFilePath := filepath.Join(bundlePath, "pidfile")

	var runcStartCmd *exec.Cmd
	switch command {
	case "run":
		runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "start", "-d", "-pid-file", pidFilePath, containerId)
		runcStartCmd.Dir = bundlePath

		// listen to an exit socket early so waiters can wait for dadoo
		dadoo.Listen(filepath.Join(bundlePath, "exit.sock"))
	case "exec":
		check(os.MkdirAll(bundlePath, 0700))
		runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "exec", "-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()), "-d", "-pid-file", pidFilePath, containerId)
		runcStartCmd.Stdin = forwardReadFIFO(stdinPath)
		runcStartCmd.Stdout = forwardWriteFIFO(stdoutPath)
		runcStartCmd.Stderr = forwardWriteFIFO(stderrPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s", command)
		os.Exit(127)
	}

	// we need to be the subreaper so we can wait on the detached container process
	system.SetSubreaper(os.Getpid())

	if err := runcStartCmd.Start(); err != nil {
		fd3.Write([]byte{2})
		os.Exit(2)
	}

	containerPid := -2
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

				containerPid, err = readPid(pidFilePath)
				check(err)
			}

			if wpid == containerPid || containerPid < 0 {
				exits[wpid] = status.ExitStatus()
			}

			if status, ok := exits[containerPid]; ok {
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

func forwardReadFIFO(path string) io.Reader {
	if path == "" {
		return nil
	}

	r, err := os.Open(path)
	check(err)

	return r
}

func forwardWriteFIFO(path string) io.Writer {
	if path == "" {
		return nil
	}

	w, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	check(err)

	return w
}
