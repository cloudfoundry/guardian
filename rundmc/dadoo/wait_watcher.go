package dadoo

import "net"

type WaitWatcher struct{}

func (ww *WaitWatcher) Wait(path string) (<-chan struct{}, error) {
	c, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}

	ch := make(chan struct{})
	go func(c net.Conn) {
		defer c.Close()

		b := make([]byte, 1)
		c.Read(b)
		close(ch)
	}(c)

	return ch, nil
}

func Listen(socketPath string) {
	sock, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			sock.Accept()
		}
	}()
}
