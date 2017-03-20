package dns_test

import (
	"net"

	. "code.cloudfoundry.org/guardian/kawasaki/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("NameserversDeterminer", func() {
	var (
		determiner = &NameserversDeterminer{}
		hostIP     = net.ParseIP("9.8.7.6")
	)

	DescribeTable(
		"Determine",
		func(hostResolvContents string, pluginNameservers, operatorNameservers, additionalNameservers, expectedNameservers []net.IP) {
			nameservers := determiner.Determine(hostResolvContents, hostIP, pluginNameservers, operatorNameservers, additionalNameservers)
			Expect(nameservers).To(Equal(expectedNameservers))
		},
		Entry(
			"when passed >=1 pluginNameservers, it returns only them",
			"nameserver 1.2.3.4\n", ips("10.0.0.3"), ips("10.0.0.1", "10.0.0.2"), ips("10.0.0.4"),
			ips("10.0.0.3"),
		),
		Entry(
			"when explicitly passed 0 pluginNameservers, it returns an empty list",
			"nameserver 1.2.3.4\n", ips(), ips("10.0.0.1", "10.0.0.2"), ips("10.0.0.4"),
			ips(),
		),
		Entry(
			"when passed >=1 operatorNameservers, it returns them",
			"nameserver 1.2.3.4\n", nil, ips("10.0.0.1", "10.0.0.2"), ips(),
			ips("10.0.0.1", "10.0.0.2"),
		),
		Entry("when passed >=1 additionalNameservers and 0 operatorNameservers, it appends them to the host's nameservers",
			"nameserver 1.2.3.4\n", nil, nil, ips("10.0.0.1", "10.0.0.2"),
			ips("1.2.3.4", "10.0.0.1", "10.0.0.2"),
		),
		Entry("when the host nameservers contain loopback (127.0.0.0/24 for now) entries, it returns all other entries",
			"nameserver 1.2.3.4\nnameserver 127.0.0.19\n", nil, nil, ips(),
			ips("1.2.3.4"),
		),
		Entry("when the host nameservers consist of exactly one loopback entry, it returns the host IP",
			"nameserver 127.0.0.19\n", nil, nil, ips(),
			[]net.IP{hostIP},
		),
		Entry("when passed >=1 additionalNameservers and >1 operatorNameservers, it returns those lists and nothing from host",
			"nameserver 1.2.3.4\n", nil, ips("10.0.0.3"), ips("10.0.0.1", "10.0.0.2"),
			ips("10.0.0.3", "10.0.0.1", "10.0.0.2"),
		),
		Entry("when passed 0 additionalNameservers and 0 operatorNameservers, it returns the host's non-127.0.0.0/24 nameservers",
			"nameserver 1.2.3.4\nnameserver 127.0.0.1\nnameserver 5.6.7.8\n", nil, nil, ips(),
			ips("1.2.3.4", "5.6.7.8"),
		),
		Entry("when the host has no nameservers, it returns an empty list",
			"", nil, nil, ips(),
			ips(),
		),
	)
})
