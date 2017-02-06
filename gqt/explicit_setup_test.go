package gqt_test

import (
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = FDescribe("gdn setup", func() {
	BeforeEach(func() {
		setupProcess, err := gexec.Start(exec.Command(gardenBin, "setup"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))
	})

	It("sets up cgroups", func() {})

	It("sets up iptables", func() {})
})

var _ = FDescribe("running gdn setup before starting server", func() {
	var client *runner.RunningGarden

	BeforeEach(func() {
		setupProcess, err := gexec.Start(exec.Command(gardenBin, "setup"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))
		client = startGarden("server")
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("creates a container", func() {
		_, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})
})
