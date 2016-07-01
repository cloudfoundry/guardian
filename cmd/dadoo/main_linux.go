package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	"github.com/eapache/go-resiliency/retrier"
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

	command := flag.Args()[0] // e.g. exec
	runtime := flag.Args()[1] // e.g. runc
	dir := flag.Args()[2]     // bundlePath for run, processPath for exec
	containerId := flag.Args()[3]

	fd3 := os.NewFile(3, "/proc/self/fd/3")

	signals := make(chan os.Signal, 100)
	signal.Notify(signals, syscall.SIGCHLD)

	logFile := fmt.Sprintf("/proc/%d/fd/4", os.Getpid())
	logFD := os.NewFile(4, "/proc/self/fd/4")

	ttyWindowSizeFD := os.NewFile(5, "/proc/self/fd/5")

	pidFilePath := filepath.Join(dir, "pidfile")

	check(os.MkdirAll(dir, 0700))
	defer os.RemoveAll(dir) // for exec dadoo is responsible for creating & cleaning up

	stdin := forwardReadFIFO(filepath.Join(dir, "stdin"))
	stdout := forwardWriteFIFO(filepath.Join(dir, "stdout"))
	stderr := forwardWriteFIFO(filepath.Join(dir, "stderr"))

	var runcStartCmd *exec.Cmd
	if tty {
		ttySlave := setupTty(stdin, stdout, ttyWindowSizeFD, pidFilePath)
		check(ttySlave.Chown(uid, gid))
		runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "exec", "-d", "-tty", "-console", ttySlave.Name(), "-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()), "-pid-file", pidFilePath, containerId)
	} else {
		runcStartCmd = exec.Command(runtime, "-debug", "-log", logFile, "exec", "-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()), "-d", "-pid-file", pidFilePath, containerId)
		runcStartCmd.Stdin = stdin
		runcStartCmd.Stdout = stdout
		runcStartCmd.Stderr = stderr
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

	containerPid, err := parsePid(pidFilePath)
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

func setupTty(stdin io.Reader, stdout io.Writer, ttyWindowSizeFD io.Reader, pidFilePath string) *os.File {
	m, s, err := pty.Open()
	if err != nil {
		check(err)
	}

	go io.Copy(stdout, m)

	go func() {
		io.Copy(m, stdin)
		m.Close()
	}()

	go func() {
		setWinSize(m, ttyWindowSizeFD)

		pid, err := readPid(pidFilePath)
		if err != nil {
			println("Timed out trying to open pidfile: ", err.Error())
			return
		}

		for {
			setWinSize(m, ttyWindowSizeFD)

			p, _ := os.FindProcess(pid)
			p.Signal(syscall.SIGWINCH)
		}
	}()

	return s
}

func setWinSize(m *os.File, ttyWindowSizeFD io.Reader) {
	ttySize := &dadoo.TtySize{}
	json.NewDecoder(ttyWindowSizeFD).Decode(ttySize)
	dadoo.SetWinSize(m, ttySize)
}

func readPid(pidFilePath string) (int, error) {
	retrier := retrier.New(retrier.ConstantBackoff(20, 500*time.Millisecond), nil)
	var (
		pid int = -1
		err error
	)
	retrier.Run(func() error {
		pid, err = parsePid(pidFilePath)
		return err
	})

	return pid, err
}

func parsePid(pidFile string) (int, error) {
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
