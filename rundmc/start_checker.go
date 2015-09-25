package rundmc

import (
	"errors"
	"io"
	"time"
)

type StartChecker struct {
	Expect  string
	Timeout time.Duration
}

func (s StartChecker) Check(output io.Reader) error {
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
