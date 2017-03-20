package dns

import (
	"fmt"
	"net"
)

type NameserversSerializer struct{}

func (*NameserversSerializer) Serialize(nameservers []net.IP) []byte {
	var output string

	for _, nameserver := range nameservers {
		output = fmt.Sprintf("%snameserver %s\n", output, nameserver.String())
	}

	return []byte(output)
}
