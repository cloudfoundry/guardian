package dns_test

import (
	"net"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dns Suite")
}

func ips(addresses ...string) []net.IP {
	netIPs := []net.IP{}
	for _, address := range addresses {
		netIPs = append(netIPs, net.ParseIP(address))
	}
	return netIPs
}
