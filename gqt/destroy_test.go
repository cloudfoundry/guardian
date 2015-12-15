package gqt_test

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Destroying a Container", func() {
	var (
		client          *runner.RunningGarden
		container       garden.Container
		containerHandle string
	)

	BeforeEach(func() {
		var err error

		client = startGarden()

		container, err = client.Create(garden.ContainerSpec{
			Network: "177.100.10.12/24",
		})
		Expect(err).NotTo(HaveOccurred())

		containerHandle = container.Handle()
		Expect(client.Destroy(containerHandle)).To(Succeed())
	})

	It("should remove iptable entries", func() {
		session, err := gexec.Start(
			exec.Command("iptables", "-S", "-t", "filter"),
			GinkgoWriter, GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())
		Consistently(session).ShouldNot(gbytes.Say("br-177-100-10-0"))

		session, err = gexec.Start(
			exec.Command("iptables", "-S", "-t", "nat"),
			GinkgoWriter, GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())
		Consistently(session).ShouldNot(gbytes.Say("br-177-100-10-0"))
	})

	It("should remove namespaces", func() {
		session, err := gexec.Start(
			exec.Command("ip", "netns", "list"),
			GinkgoWriter, GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())
		Consistently(session).ShouldNot(gbytes.Say(containerHandle))
	})

	It("should remove virtual ethernet cards", func() {
		ifaceName := fmt.Sprintf("w%d-%s-0", GinkgoParallelNode(), networkHandle(containerHandle))

		ifconfigExits := func() int {
			session, err := gexec.Start(exec.Command("ifconfig", ifaceName), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			return session.Wait().ExitCode()
		}

		Eventually(ifconfigExits).ShouldNot(Equal(0))
	})

	It("should remove the network bridge", func() {
		session, err := gexec.Start(
			exec.Command("ifconfig"),
			GinkgoWriter, GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())
		Consistently(session).ShouldNot(gbytes.Say("br-177-100-10-0"))
	})
})

func networkHandle(handle string) string {
	return handle[0:8]
}
