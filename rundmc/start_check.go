package rundmc

import (
	"errors"
	"io"
	"io/ioutil"
	"time"
)

type StdoutCheck struct {
	Expect  string
	Timeout time.Duration
}

func (s StdoutCheck) Check(stdout, stderr io.Reader) error {
	detected := make(chan struct{})
	go func() {
		buff := make([]byte, len(s.Expect))
		io.ReadAtLeast(stdout, buff, len(s.Expect))

		if string(buff) == s.Expect {
			close(detected)
		}
	}()

	select {
	case <-detected:
		return nil
	case <-time.After(s.Timeout):
		if b, err := ioutil.ReadAll(stderr); err == nil {
			return errors.New(string(b))
		}

		return errors.New("timeout, and could not read stderr")
	}

	return nil
}
