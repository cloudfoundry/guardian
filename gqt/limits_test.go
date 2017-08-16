package gqt_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Limits", func() {
	var (
		client     *runner.RunningGarden
		container  garden.Container
		cgroupPath string
		cgroupName string
		cgroupType string
		limits     garden.Limits
	)

	JustBeforeEach(func() {
		client = runner.Start(config)
		var err error
		container, err = client.Create(garden.ContainerSpec{
			Limits: limits,
		})
		Expect(err).NotTo(HaveOccurred())

		currentCgroup, err := exec.Command("sh", "-c", "cat /proc/self/cgroup | head -1 | awk -F ':' '{print $3}'").CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		cgroupName = strings.TrimSpace(string(currentCgroup))

		cgroupPath = fmt.Sprintf("%s/cgroups-%s/%s/%s/garden-%s/%s", client.TmpDir, config.Tag, cgroupType,
			cgroupName, config.Tag, container.Handle())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("TCP Mem Limits", func() {
		const tcpMemDefault = "9223372036854771712"

		var (
			tcpMemLimit string
		)

		BeforeEach(func() {
			limits = garden.Limits{}
			cgroupType = "memory"
		})

		JustBeforeEach(func() {
			memLimitBytes := readFile(filepath.Join(cgroupPath, "memory.kmem.tcp.limit_in_bytes"))
			tcpMemLimit = strings.TrimSpace(memLimitBytes)
		})

		Context("when starting the server with --tcp-memory-limit set to 0", func() {
			It("does not explicitly set memory.kmem.tcp.limit_in_bytes and uses the default value instead", func() {
				Expect(tcpMemLimit).To(Equal(tcpMemDefault))
			})
		})

		Context("when starting the server with --tcp-memory-limit set to > 0", func() {
			BeforeEach(func() {
				config.TCPMemoryLimit = uint64ptr(212992)
			})

			It("sets memory.kmem.tcp.limit_in_bytes to the provided value", func() {
				Expect(tcpMemLimit).To(Equal("212992"))
			})
		})
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
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_period_us"))
					Expect(strings.TrimSpace(period)).To(Equal("100000"))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("1280"))
				})
			})
		})

		Context("when started with low cpu limit turned off", func() {
			Context("when when a container with cpu limits is created", func() {
				It("throttles process cpu usage", func() {
					periods, throttled, time, err := parseCpuStats(filepath.Join(cgroupPath, "cpu.stat"))
					Expect(err).NotTo(HaveOccurred())
					Expect(periods).To(BeNumerically("==", 0))
					Expect(throttled).To(BeNumerically("==", 0))
					Expect(time).To(BeNumerically("==", 0))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("-1"))
				})
			})
		})
	})

	Describe("device restrictions", func() {
		BeforeEach(func() {
			cgroupType = "devices"
		})

		It("allows only certain devices", func() {
			expectedAllowedDevices := `c 1:3 rwm
c 5:0 rwm
c 1:8 rwm
c 1:9 rwm
c 1:5 rwm
c 1:7 rwm
c 10:229 rwm
c *:* m
b *:* m
c 5:1 rwm
c 136:* rwm
c 5:2 rwm
c 10:200 rwm
`

			allowedDevices := readFile(filepath.Join(cgroupPath, "devices.list"))
			Expect(allowedDevices).To(Equal(expectedAllowedDevices))
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
