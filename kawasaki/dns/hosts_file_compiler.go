package dns

import (
	"fmt"
	"net"
	"strings"

	"code.cloudfoundry.org/lager"
)

type HostsFileCompiler struct {
}

func (h *HostsFileCompiler) Compile(log lager.Logger, ip net.IP, handle string, additionalHostEntries []string) ([]byte, error) {
	if len(handle) > 49 {
		handle = handle[len(handle)-49:]
	}
	hostEntries := []string{"127.0.0.1 localhost"}
	hostEntries = append(hostEntries, fmt.Sprintf("%s %s", ip, handle))
	hostEntries = append(hostEntries, additionalHostEntries...)
	contents := strings.Join(hostEntries, "\n")
	contents = contents + "\n"

	return []byte(contents), nil
}
