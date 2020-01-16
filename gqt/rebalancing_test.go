package gqt_test

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo"
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

		goodCgroupPath = getAbsoluteCPUCgroupPath(config.Tag, "good")
		badCgroupPath = getAbsoluteCPUCgroupPath(config.Tag, "bad")
		config.CPUThrottlingCheckInterval = uint64ptr(1)
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	FIt("starts with all shares allocated to the good cgroup", func() {
		Eventually(func() int64 { return readCgroupFile(goodCgroupPath, "cpu.shares") }).Should(Equal(2))
		Eventually(func() int64 { return readCgroupFile(badCgroupPath, "cpu.shares") }).Should(Equal(2))
	})

	Describe("rebalancing", func() {
		var (
			container               garden.Container
			containerPort           uint32
			containerGoodCgroupPath string
			containerBadCgroupPath  string
		)

		BeforeEach(func() {
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

			containerGoodCgroupPath = ensureInCgroup(container, containerPort, cgroups.GoodCgroupName)
			containerBadCgroupPath = strings.Replace(containerGoodCgroupPath, cgroups.GoodCgroupName, cgroups.BadCgroupName, 1)
			fmt.Println(containerBadCgroupPath)
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
