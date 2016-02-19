package dns

import (
	"fmt"
	"net"

	"github.com/pivotal-golang/lager"
)

type HostsFileCompiler struct {
	Handle string
	IP     net.IP
}

func (h *HostsFileCompiler) Compile(log lager.Logger) ([]byte, error) {
	contents := fmt.Sprintf("127.0.0.1 localhost\n%s %s\n", h.IP, h.Handle)
	return []byte(contents), nil
}
