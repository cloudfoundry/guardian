package rundmc

import (
	"errors"
	"io"
	"time"

	"github.com/pivotal-golang/lager"
)

type StartChecker struct {
	Expect  string
	Timeout time.Duration
}

func (s StartChecker) Check(log lager.Logger, output io.Reader) error {
	log = log.Session("check", lager.Data{
		"expect":  s.Expect,
		"timeout": s.Timeout,
	})

	log.Info("started")
	defer log.Info("finished")

	detected := make(chan struct{})
	go func() {
		buff := make([]byte, len(s.Expect))
		io.ReadAtLeast(output, buff, len(s.Expect))

		if string(buff) == s.Expect {
			close(detected)
		}
	}()

	select {
	case <-detected:
		return nil
	case <-time.After(s.Timeout):
		return errors.New("timed out waiting for container to start")
	}
}
