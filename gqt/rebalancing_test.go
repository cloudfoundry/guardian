package gqt_test

import (
	"fmt"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/sysinfo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CPU shares rebalancing", func() {
	var (
		goodCgroupPath string
		badCgroupPath  string
		client         *runner.RunningGarden
	)

	BeforeEach(func() {
		skipIfNotCPUThrottling()

		// We want an aggressive throttling check to speed moving containers across cgroups up
		// in order to reduce test run time
		config.CPUThrottlingCheckInterval = uint64ptr(1)
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
		var err error
		goodCgroupPath, err = cgrouper.GetCGroupPath(client.CgroupsRootPath(), "cpu", strconv.Itoa(GinkgoParallelProcess()), false, cpuThrottlingEnabled())
		Expect(err).NotTo(HaveOccurred())
		badCgroupPath = filepath.Join(goodCgroupPath, "..", "bad")
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("starts with all shares allocated to the good cgroup", func() {
		Eventually(func() int64 { return readCgroupFile(goodCgroupPath, "cpu.shares") }).Should(BeNumerically(">", 1024))
		Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }).Should(Equal(int64(2)))
	})

	Describe("rebalancing", func() {
		var (
			container               garden.Container
			containerPort           uint32
			goodCgroupInitialShares int64
		)

		JustBeforeEach(func() {
			Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }).Should(Equal(int64(2)))
			goodCgroupInitialShares = readCgroupFile(goodCgroupPath, "cpu.shares")

			var err error
			container, err = client.Create(garden.ContainerSpec{
				Image: garden.ImageRef{URI: "docker:///cfgarden/throttled-or-not"},
				Limits: garden.Limits{
					CPU: garden.CPULimits{
						Weight: 1000,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			containerPort, _, err = container.NetIn(0, 8080)
			Expect(err).NotTo(HaveOccurred())

			_, err = container.Run(garden.ProcessSpec{Path: "/go/src/app/main"}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() (string, error) {
				return httpGet(fmt.Sprintf("http://%s:%d/ping", externalIP(container), containerPort))
			}).Should(Equal("pong"))
		})

		When("the application is punished to the bad cgroup", func() {
			JustBeforeEach(func() {
				Expect(spin(container, containerPort)).To(Succeed())
				ensureInCgroup(container, containerPort, cgroups.BadCgroupName)
			})

			It("redistributes the container shares to the bad cgroup", func() {
				Eventually(func() int64 { return readCgroupFile(goodCgroupPath, "cpu.shares") }).Should(Equal(int64(goodCgroupInitialShares - (1000 - 2))))
				Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }).Should(Equal(int64(1000)))
			})

			When("the application is released back to the good cgroup", func() {
				JustBeforeEach(func() {
					Expect(unspin(container, containerPort)).To(Succeed())
					ensureInCgroup(container, containerPort, cgroups.GoodCgroupName)
				})

				It("redistributes the container shares to the good cgroup", func() {
					Eventually(func() int64 { return readCgroupFile(goodCgroupPath, "cpu.shares") }).Should(Equal(goodCgroupInitialShares))
					Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }).Should(Equal(int64(2)))
				})
			})

			When("cpu-entitlement-per-share is explicitly set", func() {
				BeforeEach(func() {
					resourcesProvider := sysinfo.NewResourcesProvider(config.DepotDir)
					memoryInBytes, err := resourcesProvider.TotalMemory()
					Expect(err).NotTo(HaveOccurred())
					memoryInMbs := memoryInBytes / 1024 / 1024
					cpuCores, err := resourcesProvider.CPUCores()
					Expect(err).NotTo(HaveOccurred())

					defaultEntitlementPerShare := float64(100*cpuCores) / float64(memoryInMbs)
					config.CPUEntitlementPerShare = float64ptr(2 * defaultEntitlementPerShare)
				})

				It("sets the bad cgroup shares proportionally", func() {
					Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }, "5s").Should(BeNumerically("~", 2000, 1))
				})
			})
		})
	})
})

func ensureInCgroup(container garden.Container, containerPort uint32, cgroupType string) string {
	cgroupPath := ""
	EventuallyWithOffset(1, func() (string, error) {
		var err error
		cgroupPath, err = getCgroup(container, containerPort)
		return cgroupPath, err
	}, "2m", "100ms").Should(HaveSuffix(filepath.Join(cgroupType, container.Handle())))

	return getAbsoluteCPUCgroupPath(config.Tag, cgroupPath)
}
