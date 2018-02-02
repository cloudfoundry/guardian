package netns_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/kawasaki/netns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

var _ = Describe("NetnsExecerLinux", func() {
	var netnsName string

	BeforeEach(func() {
		netnsName = fmt.Sprintf("gdn-netnstest-%d", GinkgoParallelNode())
	})

	JustBeforeEach(func() {
		Expect(runCommand(exec.Command("ip", "netns", "add", netnsName))).To(Succeed())
	})

	AfterEach(func() {
		Expect(runCommand(exec.Command("ip", "netns", "delete", netnsName))).To(Succeed())
	})

	Describe("Executing a function inside the network namespace", func() {
		It("should be inside the namespace", func() {
			fd, err := os.Open(fmt.Sprintf("/var/run/netns/%s", netnsName))
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
			fd, err := os.Open(fmt.Sprintf("/var/run/netns/%s", netnsName))
			Expect(err).NotTo(HaveOccurred())

			Expect(
				netns.Exec(fd, func() error { return errors.New("boom") }),
			).To(MatchError("boom"))
		})
	})
})

func runCommand(cmd *exec.Cmd) error {
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	return cmd.Run()
}
