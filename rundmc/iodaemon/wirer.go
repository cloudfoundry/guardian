package iodaemon

import (
	"os"
	"os/exec"

	"github.com/kr/pty"
)

type Wirer struct {
	WithTty       bool
	WindowColumns int
	WindowRows    int
}

func (w *Wirer) Wire(cmd *exec.Cmd) (*os.File, *os.File, *os.File, *os.File, error) {
	extraFdR, extraFdW, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cmd.ExtraFiles = []*os.File{extraFdR}

	var stdinW, stdoutR, stderrR *os.File
	if w.WithTty {
		cmd.Stdin, stdinW, stdoutR, cmd.Stdout, stderrR, cmd.Stderr, err = createTtyPty(w.WindowColumns, w.WindowRows)
		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Setsid = true
	} else {
		cmd.Stdin, stdinW, stdoutR, cmd.Stdout, stderrR, cmd.Stderr, err = createPipes()
	}

	return stdinW, stdoutR, stderrR, extraFdW, nil
}

func createPipes() (stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW *os.File, err error) {
	// stderr will not be assigned in the case of a tty, so make
	// a dummy pipe to send across instead
	stderrR, stderrW, err = os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	stdinR, stdinW, err = os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	stdoutR, stdoutW, err = os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return
}

func createTtyPty(windowColumns int, windowRows int) (stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW *os.File, err error) {
	// stderr will not be assigned in the case of a tty, so ensure it will return EOF on read
	stderrR, err = os.Open("/dev/null")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	pty, tty, err := pty.Open()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	// do NOT assign stderrR to pty; the receiving end should only receive one
	// pty output stream, as they're both the same fd

	stdinW = pty
	stdoutR = pty

	stdinR = tty
	stdoutW = tty
	stderrW = tty

	setWinSize(stdinW, windowColumns, windowRows)

	return
}
