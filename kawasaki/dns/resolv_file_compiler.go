package dns

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"

	"code.cloudfoundry.org/lager"
)

type ResolvFileCompiler struct{}

func (r *ResolvFileCompiler) Compile(log lager.Logger, resolvConfPath string, hostIP net.IP, overrideServers []net.IP) ([]byte, error) {
	log = log.Session("resolv-file-compile", lager.Data{
		"HostResolvConfPath": resolvConfPath,
		"HostIP":             hostIP,
		"OverrideServers":    overrideServers,
	})

	f, err := os.Open(resolvConfPath)
	if err != nil {
		log.Error("reading-host-resolv-conf", err)
		return nil, fmt.Errorf("reading file '%s': %s", resolvConfPath, err)
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		log.Error("reading-host-resolv-conf", err)
		return nil, fmt.Errorf("reading file '%s': %s", resolvConfPath, err)
	}

	if len(overrideServers) > 0 {
		var buf bytes.Buffer
		for _, name := range overrideServers {
			fmt.Fprintf(&buf, "nameserver %s\n", name.String())
		}
		return buf.Bytes(), nil
	}

	matches, err := regexp.Match(`^\s*nameserver\s+127\.0\.0\.1\s*$`, contents)
	if err != nil {
		log.Error("matching-regexp", err)
		return nil, err
	}

	if matches {
		return []byte(fmt.Sprintf("nameserver %s\n", hostIP.String())), nil
	}
	return contents, nil
}
