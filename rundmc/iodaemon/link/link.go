package link

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/npat-efault/poller"
)

type SignalMsg struct {
	Signal syscall.Signal `json:"signal"`
}

type Link struct {
	*Writer

	exitStatus <-chan int
}

func Create(socketPath string, stdout io.Writer, stderr io.Writer) (*Link, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to i/o daemon: %s", err)
	}

	var b [2048]byte
	var oob [2048]byte

	n, oobn, _, _, err := conn.(*net.UnixConn).ReadMsgUnix(b[:], oob[:])
	if err != nil {
		return nil, fmt.Errorf("failed to read unix msg: %s (read: %d, %d)", err, n, oobn)
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, fmt.Errorf("failed to parse socket control message: %s", err)
	}

	if len(scms) < 1 {
		return nil, fmt.Errorf("no socket control messages sent")
	}

	scm := scms[0]

	fds, err := syscall.ParseUnixRights(&scm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unix rights: %s", err)
	}

	if len(fds) != 3 {
		return nil, fmt.Errorf("invalid number of fds; need 3, got %d", len(fds))
	}

	for _, fd := range fds {
		syscall.CloseOnExec(fd)
	}

	lstdout, err := poller.NewFD(fds[0])
	if err != nil {
		return nil, err
	}

	var devNullS syscall.Stat_t
	if err := syscall.Stat("/dev/null", &devNullS); err != nil {
		return nil, err
	}

	var s syscall.Stat_t
	if err := syscall.Fstat(fds[1], &s); err != nil {
		return nil, err
	}

	lstatus, err := poller.NewFD(fds[2])
	if err != nil {
		return nil, err
	}

	linkWriter := NewWriter(conn)
	done := make(chan struct{})

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func(fd io.Reader) {
		defer wg.Done()

		for {
			buff := make([]byte, 32*1024)
			n, err := fd.Read(buff)

			if n > 0 {
				stdout.Write(buff[0:n])
			}

			if err != nil {
				return
			}
		}
	}(lstdout)

	// if using a tty, stderr will be /dev/null remotely, which we can't poll
	var lstderr io.ReadCloser = ioutil.NopCloser(&bytes.Buffer{})
	if s.Rdev != devNullS.Rdev {
		lstderr, err = poller.NewFD(fds[1])
		if err != nil {
			return nil, err
		}

		wg.Add(1)
		go func(fd io.Reader) {
			defer wg.Done()

			for {
				buff := make([]byte, 32*1024)
				n, err := fd.Read(buff)

				if n > 0 {
					stderr.Write(buff[0:n])
				}

				if err != nil {
					return
				}
			}
		}(lstderr)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	exitStatus := make(chan int, 2)
	go func() {
		var s int
		_, err := fmt.Fscanf(lstatus, "%d\n", &s)
		if err != nil {
			s = 255
		}

		select {
		case <-done:
			// quit immediately if stdout and error have closed
		case <-time.After(200 * time.Millisecond):
			// need a timeout here in case streams never close
		}

		conn.Close()
		lstatus.Close()
		lstdout.Close()
		lstderr.Close()

		exitStatus <- s
	}()

	return &Link{
		Writer:     linkWriter,
		exitStatus: exitStatus,
	}, nil
}

func (link *Link) Wait() (int, error) {
	return <-link.exitStatus, nil
}
