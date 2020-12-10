package gqt_test

import (
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sysctl", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		config.TCPKeepaliveTime = intptr(100)
		config.TCPKeepaliveInterval = intptr(200)
		config.TCPKeepaliveProbes = intptr(300)
		config.TCPRetries1 = intptr(255)
		config.TCPRetries2 = intptr(500)
	})

	JustBeforeEach(func() {
		var err error
		client = runner.Start(config)
		container, err = client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("sets the container tcp_keepalive_time parameter to the specified value", func() {
		output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_time"})
		Expect(strings.TrimSpace(output)).To(Equal("100"))
	})

	It("sets the container tcp_keepalive_intvl parameter to the specified value", func() {
		output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_intvl"})
		Expect(strings.TrimSpace(output)).To(Equal("200"))
	})

	It("sets the container tcp_keepalive_probes parameter to the specified value", func() {
		output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_probes"})
		Expect(strings.TrimSpace(output)).To(Equal("300"))
	})

	It("sets the container tcp_retries1 parameter to the specified value", func() {
		output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_retries1"})
		Expect(strings.TrimSpace(output)).To(Equal("255"))
	})

	It("sets the container tcp_retries2 parameter to the specified value", func() {
		output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_retries2"})
		Expect(strings.TrimSpace(output)).To(Equal("500"))
	})

	When("the tcp_keepalive_time specified value is 0", func() {
		BeforeEach(func() {
			config.TCPKeepaliveTime = intptr(0)
		})

		It("uses the default from the init_net namespace", func() {
			output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_time"})
			Expect(strings.TrimSpace(output)).NotTo(Equal("0"))
		})
	})

	When("the tcp_keepalive_intvl specified value is 0", func() {
		BeforeEach(func() {
			config.TCPKeepaliveInterval = intptr(0)
		})

		It("uses the default from the init_net namespace", func() {
			output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_intvl"})
			Expect(strings.TrimSpace(output)).NotTo(Equal("0"))
		})
	})

	When("the tcp_keepalive_probes specified value is 0", func() {
		BeforeEach(func() {
			config.TCPKeepaliveProbes = intptr(0)
		})

		It("uses the default from the init_net namespace", func() {
			output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_keepalive_probes"})
			Expect(strings.TrimSpace(output)).NotTo(Equal("0"))
		})
	})

	When("the tcp_retries1 specified value is 0", func() {
		BeforeEach(func() {
			config.TCPRetries1 = intptr(0)
		})

		It("uses the default from the init_net namespace", func() {
			output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_retries1"})
			Expect(strings.TrimSpace(output)).NotTo(Equal("0"))
		})
	})

	When("the tcp_retries2 specified value is 0", func() {
		BeforeEach(func() {
			config.TCPRetries2 = intptr(0)
		})

		It("uses the default from the init_net namespace", func() {
			output := runInContainerCombinedOutput(container, "/bin/cat", []string{"/proc/sys/net/ipv4/tcp_retries2"})
			Expect(strings.TrimSpace(output)).NotTo(Equal("0"))
		})
	})
})
