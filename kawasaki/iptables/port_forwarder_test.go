package iptables_test

import (
	"net"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PortForwarder", func() {
	var (
		fakeRunner *fake_command_runner.FakeCommandRunner
		forwarder  *iptables.PortForwarder
	)

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		forwarder = iptables.NewPortForwarder(
			iptables.New("/sbin/iptables", fakeRunner, "prefix-"),
		)
	})

	It("adds a NAT rule to forward the port", func() {
		Expect(forwarder.Forward(kawasaki.PortForwarderSpec{
			InstanceID:  "some-instance",
			ExternalIP:  net.ParseIP("5.6.7.8"),
			ContainerIP: net.ParseIP("1.2.3.4"),
			FromPort:    22,
			ToPort:      33,
		})).To(Succeed())

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "/sbin/iptables",
				Args: []string{
					"-w",
					"-A", "prefix-instance-some-instance",
					"--table", "nat",
					"--protocol", "tcp",
					"--destination", "5.6.7.8",
					"--destination-port", "22",
					"--jump", "DNAT",
					"--to-destination", "1.2.3.4:33",
				},
			},
		))
	})
})
