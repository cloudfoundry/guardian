package dns

import (
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
}

func (r *ResolvFileCompiler) Compile(log lager.Logger) ([]byte, error) {
	log = log.Session("resolv-file-compile", lager.Data{
		"HostResolvConfPath": r.HostResolvConfPath,
		"HostIP":             r.HostIP,
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
