package gardener_test

import (
	"errors"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/gardener"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SysFSContainerNetworkMetricsProvider", func() {
	var (
		logger          lager.Logger
		containerizer   *fakes.FakeContainerizer
		propertyManager *fakes.FakePropertyManager

		networkMetricsProvider *gardener.SysFSContainerNetworkMetricsProvider
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		containerizer = new(fakes.FakeContainerizer)
		propertyManager = new(fakes.FakePropertyManager)

		networkMetricsProvider = gardener.NewSysFSContainerNetworkMetricsProvider(containerizer, propertyManager)
	})

	Describe("Get", func() {
		var (
			handle      string
			ifName      string
			networkStat garden.ContainerNetworkStat

			networkStatProcess *gardenfakes.FakeProcess
		)
		BeforeEach(func() {
			handle = "random-handle"
			ifName = "random-eth"

			networkStat = garden.ContainerNetworkStat{
				RxBytes: 42,
				TxBytes: 43,
			}

			propertyManager.GetReturnsOnCall(0, ifName, true)

			networkStatProcess = new(gardenfakes.FakeProcess)
			networkStatProcess.WaitReturns(0, nil)

			containerizer.RunCalls(func(logger lager.Logger, s string, processSpec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				_, _ = io.Stdout.Write([]byte(fmt.Sprintf("%d\n%d\n", networkStat.RxBytes, networkStat.TxBytes)))
				return networkStatProcess, nil
			})
		})

		It("should return network statistics", func() {
			actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.RunCallCount()).To(Equal(1))
			_, _, spec, _ := containerizer.RunArgsForCall(0)

			Expect(spec.Path).To(Equal("cat"))

			Expect(spec.Args).To(Equal([]string{
				filepath.Join("/sys/class/net/", ifName, "/statistics/rx_bytes"),
				filepath.Join("/sys/class/net/", ifName, "/statistics/tx_bytes"),
			}))

			Expect(actualNetworkMetrics.TxBytes).To(Equal(networkStat.TxBytes))
			Expect(actualNetworkMetrics.RxBytes).To(Equal(networkStat.RxBytes))
		})

		Context("when the process execution to fetch the network statistics fails", func() {
			BeforeEach(func() {
				containerizer.RunReturns(nil, errors.New("processError"))
			})

			It("should propagate the error", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring("processError")))
			})
		})

		Context("when waiting for the process execution to fetch the network statistics fails", func() {
			BeforeEach(func() {
				networkStatProcess.WaitReturns(-1, errors.New("waitError"))
			})

			It("should propagate the error", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring("waitError")))
			})
		})

		Context("when the process execution to fetch the network statistics returns an exit status not equal to 0", func() {
			BeforeEach(func() {
				containerizer.RunCalls(func(logger lager.Logger, s string, processSpec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
					_, _ = io.Stderr.Write([]byte("randomStderr"))
					return networkStatProcess, nil
				})
				networkStatProcess.WaitReturns(42, nil)
			})

			It("should return an error that contains the exit status and stderr output", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring("42")))
				Expect(err).To(MatchError(ContainSubstring("randomStderr")))
			})
		})

		Context("when network statistics are missing", func() {
			BeforeEach(func() {
				containerizer.RunReturns(networkStatProcess, nil)
			})

			It("should return an error", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring(`expected two values but got ""`)))
			})
		})

		Context("when the rx_bytes value cannot be parsed", func() {
			BeforeEach(func() {
				containerizer.RunCalls(func(logger lager.Logger, s string, processSpec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
					_, _ = io.Stdout.Write([]byte("abc\n42\n"))
					return networkStatProcess, nil
				})
			})

			It("should return an error", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring("could not parse rx_bytes value")))
			})
		})

		Context("when the tx_bytes value cannot be parsed", func() {
			BeforeEach(func() {
				containerizer.RunCalls(func(logger lager.Logger, s string, processSpec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
					_, _ = io.Stdout.Write([]byte("42\nabc\n"))
					return networkStatProcess, nil
				})
			})

			It("should return an error", func() {
				_, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).To(MatchError(ContainSubstring("could not parse tx_bytes value")))
			})
		})

		Context("when the network interface name is not stored in the property manager", func() {
			BeforeEach(func() {
				propertyManager.GetReturnsOnCall(0, "", false)
			})

			It("should return nil", func() {
				actualNetworkMetrics, err := networkMetricsProvider.Get(logger, handle)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualNetworkMetrics).To(BeNil())
			})
		})
	})

})
