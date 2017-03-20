package dns

import (
	"net"
	"regexp"
	"strings"
)

type NameserversDeterminer struct{}

func (n *NameserversDeterminer) Determine(resolvContents string, hostIP net.IP, pluginNameservers, operatorNameservers, additionalNameservers []net.IP) []net.IP {
	if pluginNameservers != nil {
		return pluginNameservers
	}

	if len(operatorNameservers) > 0 {
		return append(operatorNameservers, additionalNameservers...)
	}

	nameserversFromHost := parseResolvContents(resolvContents, hostIP)
	return append(nameserversFromHost, additionalNameservers...)
}

func parseResolvContents(resolvContents string, hostIP net.IP) []net.IP {
	loopbackNameserver := regexp.MustCompile(`^\s*nameserver\s+127\.0\.0\.\d+\s*$`)

	if loopbackNameserver.MatchString(resolvContents) {
		return []net.IP{hostIP}
	}

	nameservers := []net.IP{}
	for _, resolvEntry := range strings.Split(strings.TrimSpace(resolvContents), "\n") {
		if !strings.Contains(resolvEntry, "127.0.0.") {
			nameserverFields := strings.Fields(resolvEntry)
			if len(nameserverFields) != 2 {
				continue
			}
			nameservers = append(nameservers, net.ParseIP(nameserverFields[1]))
		}
	}

	return nameservers
}
