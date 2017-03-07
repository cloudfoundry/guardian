package mtu_test

import (
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/guardian/kawasaki/mtu"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MTU", func() {
	It("returns the MTU for a given IP address", func() {
		iface, err := net.InterfaceByIndex(1)
		Expect(err).NotTo(HaveOccurred())
		ifconfigOutput, err := exec.Command("ifconfig", iface.Name).CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		groups := regexp.MustCompile(`(MTU|mtu).(\d+)`).FindStringSubmatch(string(ifconfigOutput))
		mtuStr := groups[2]
		ifaceMtu, err := strconv.Atoi(mtuStr)
		Expect(err).NotTo(HaveOccurred())

		addrs, err := iface.Addrs()
		Expect(err).NotTo(HaveOccurred())
		ipAddr := strings.Split(addrs[0].String(), "/")[0]

		unit, err := mtu.MTU(ipAddr)
		Expect(err).NotTo(HaveOccurred())
		Expect(unit).To(Equal(ifaceMtu))
	})
})
