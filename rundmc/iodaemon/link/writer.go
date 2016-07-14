package link

import (
	"encoding/gob"
	"net"

	"code.cloudfoundry.org/garden"
)

type Input struct {
	StdinData  []byte
	EOF        bool
	WindowSize *WindowSize
	Signal     *Signal
}

type Signal struct {
	Signal garden.Signal
}

type WindowSize struct {
	Columns int
	Rows    int
}

type Writer struct {
	conn net.Conn
	enc  *gob.Encoder
}

func NewWriter(conn net.Conn) *Writer {
	return &Writer{conn: conn, enc: gob.NewEncoder(conn)}
}

func (w *Writer) TerminateConnection() error {
	return w.conn.Close()
}

func (w *Writer) Write(d []byte) (int, error) {
	err := w.enc.Encode(Input{StdinData: d})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func (w *Writer) Close() error {
	return w.enc.Encode(Input{EOF: true})
}

func (w *Writer) SetWindowSize(cols, rows int) error {
	return w.enc.Encode(Input{
		WindowSize: &WindowSize{
			Columns: cols,
			Rows:    rows,
		},
	})
}

func (w *Writer) Signal(signal garden.Signal) error {
	return w.enc.Encode(Input{
		Signal: &Signal{signal},
	})

}
