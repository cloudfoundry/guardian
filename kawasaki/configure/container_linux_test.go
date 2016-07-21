package configure_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/configure"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/docker/docker/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("Container", func() {
	var (
		netNsName string
		netNsFd   *os.File
		linkName  string

		configurer    *configure.Container
		logger        lager.Logger
		networkConfig kawasaki.NetworkConfig
	)

	BeforeEach(func() {
		netNsName = fmt.Sprintf("my-netns-%d", GinkgoParallelNode())
		linkName = fmt.Sprintf("my-dummy-link-%d", GinkgoParallelNode())

		sess, err := gexec.Start(exec.Command("ip", "netns", "add", netNsName), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		netNsPath := fmt.Sprintf("/run/netns/%s", netNsName)
		netNsFd, err = os.Open(netNsPath)
		Expect(err).NotTo(HaveOccurred())

		sess, err = gexec.Start(exec.Command("ip", "link", "add", linkName, "type", "dummy"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		sess, err = gexec.Start(exec.Command("ip", "link", "set", linkName, "netns", netNsName), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		configurer = &configure.Container{
			FileOpener: netns.Opener(func(path string) (*os.File, error) {
				return netNsFd, nil
			}),
		}

		logger = lagertest.NewTestLogger("container")

		containerIP, subnet, err := net.ParseCIDR("192.0.2.20/24")
		Expect(err).NotTo(HaveOccurred())
		networkConfig = kawasaki.NetworkConfig{
			ContainerIntf: linkName,
			ContainerIP:   containerIP,
			BridgeIP:      net.ParseIP("192.0.2.1"),
			Subnet:        subnet,
			Mtu:           1600,
		}
	})

	AfterEach(func() {
		sess, err := gexec.Start(exec.Command("ip", "netns", "delete", netNsName), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	It("sets the container IP", func() {
		Expect(configurer.Apply(logger, networkConfig, 42)).To(Succeed())

		Expect(linkIP(netNsName, linkName)).To(Equal(networkConfig.ContainerIP.String()))
	})

	It("brings the link up", func() {
		Expect(configurer.Apply(logger, networkConfig, 42)).To(Succeed())

		Expect(linkUp(netNsName, linkName)).To(BeTrue())
	})

	It("sets the default gateway", func() {
		Expect(configurer.Apply(logger, networkConfig, 42)).To(Succeed())

		Expect(linkDefaultGW(netNsName, linkName)).To(Equal(networkConfig.BridgeIP.String()))
	})

	It("sets the MTU", func() {
		Expect(configurer.Apply(logger, networkConfig, 42)).To(Succeed())

		Expect(linkMTU(netNsName, linkName)).To(Equal(networkConfig.Mtu))
	})

	Context("when the netns file disappears", func() {
		BeforeEach(func() {
			var err error
			netNsFd, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Remove(netNsFd.Name())).To(Succeed())
		})

		It("returns the error", func() {
			err := configurer.Apply(logger, networkConfig, 42)
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("opening netns `%s`", netNsFd.Name()))))
		})
	})

	Context("when the netns path is invalid", func() {
		BeforeEach(func() {
			var err error
			netNsFd, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the error", func() {
			err := configurer.Apply(logger, networkConfig, 42)
			Expect(err).To(MatchError(ContainSubstring("set netns")))
		})
	})

	Context("when the container interface does not exist", func() {
		BeforeEach(func() {
			networkConfig.ContainerIntf = "not-a-link"
		})

		It("returns the error", func() {
			err := configurer.Apply(logger, networkConfig, 42)
			Expect(err).To(MatchError("interface `not-a-link` was not found"))
		})
	})
})

func linkIP(netNsName, linkName string) string {
	cmd := exec.Command("ip", "netns", "exec", netNsName, "ifconfig", linkName)

	buffer := gbytes.NewBuffer()
	sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	re, err := regexp.Compile(`inet addr:([0-9.]+)`)
	Expect(err).NotTo(HaveOccurred())

	ret := re.FindStringSubmatch(string(buffer.Contents()))
	Expect(ret).NotTo(BeEmpty())

	return ret[1]
}

func linkMTU(netNsName, linkName string) int {
	cmd := exec.Command("ip", "netns", "exec", netNsName, "ifconfig", linkName)

	buffer := gbytes.NewBuffer()
	sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	re, err := regexp.Compile(`MTU:([0-9]+)`)
	Expect(err).NotTo(HaveOccurred())

	ret := re.FindStringSubmatch(string(buffer.Contents()))
	Expect(ret).NotTo(BeEmpty())

	mtu, err := strconv.ParseInt(ret[1], 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return int(mtu)
}

func linkUp(netNsName, linkName string) bool {
	cmd := exec.Command("ip", "netns", "exec", netNsName, "ip", "link", "list", linkName)

	buffer := gbytes.NewBuffer()
	sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	return !strings.Contains(string(buffer.Contents()), "DOWN")
}

func linkDefaultGW(netNsName, linkName string) string {
	cmd := exec.Command("ip", "netns", "exec", netNsName, "ip", "route", "list", "dev", linkName)

	buffer := gbytes.NewBuffer()
	sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	re, err := regexp.Compile(`default via ([0-9.]+)`)
	Expect(err).NotTo(HaveOccurred())

	ret := re.FindStringSubmatch(string(buffer.Contents()))
	Expect(ret).NotTo(BeEmpty())

	return ret[1]
}
