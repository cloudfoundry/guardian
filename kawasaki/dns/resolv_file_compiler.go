package dns

import (
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager"
)

type ResolvFileCompiler struct{}

func (r *ResolvFileCompiler) Compile(log lager.Logger, resolvFilePath string, hostIP net.IP, overridingDNSServers, additionalDNSServers []net.IP) ([]byte, error) {
	log = log.Session("resolv-file-compile", lager.Data{
		"HostResolvFilePath":   resolvFilePath,
		"HostIP":               hostIP,
		"overridingDNSServers": overridingDNSServers,
		"AdditionalDNSServers": additionalDNSServers,
	})

	servers := []string{}
	for _, dnsServer := range overridingDNSServers {
		servers = append(servers, nameserver(dnsServer))
	}

	if len(servers) == 0 {
		var err error
		servers, err = parseHostResolvFile(resolvFilePath, hostIP)
		if err != nil {
			log.Error("reading-host-resolv", err)
			return nil, err
		}
	}

	for _, dnsServer := range additionalDNSServers {
		servers = append(servers, nameserver(dnsServer))
	}

	return []byte(strings.Join(append(servers, ""), "\n")), nil
}

func parseHostResolvFile(resolvFilePath string, hostIP net.IP) ([]string, error) {
	contents, err := ioutil.ReadFile(resolvFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading file '%s': %s", resolvFilePath, err)
	}

	matches, err := regexp.Match(`^\s*nameserver\s+127\.0\.0\.1\s*$`, contents)
	if err != nil {
		return nil, err
	}

	if matches {
		return []string{nameserver(hostIP)}, nil
	}

	servers := []string{}
	for _, entry := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if !strings.Contains(entry, "127.0.0.") {
			servers = append(servers, entry)
		}
	}

	return servers, nil
}

func nameserver(ip net.IP) string {
	return fmt.Sprintf("nameserver %s", ip.String())
}
