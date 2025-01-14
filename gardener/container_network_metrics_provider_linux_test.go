package gardener_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/gardener"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/moby/sys/reexec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("LinuxContainerNetworkMetricsProvider", func() {
	Describe("Get", func() {
		var (
			logger          lager.Logger
			containerizer   *fakes.FakeContainerizer
			propertyManager *fakes.FakePropertyManager

			networkMetricsProvider *gardener.LinuxContainerNetworkMetricsProvider

			netNsName  string
			netNsName2 string
			netNsPath  string
			netNsPath2 string
			netNsFd    *os.File
			linkName   string
			linkName2  string
			handle     string
		)

		BeforeEach(func() {
			var err error
			logger = lagertest.NewTestLogger("test")
			containerizer = new(fakes.FakeContainerizer)
			propertyManager = new(fakes.FakePropertyManager)
			netNsName = fmt.Sprintf("metrics-netns-%d", GinkgoParallelProcess())
			netNsName2 = fmt.Sprintf("metrics-netns2-%d", GinkgoParallelProcess())
			linkName = fmt.Sprintf("metrics-link-%d", GinkgoParallelProcess())
			linkName2 = fmt.Sprintf("metrics-link2-%d", GinkgoParallelProcess())
			netNsPath = "/run/netns/" + netNsName
			netNsPath2 = "/run/netns/" + netNsName2
			handle = "random-handle"

			createNs(netNsName)
			netNsFd, err = os.Open(netNsPath)
			Expect(err).NotTo(HaveOccurred())

			createNs(netNsName2)

			runCommandAssertNoError("ip", "link", "add", linkName, "netns", netNsName, "type", "veth", "peer", "name", linkName2, "netns", netNsName2)

			setupNetworkDevice(linkName, netNsName, "10.0.0.1/24")
			setupNetworkDevice(linkName2, netNsName2, "10.0.0.2/24")

			propertyManager.GetReturns(linkName, true)

			pid := 42
			containerizer.InfoReturns(spec.ActualContainerSpec{
				Pid: pid,
			}, nil)

			networkMetricsProvider = gardener.NewLinuxContainerNetworkMetricsProvider(containerizer, propertyManager, func(path string) (*os.File, error) {
				Expect(path).To(Equal(fmt.Sprintf("/proc/%d/ns/net", pid)))
				return netNsFd, nil
			})

		})

		AfterEach(func() {
			deleteNsIfExists(netNsPath, netNsName)
			deleteNsIfExists(netNsPath2, netNsName2)

			Eventually(netNsPath).ShouldNot(BeAnExistingFile())
			Eventually(netNsPath2).ShouldNot(BeAnExistingFile())
		})

		It("should return network statistics", func() {
			runCommandAssertNoError("ip", "netns", "exec", netNsName, "ping", "10.0.0.2", "-n", "-c", "1", "-s", "4096", "-W", "1")

			// reject second ICMP request to get distinct rx_bytes/tx_bytes numbers
			runCommandAssertNoError("ip", "netns", "exec", netNsName2, "iptables", "-A", "INPUT", "-p", "icmp", "-j", "REJECT")
			err, _ := runCommand("ip", "netns", "exec", netNsName, "ping", "10.0.0.2", "-n", "-c", "1", "-s", "4096", "-W", "1")
			Expect(err).To(HaveOccurred())

			actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
			Expect(err).ToNot(HaveOccurred())

			// since there is some noise on the network interface, a delta of 1000 is accepted
			Expect(actualNetworkMetrics.TxBytes).To(BeNumerically("~", 8192, 1000))
			Expect(actualNetworkMetrics.RxBytes).To(BeNumerically("~", 4096, 1000))
		})

		Context("when the network interface name is not stored in the property manager", func() {
			BeforeEach(func() {
				propertyManager.GetReturns("", false)
			})

			It("should return nil", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})

		Context("when the container info cannot be found", func() {
			BeforeEach(func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{}, errors.New("no-container-info"))
			})

			It("should return an error", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError("no-container-info"))
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})

		Context("network namespace file cannot be found", func() {
			BeforeEach(func() {
				networkMetricsProvider = gardener.NewLinuxContainerNetworkMetricsProvider(containerizer, propertyManager, func(path string) (*os.File, error) {
					return nil, errors.New("no-network-namespace")
				})
			})

			It("should return an error", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError("no-network-namespace"))
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})

		Context("network namespace does not exist", func() {
			BeforeEach(func() {
				deleteNsIfExists(netNsPath, netNsName)
			})

			It("should return an error", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(`could not fetch container network metrics, "opening netns '%[1]s': open %[1]s: no such file or directory"`, netNsPath)))
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})

		Context("network interface does not exist", func() {
			BeforeEach(func() {
				propertyManager.GetReturns("random", true)
			})

			It("should return an error", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(`could not fetch container network metrics, "could not get link 'random'`))
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})
	})

	Describe("Marshal and Unmarshal ContainerNetworkStat", func() {
		var (
			expectedRxBytes                uint64
			expectedTxBytes                uint64
			link                           netlink.Link
			containerNetworkStatMarshaller *gardener.ContainerNetworkStatMarshaller
		)

		BeforeEach(func() {
			expectedRxBytes = 42
			expectedTxBytes = 84

			link = &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{
					Statistics: &netlink.LinkStatistics{
						RxBytes: expectedRxBytes,
						TxBytes: expectedTxBytes,
					},
				},
			}

			containerNetworkStatMarshaller = &gardener.ContainerNetworkStatMarshaller{}
		})

		It("should allow marshalling and unmarshalling network container statistics", func() {
			str := containerNetworkStatMarshaller.MarshalLink(link)
			containerNetworkStat, err := containerNetworkStatMarshaller.Unmarshal(str)
			Expect(err).ToNot(HaveOccurred())
			Expect(containerNetworkStat.RxBytes).To(Equal(expectedRxBytes))
			Expect(containerNetworkStat.TxBytes).To(Equal(expectedTxBytes))
		})

		Context("when an invalid value is unmarshalled", func() {

			It("should return an error", func() {
				str := "random-value"
				containerNetworkStat, err := containerNetworkStatMarshaller.Unmarshal(str)
				Expect(err).To(MatchError(`expected two values but got "random-value"`))
				Expect(containerNetworkStat).To(BeNil())
			})
		})

		Context("when an rx_bytes value is unmarshalled", func() {

			It("should return an error", func() {
				str := ",42"
				containerNetworkStat, err := containerNetworkStatMarshaller.Unmarshal(str)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not parse rx_bytes value"))
				Expect(containerNetworkStat).To(BeNil())
			})
		})

		Context("when an tx_bytes value is unmarshalled", func() {

			It("should return an error", func() {
				str := "42,"
				containerNetworkStat, err := containerNetworkStatMarshaller.Unmarshal(str)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not parse tx_bytes value"))
				Expect(containerNetworkStat).To(BeNil())
			})
		})

	})

})

func runCommand(arg0 string, argv ...string) (error, string) {
	var stdout bytes.Buffer
	cmd := exec.Command(arg0, argv...)
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	return cmd.Run(), stdout.String()
}

func runCommandAssertNoError(arg0 string, argv ...string) string {
	err, s := runCommand(arg0, argv...)
	ExpectWithOffset(1, err).To(Succeed())
	return s
}

func setupNetworkDevice(link, ns, ip string) {
	runCommandAssertNoError("ip", "netns", "exec", ns, "ip", "address", "add", ip, "dev", link)
	runCommandAssertNoError("ip", "netns", "exec", ns, "ip", "link", "set", "dev", link, "up")
}

func createNs(ns string) {
	runCommandAssertNoError("ip", "netns", "add", ns)
}

func deleteNsIfExists(path, ns string) {
	if _, err := os.Open(path); err == nil {
		runCommandAssertNoError("ip", "netns", "del", ns)
	}
}
