package netns_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("NetnsExecerLinux", func() {
	BeforeEach(func() {
		sess, err := gexec.Start(exec.Command("ip", "netns", "add", fmt.Sprintf("gdn-netnstest-%d", GinkgoParallelNode())), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		sess, err := gexec.Start(exec.Command("ip", "netns", "delete", fmt.Sprintf("gdn-netnstest-%d", GinkgoParallelNode())), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	Describe("Executing a function inside the network namespace", func() {
		It("should be inside the namespace", func() {
			fd, err := os.Open(fmt.Sprintf("/var/run/netns/gdn-netnstest-%d", GinkgoParallelNode()))
			Expect(err).NotTo(HaveOccurred())

			Expect(netns.Exec(fd, func() error {

				link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "banana-iface"}}
				Expect(netlink.LinkAdd(link)).To(Succeed())

				_, err := net.InterfaceByName("banana-iface")
				Expect(err).NotTo(HaveOccurred())
				return nil
			})).To(Succeed())

			_, err = net.InterfaceByName("banana-iface")
			Expect(err).To(HaveOccurred())
		})

		It("bubbles up any errors", func() {
			fd, err := os.Open(fmt.Sprintf("/var/run/netns/gdn-netnstest-%d", GinkgoParallelNode()))
			Expect(err).NotTo(HaveOccurred())

			Expect(
				netns.Exec(fd, func() error { return errors.New("boom") }),
			).To(MatchError("boom"))
		})
	})
})
