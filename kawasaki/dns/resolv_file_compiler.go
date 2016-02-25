package dns

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"

	"github.com/pivotal-golang/lager"
)

type ResolvFileCompiler struct {
	HostResolvConfPath string
	HostIP             net.IP
	OverrideServers    []net.IP
}

func (r *ResolvFileCompiler) Compile(log lager.Logger) ([]byte, error) {
	log = log.Session("resolv-file-compile", lager.Data{
		"HostResolvConfPath": r.HostResolvConfPath,
		"HostIP":             r.HostIP,
		"OverrideServers":    r.OverrideServers,
	})

	f, err := os.Open(r.HostResolvConfPath)
	if err != nil {
		log.Error("reading-host-resolv-conf", err)
		return nil, fmt.Errorf("reading file '%s': %s", r.HostResolvConfPath, err)
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		log.Error("reading-host-resolv-conf", err)
		return nil, fmt.Errorf("reading file '%s': %s", r.HostResolvConfPath, err)
	}

	if len(r.OverrideServers) > 0 {
		var buf bytes.Buffer
		for _, name := range r.OverrideServers {
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
		return []byte(fmt.Sprintf("nameserver %s\n", r.HostIP.String())), nil
	}
	return contents, nil
}
