package iodaemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"io"
)

// spawn listens on a unix socket at the given socketPath and when the first connection
// is received, starts a child process.
func Spawn(
	socketPath string,
	argv []string,
	timeout time.Duration,
	notifyStream io.WriteCloser,

	wirer *Wirer,
	daemon *Daemon,
) error {
	var listener net.Listener

	listener, err := listen(socketPath)
	if err != nil {
		return err
	}

	defer listener.Close()

	executablePath, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}

	cmd := child(executablePath, argv)

	stdinW, stdoutR, stderrR, extraFdW, err := wirer.Wire(cmd)
	if err != nil {
		return err
	}

	statusR, statusW, err := os.Pipe()
	if err != nil {
		return err
	}

	launched := make(chan bool)

	go func() {
		var once sync.Once

		for {
			fmt.Fprintln(notifyStream, "ready")
			conn, err := acceptConnection(listener, stdoutR, stderrR, statusR)
			if err != nil {
				return // in general this means the listener has been closed
			}

			once.Do(func() {
				err := cmd.Start()
				if err != nil {
					panic(err)
				}

				fmt.Fprintln(notifyStream, "active")
				notifyStream.Close()
				launched <- true
			})

			daemon.HandleConnection(conn, cmd.Process, stdinW, extraFdW)
		}
	}()

	select {
	case <-launched:
		var exit byte = 0
		if err := cmd.Wait(); err != nil {
			ws := err.(*exec.ExitError).ProcessState.Sys().(syscall.WaitStatus)
			exit = byte(ws.ExitStatus())
		}

		fmt.Fprintf(statusW, "%d\n", exit)
	case <-time.After(timeout):
		return fmt.Errorf("expected client to connect within %s", timeout)
	}

	return nil
}

func listen(socketPath string) (net.Listener, error) {
	// Delete socketPath if it exists to avoid bind failures.
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	err = os.MkdirAll(filepath.Dir(socketPath), 0755)
	if err != nil {
		return nil, err
	}

	return net.Listen("unix", socketPath)
}

func acceptConnection(listener net.Listener, stdoutR, stderrR, statusR *os.File) (net.Conn, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, err
	}

	rights := syscall.UnixRights(
		int(stdoutR.Fd()),
		int(stderrR.Fd()),
		int(statusR.Fd()),
	)

	_, _, err = conn.(*net.UnixConn).WriteMsgUnix([]byte{}, rights, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
