package iodaemon

import (
	"encoding/gob"
	"io"
	"os"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/link"
)

type Daemon struct {
	WithTty bool
}

func (d *Daemon) HandleConnection(conn io.ReadCloser, process *os.Process, stdin *os.File) {
	decoder := gob.NewDecoder(conn)

	for {
		var input link.Input
		err := decoder.Decode(&input)
		if err != nil {
			break
		}

		if err := d.handle(input, process, stdin); err != nil {
			conn.Close()
			break
		}
	}
}

func (d *Daemon) handle(input link.Input, process *os.Process, stdin *os.File) error {
	if input.WindowSize != nil {
		setWinSize(stdin, input.WindowSize.Columns, input.WindowSize.Rows)
		process.Signal(syscall.SIGWINCH)
	} else if input.EOF {
		stdin.Sync()
		err := stdin.Close()
		if d.WithTty {
			process.Signal(syscall.SIGHUP)
		}
		if err != nil {
			return err
		}
	} else if input.Signal != nil {
		if input.Signal.Signal == garden.SignalTerminate {
			process.Signal(syscall.SIGTERM)
		} else if input.Signal.Signal == garden.SignalKill {
			process.Signal(syscall.SIGKILL)
		}
	} else {
		_, err := stdin.Write(input.StdinData)
		if err != nil {
			return err
		}
	}

	return nil
}
