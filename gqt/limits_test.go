package gqt_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Limits", func() {
	var (
		client     *runner.RunningGarden
		container  garden.Container
		cgroupPath string
		cgroupType string
		limits     garden.Limits
		privileged bool
	)

	BeforeEach(func() {
		privileged = false
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
		var err error
		container, err = client.Create(garden.ContainerSpec{
			Limits:     limits,
			Privileged: privileged,
		})
		Expect(err).NotTo(HaveOccurred())

		parentPath, err := cgrouper.GetCGroupPath(client.CgroupsRootPath(), cgroupType, config.Tag, privileged, cpuThrottlingEnabled())
		Expect(err).NotTo(HaveOccurred())
		cgroupPath = filepath.Join(parentPath, container.Handle())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("CPU Limits", func() {
		BeforeEach(func() {
			limits = garden.Limits{CPU: garden.CPULimits{LimitInShares: 128}}
			cgroupType = "cpu"
		})

		Context("when started with low cpu limit turned on", func() {
			BeforeEach(func() {
				config.CPUQuotaPerShare = uint64ptr(10)
			})

			Context("when a container with cpu limits is created", func() {
				It("throttles process cpu usage", func() {
					periods, throttled, time, err := parseCpuStats(filepath.Join(cgroupPath, "cpu.stat"))
					Expect(err).NotTo(HaveOccurred())
					Expect(periods).To(BeNumerically(">", 0))
					Expect(throttled).To(BeNumerically(">", 0))
					Expect(time).To(BeNumerically(">", 0))
				})

				It("sets cpu.cfs_period_us to 100000 (100ms)", func() {
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_period_us"))
					Expect(strings.TrimSpace(period)).To(Equal("100000"))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("1280"))
				})
			})
		})

		Context("when started with low cpu limit turned off", func() {
			Context("when when a container with cpu limits is created", func() {
				It("does not throttle process cpu usage", func() {
					periods, throttled, time, err := parseCpuStats(filepath.Join(cgroupPath, "cpu.stat"))
					Expect(err).NotTo(HaveOccurred())
					Expect(periods).To(BeNumerically("==", 0))
					Expect(throttled).To(BeNumerically("==", 0))
					Expect(time).To(BeNumerically("==", 0))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("-1"))
				})
			})
		})
	})

	Describe("device restrictions", func() {
		BeforeEach(func() {
			cgroupType = "devices"
		})

		itAllowsOnlyCertainDevices := func(privileged bool) {
			It("only allows certain devices", func() {
				content := readFileString(filepath.Join(cgroupPath, "devices.list"))
				expectedAllowedDevices := []string{
					"c 1:3 rwm",
					"c 5:0 rwm",
					"c 1:8 rwm",
					"c 1:9 rwm",
					"c 1:5 rwm",
					"c 1:7 rwm",
					"c *:* m",
					"b *:* m",
					"c 136:* rwm",
					"c 5:2 rwm",
					"c 10:200 rwm",
				}

				if privileged {
					expectedAllowedDevices = append(expectedAllowedDevices, "c 10:229 rwm")
				}

				contentLines := strings.Split(strings.TrimSpace(content), "\n")
				Expect(contentLines).To(HaveLen(len(expectedAllowedDevices)))
				Expect(contentLines).To(ConsistOf(expectedAllowedDevices))
			})
		}

		itAllowsOnlyCertainDevices(false)

		Context("in a privileged container", func() {
			BeforeEach(func() {
				privileged = true
			})

			itAllowsOnlyCertainDevices(true)
		})
	})
})

func parseCpuStats(statFilePath string) (int, int, int, error) {
	statFile, err := os.Open(statFilePath)
	if err != nil {
		return -1, -1, -1, err
	}

	var periods, throttled, time int = -1, -1, -1

	scanner := bufio.NewScanner(statFile)
	for scanner.Scan() {
		var (
			key   string
			value int
		)
		if _, err := fmt.Sscanf(scanner.Text(), "%s %d", &key, &value); err != nil {
			return -1, -1, -1, err
		}
		switch key {
		case "nr_periods":
			periods = value
		case "nr_throttled":
			throttled = value
		case "throttled_time":
			time = value
		}
	}

	return periods, throttled, time, nil
}
