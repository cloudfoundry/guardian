package configure_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/configure"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/lager/v3"
	"github.com/docker/docker/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		netNsName = fmt.Sprintf("my-netns-%d", GinkgoParallelProcess())
		linkName = fmt.Sprintf("my-dummy-link-%d", GinkgoParallelProcess())

		runCommand("ip", "netns", "add", netNsName)

		netNsPath := "/run/netns/" + netNsName
		var err error
		netNsFd, err = os.Open(netNsPath)
		Expect(err).NotTo(HaveOccurred())

		runCommand("ip", "link", "add", linkName, "type", "dummy")
		runCommand("ip", "link", "set", linkName, "netns", netNsName)

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
		runCommand("ip", "netns", "delete", netNsName)
		netNsPath := "/var/run/netns/" + netNsName
		Eventually(netNsPath).ShouldNot(BeAnExistingFile())
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
			netNsFd = tempFile("", "")
			Expect(os.Remove(netNsFd.Name())).To(Succeed())
		})

		It("returns the error", func() {
			err := configurer.Apply(logger, networkConfig, 42)
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("opening netns `%s`", netNsFd.Name()))))
		})
	})

	Context("when the netns path is invalid", func() {
		BeforeEach(func() {
			netNsFd = tempFile("", "")
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
	stdout := runCommand("ip", "netns", "exec", netNsName, "ifconfig", linkName)

	re := regexp.MustCompile(`inet ([0-9.]+)`)

	ret := re.FindStringSubmatch(stdout)
	Expect(ret).NotTo(BeEmpty())

	return ret[1]
}

func linkMTU(netNsName, linkName string) int {
	stdout := runCommand("ip", "netns", "exec", netNsName, "ifconfig", linkName)

	re := regexp.MustCompile(`mtu ([0-9]+)`)

	ret := re.FindStringSubmatch(stdout)
	Expect(ret).NotTo(BeEmpty())

	mtu, err := strconv.ParseInt(ret[1], 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return int(mtu)
}

func linkUp(netNsName, linkName string) bool {
	stdout := runCommand("ip", "netns", "exec", netNsName, "ip", "link", "list", linkName)
	return !strings.Contains(stdout, "DOWN")
}

func linkDefaultGW(netNsName, linkName string) string {
	stdout := runCommand("ip", "netns", "exec", netNsName, "ip", "route", "list", "dev", linkName)

	re := regexp.MustCompile(`default via ([0-9.]+)`)

	ret := re.FindStringSubmatch(stdout)
	Expect(ret).NotTo(BeEmpty())

	return ret[1]
}

func tempFile(dir, prefix string) *os.File {
	f, err := ioutil.TempFile(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return f
}
