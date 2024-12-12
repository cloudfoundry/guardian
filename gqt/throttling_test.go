package gqt_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

var _ = Describe("throttle tests", func() {
	var (
		client        *runner.RunningGarden
		container     garden.Container
		containerPort uint32
	)

	BeforeEach(func() {
		skipIfNotCPUThrottling()

		// We want an aggressive throttling check to speed moving containers across cgroups up
		// in order to reduce test run time
		config.CPUThrottlingCheckInterval = uint64ptr(1)
		client = runner.Start(config)

		var err error
		container, err = client.Create(garden.ContainerSpec{
			Limits: garden.Limits{
				CPU: garden.CPULimits{
					Weight: 1000,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		containerPort, _, err = container.NetIn(0, 8080)
		Expect(err).NotTo(HaveOccurred())

		_, err = container.Run(garden.ProcessSpec{Path: "/bin/throttled-or-not"}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (string, error) {
			return httpGet(fmt.Sprintf("http://%s:%d/ping", externalIP(container), containerPort))
		}).Should(Equal("pong"))
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	ensureInCgroup := func(cgroupType string) string {
		cgroupPath := ""
		EventuallyWithOffset(1, func() (string, error) {
			var err error
			cgroupPath, err = getCgroup(container, containerPort)
			return cgroupPath, err
		}, "2m", "100ms").Should(ContainSubstring(filepath.Join(cgroupType, container.Handle())))

		return getAbsoluteCPUCgroupPath(config.Tag, cgroupPath)
	}

	It("will create both a good and a bad cgroup for that container", func() {
		goodCgroupPath := ensureInCgroup(gardencgroups.GoodCgroupName)
		badCgroup := strings.Replace(goodCgroupPath, gardencgroups.GoodCgroupName, gardencgroups.BadCgroupName, 1)
		if cgroups.IsCgroup2UnifiedMode() {
			// only main process is moved to bad cgroup, other peas are left in good cgroup
			// in cgroups v2 main process is in init folder
			badCgroup = strings.TrimSuffix(badCgroup, "init")
		}
		Expect(badCgroup).To(BeAnExistingFile())
	})

	It("will eventually move the app to the bad cgroup", func() {
		ensureInCgroup(gardencgroups.GoodCgroupName)
		Expect(spin(container, containerPort)).To(Succeed())
		ensureInCgroup(gardencgroups.BadCgroupName)
	})

	It("preserves the container shares in the bad cgroup", func() {
		goodCgroupPath := ensureInCgroup(gardencgroups.GoodCgroupName)
		Expect(spin(container, containerPort)).To(Succeed())
		badCgroupPath := ensureInCgroup(gardencgroups.BadCgroupName)

		cpuSharesFile := "cpu.shares"
		if cgroups.IsCgroup2UnifiedMode() {
			cpuSharesFile = "cpu.weight"
		}
		goodShares := readCgroupFile(goodCgroupPath, cpuSharesFile)
		badShares := readCgroupFile(badCgroupPath, cpuSharesFile)
		Expect(goodShares).To(Equal(badShares))
	})

	It("will delete the bad cgroup after the container gets destroyed", func() {
		currentCgroupSubpath, err := getCgroup(container, containerPort)
		Expect(err).NotTo(HaveOccurred())

		currentCgroupPath := getAbsoluteCPUCgroupPath(config.Tag, currentCgroupSubpath)

		badCgroup := strings.Replace(currentCgroupPath, gardencgroups.GoodCgroupName, gardencgroups.BadCgroupName, 1)

		Expect(client.Destroy(container.Handle())).To(Succeed())
		Expect(badCgroup).NotTo(BeAnExistingFile())
	})

	It("CPU metrics are combined from the good and bad cgroup", func() {
		goodCgroupPath := ensureInCgroup(gardencgroups.GoodCgroupName)
		// Spinning the app should stop updating the usage in the good cgroup
		Expect(spin(container, containerPort)).To(Succeed())

		ensureInCgroup(gardencgroups.BadCgroupName)

		var goodCgroupUsage int64
		if cgroups.IsCgroup2UnifiedMode() {
			goodCgroupUsage = readCgroupV2CPUUsage(goodCgroupPath)
		} else {
			goodCgroupUsage = readCgroupFile(goodCgroupPath, "cpuacct.usage")
		}
		// Usage should be bigger than just the value in the metrics
		Eventually(func() uint64 {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())
			return metrics.CPUStat.Usage
		}).Should(BeNumerically(">", goodCgroupUsage))
	})

	When("a bad application starts behaving nicely again", func() {
		BeforeEach(func() {
			Expect(spin(container, containerPort)).To(Succeed())
			ensureInCgroup(gardencgroups.BadCgroupName)
			Expect(unspin(container, containerPort)).To(Succeed())
		})

		It("will eventually move the app to the good cgroup", func() {
			ensureInCgroup(gardencgroups.GoodCgroupName)
		})
	})
})

func getCgroup(container garden.Container, containerPort uint32) (string, error) {
	cgroup, err := httpGet(fmt.Sprintf("http://%s:%d/cpucgroup", externalIP(container), containerPort))
	if err != nil {
		return "", fmt.Errorf("cpucgroup failed: %+v", err)
	}

	return cgroup, nil
}

func spin(container garden.Container, containerPort uint32) error {
	if _, err := httpGet(fmt.Sprintf("http://%s:%d/spin", externalIP(container), containerPort)); err != nil {
		return fmt.Errorf("spin failed: %+v", err)
	}

	return nil
}

func unspin(container garden.Container, containerPort uint32) error {
	if _, err := httpGet(fmt.Sprintf("http://%s:%d/unspin", externalIP(container), containerPort)); err != nil {
		return fmt.Errorf("unspin failed: %+v", err)
	}

	return nil
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

func getAbsoluteCPUCgroupPath(tag, cgroupSubPath string) string {
	cgroupMountpoint := fmt.Sprintf("/tmp/cgroups-%s", tag)
	if cgroups.IsCgroup2UnifiedMode() {
		return filepath.Join(cgroupMountpoint, gardencgroups.Unified, cgroupSubPath)
	}
	return filepath.Join(cgroupMountpoint, "cpu", cgroupSubPath)
}

func readCgroupFile(cgroupPath, file string) int64 {
	usageContent, err := os.ReadFile(filepath.Join(cgroupPath, file))
	Expect(err).NotTo(HaveOccurred())

	usage, err := strconv.ParseInt(strings.TrimSpace(string(usageContent)), 10, 64)
	Expect(err).NotTo(HaveOccurred())

	return usage
}

func readCgroupV2CPUUsage(cgroupPath string) int64 {
	statContents, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.stat"))
	Expect(err).NotTo(HaveOccurred())
	r, err := regexp.Compile("usage_usec (.*)\n")
	Expect(err).NotTo(HaveOccurred())
	matches := r.FindStringSubmatch(string(statContents))
	Expect(matches).To(HaveLen(2))
	usage, err := strconv.ParseInt(matches[1], 10, 64)
	Expect(err).NotTo(HaveOccurred())

	// usage_usec is ms, return value in ns, see opencontainers/runc/libcontainer/cgroups/fs2/cpu.go
	return usage * 1000
}
