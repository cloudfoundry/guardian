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
	"github.com/kr/pty"
	"github.com/opencontainers/runc/libcontainer/system"
)

func main() {
	os.Exit(run())
}

func run() int {
	var uid, gid int
	var tty bool
	flag.IntVar(&uid, "uid", 0, "uid to chown console to")
	flag.IntVar(&gid, "gid", 0, "gid to chown console to")
	flag.BoolVar(&tty, "tty", false, "tty requested")

	flag.Parse()

	command := flag.Args()[0] // e.g. run
	runtime := flag.Args()[1] // e.g. runc
	dir := flag.Args()[2]     // bundlePath for run, processPath for exec
	containerId := flag.Args()[3]

	fd3 := os.NewFile(3, "/proc/self/fd/3")

	signals := make(chan os.Signal, 100)
	signal.Notify(signals, syscall.SIGCHLD)

	logFile := fmt.Sprintf("/proc/%d/fd/4", os.Getpid())
	logFD := os.NewFile(4, "/proc/self/fd/4")

	pidFilePath := filepath.Join(dir, "pidfile")

	var runcStartCmd *exec.Cmd
	switch command {
	case "run":
		runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "start", "-d", "-pid-file", pidFilePath, containerId)
		runcStartCmd.Dir = dir

		// listen to an exit socket early so waiters can wait for dadoo
		dadoo.Listen(filepath.Join(dir, "exit.sock"))
	case "exec":
		check(os.MkdirAll(dir, 0700))
		defer os.RemoveAll(dir) // for exec dadoo is responsible for creating & cleaning up

		stdin := forwardReadFIFO(filepath.Join(dir, "stdin"))
		stdout := forwardWriteFIFO(filepath.Join(dir, "stdout"))
		stderr := forwardWriteFIFO(filepath.Join(dir, "stderr"))

		if tty {
			ttyFile := setupTty(stdin, stdout)
			check(ttyFile.Chown(uid, gid))
			runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "exec", "-d", "-tty", "-console", ttyFile.Name(), "-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()), "-pid-file", pidFilePath, containerId)
		} else {
			runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "exec", "-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()), "-d", "-pid-file", pidFilePath, containerId)
			runcStartCmd.Stdin = stdin
			runcStartCmd.Stdout = stdout
			runcStartCmd.Stderr = stderr
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s", command)
		return 127
	}

	// we need to be the subreaper so we can wait on the detached container process
	system.SetSubreaper(os.Getpid())

	if err := runcStartCmd.Start(); err != nil {
		fd3.Write([]byte{2})
		return 2
	}

	var status syscall.WaitStatus
	var rusage syscall.Rusage
	_, err := syscall.Wait4(runcStartCmd.Process.Pid, &status, 0, &rusage)
	check(err)    // Start succeeded but Wait4 failed, this can only be a programmer error
	logFD.Close() // No more logs from runc so close fd

	fd3.Write([]byte{byte(status.ExitStatus())})
	if status.ExitStatus() != 0 {
		return 3 // nothing to wait for, container didn't launch
	}

	containerPid, err := readPid(pidFilePath)
	check(err)

	for range signals {
		for {
			wpid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)
			if err != nil || wpid <= 0 {
				break // wait for next SIGCHLD
			}

			if wpid == containerPid {
				if command == "run" {
					check(exec.Command(runtime, "delete", containerId).Run())
				}

				return status.ExitStatus()
			}
		}
	}

	return 0
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
	r, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}

	check(err)
	return r
}

func forwardWriteFIFO(path string) io.Writer {
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if os.IsNotExist(err) {
		return nil
	}

	check(err)
	return w
}

func setupTty(stdin io.Reader, stdout io.Writer) *os.File {
	m, s, err := pty.Open()
	if err != nil {
		check(err)
	}

	go io.Copy(stdout, m)
	go func() {
		io.Copy(m, stdin)
		m.Close()
	}()

	return s
}
