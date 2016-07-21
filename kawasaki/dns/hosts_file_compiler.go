package dns

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/lager"
)

type HostsFileCompiler struct {
}

func (h *HostsFileCompiler) Compile(log lager.Logger, ip net.IP, handle string) ([]byte, error) {
	contents := fmt.Sprintf("127.0.0.1 localhost\n%s %s\n", ip, handle)
	return []byte(contents), nil
}
