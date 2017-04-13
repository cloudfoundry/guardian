package dns

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

type ResolvCompiler struct{}

func (n *ResolvCompiler) Determine(resolvContents string, hostIP net.IP, pluginNameservers, operatorNameservers, additionalNameservers []net.IP) []string {
	if pluginNameservers != nil {
		return nameserverEntries(pluginNameservers)
	}

	if len(operatorNameservers) > 0 {
		return nameserverEntries(append(operatorNameservers, additionalNameservers...))
	}

	nameserversFromHost := parseResolvContents(resolvContents, hostIP)
	return append(nameserversFromHost, nameserverEntries(additionalNameservers)...)
}

func parseResolvContents(resolvContents string, hostIP net.IP) []string {
	loopbackNameserver := regexp.MustCompile(`^\s*nameserver\s+127\.0\.0\.\d+\s*$`)
	if loopbackNameserver.MatchString(resolvContents) {
		return nameserverEntries([]net.IP{hostIP})
	}

	entries := []string{}
	for _, resolvEntry := range strings.Split(strings.TrimSpace(resolvContents), "\n") {
		if resolvEntry == "" {
			continue
		}

		if !strings.HasPrefix(resolvEntry, "nameserver") {
			entries = append(entries, resolvEntry)
			continue
		}

		if !strings.Contains(resolvEntry, "127.0.0.") {
			nameserverFields := strings.Fields(resolvEntry)
			if len(nameserverFields) != 2 {
				continue
			}
			entries = append(entries, nameserverEntry(nameserverFields[1]))
		}
	}

	return entries
}

func nameserverEntries(ips []net.IP) []string {
	nameserverEntries := []string{}

	for _, ip := range ips {
		nameserverEntries = append(nameserverEntries, nameserverEntry(ip.String()))
	}
	return nameserverEntries
}

func nameserverEntry(ip string) string {
	return fmt.Sprintf("nameserver %s", ip)
}
