package throttle_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/containerd/cgroups/v3/cgroup2"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

var _ = Describe("Enforcer", func() {
	var (
		logger            *lagertest.TestLogger
		handle            string
		cgroupRoot        string
		cpuCgroupPath     string
		command           *exec.Cmd
		expectedCPUShares int
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("enforcer-test")
		uuid, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())
		handle = uuid.String()

		cgroupRoot, err = os.MkdirTemp("", "cgroups")
		Expect(err).NotTo(HaveOccurred())

		mountCPUcgroup(cgroupRoot)
		cpuCgroupPath = filepath.Join(cgroupRoot, "cpu")

		expectedCPUShares = 3456
		if cgroups.IsCgroup2UnifiedMode() {
			expectedCPUShares = int(cgroups.ConvertCPUSharesToCgroupV2Value(3456))
		}
	})

	AfterEach(func() {
		Expect(command.Process.Kill()).To(Succeed())
		_, err := command.Process.Wait()
		Expect(err).NotTo(HaveOccurred())
		umountCgroups(cgroupRoot)
	})

	Describe("Punish", func() {
		var (
			punishErr error
		)

		JustBeforeEach(func() {
			enforcer := throttle.NewEnforcer(cpuCgroupPath)
			punishErr = enforcer.Punish(logger, handle)
		})

		Context("containers that have been created after cpu throttling enablement", func() {
			var (
				goodCgroup          string
				goodContainerCgroup string
				badCgroup           string
				badContainerCgroup  string
			)

			BeforeEach(func() {
				goodCgroup = filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName)
				goodContainerCgroup = filepath.Join(goodCgroup, handle)
				makeSubCgroup(goodCgroup, handle)

				badCgroup = filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName)
				badContainerCgroup = filepath.Join(badCgroup, handle)
				makeSubCgroup(badCgroup, handle)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

			})

			Context("when good cgroup doesn't have child init cgroup", func() {
				BeforeEach(func() {
					writeShares(goodContainerCgroup, 3456)
					Expect(cgroups.WriteCgroupProc(goodContainerCgroup, command.Process.Pid)).To(Succeed())
				})

				It("moves the process to the bad cgroup", func() {
					Expect(punishErr).NotTo(HaveOccurred())

					pids, err := cgroups.GetPids(goodContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(BeEmpty())

					pids, err = cgroups.GetPids(badContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(ContainElement(command.Process.Pid))
				})

				It("copies CPU shares to the bad container cgroup", func() {
					badContainerShares := readCPUShares(badContainerCgroup)
					Expect(badContainerShares).To(Equal(expectedCPUShares))
				})
			})

			Context("when good cgroup has init child cgroup", func() {
				var initCgroupPath string

				BeforeEach(func() {
					makeSubCgroup(goodContainerCgroup, "init")
					initCgroupPath = filepath.Join(goodContainerCgroup, "init")
					writeShares(initCgroupPath, 7890)
					Expect(cgroups.WriteCgroupProc(initCgroupPath, command.Process.Pid)).To(Succeed())
				})

				It("moves the process to the bad cgroup", func() {
					Expect(punishErr).NotTo(HaveOccurred())

					pids, err := cgroups.GetPids(goodContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(BeEmpty())

					pids, err = cgroups.GetPids(badContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(ContainElement(command.Process.Pid))
				})

				It("copies CPU shares to the bad container cgroup", func() {
					badContainerShares := readCPUShares(badContainerCgroup)
					Expect(badContainerShares).To(Equal(int(cgroups.ConvertCPUSharesToCgroupV2Value(7890))))
				})
			})
		})

		Context("containers that have been created before throttling feature was enabled", func() {
			var (
				containerCgroup string
			)

			BeforeEach(func() {
				containerCgroup = filepath.Join(cpuCgroupPath, handle)
				makeSubCgroup(cpuCgroupPath, handle)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

				Expect(cgroups.WriteCgroupProc(containerCgroup, command.Process.Pid)).To(Succeed())
			})

			It("does not move the container to another cgroup", func() {
				Expect(punishErr).NotTo(HaveOccurred())
				pids, err := cgroups.GetPids(containerCgroup)
				Expect(err).NotTo(HaveOccurred())
				Expect(pids).To(ContainElement(command.Process.Pid))
			})
		})
	})

	Describe("Release", func() {
		var (
			releaseErr error
		)

		JustBeforeEach(func() {
			enforcer := throttle.NewEnforcer(cpuCgroupPath)
			releaseErr = enforcer.Release(logger, handle)
		})

		Context("containers that have been created after cpu throttling enablement", func() {
			var (
				goodCgroup            string
				goodContainerCgroup   string
				badCgroup             string
				badContainerCgroup    string
				expectedGoodCPUShares int
			)

			BeforeEach(func() {
				goodCgroup = filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName)
				goodContainerCgroup = filepath.Join(goodCgroup, handle)
				makeSubCgroup(goodCgroup, handle)

				writeShares(goodContainerCgroup, 6543)
				expectedGoodCPUShares = 6543
				if cgroups.IsCgroup2UnifiedMode() {
					expectedGoodCPUShares = int(cgroups.ConvertCPUSharesToCgroupV2Value(uint64(6543)))
				}

				badCgroup = filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName)
				badContainerCgroup = filepath.Join(badCgroup, handle)
				makeSubCgroup(badCgroup, handle)

				writeShares(badContainerCgroup, 3456)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

				Expect(cgroups.WriteCgroupProc(badContainerCgroup, command.Process.Pid)).To(Succeed())
			})

			Context("when good cgroup doesn't have child init cgroup", func() {
				It("moves the process to the good cgroup", func() {
					Expect(releaseErr).NotTo(HaveOccurred())

					pids, err := cgroups.GetPids(badContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(BeEmpty())

					pids, err = cgroups.GetPids(goodContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(ContainElement(command.Process.Pid))
				})

				It("preserves CPU shares in the good container cgroup", func() {
					goodContainerShares := readCPUShares(goodContainerCgroup)
					Expect(goodContainerShares).To(Equal(expectedGoodCPUShares))
				})

				It("preserves CPU shares in the bad container cgroup", func() {
					badContainerShares := readCPUShares(badContainerCgroup)
					Expect(badContainerShares).To(Equal(expectedCPUShares))
				})
			})

			Context("when good cgroup has init child cgroup", func() {
				var initCgroupPath string

				BeforeEach(func() {
					makeSubCgroup(goodContainerCgroup, "init")
					initCgroupPath = filepath.Join(goodContainerCgroup, "init")
					writeShares(initCgroupPath, 6543)
					Expect(cgroups.WriteCgroupProc(initCgroupPath, command.Process.Pid)).To(Succeed())
				})

				It("moves the process to the good init cgroup", func() {
					Expect(releaseErr).NotTo(HaveOccurred())

					pids, err := cgroups.GetPids(badContainerCgroup)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(BeEmpty())

					pids, err = cgroups.GetPids(initCgroupPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(pids).To(ContainElement(command.Process.Pid))
				})

				It("preserves CPU shares in the good container cgroup", func() {
					goodContainerInitShares := readCPUShares(initCgroupPath)
					Expect(goodContainerInitShares).To(Equal(expectedGoodCPUShares))
				})

				It("preserves CPU shares in the bad container cgroup", func() {
					badContainerShares := readCPUShares(badContainerCgroup)
					Expect(badContainerShares).To(Equal(expectedCPUShares))
				})
			})
		})

		Context("containers that have been created before throttling feature was enabled", func() {
			var (
				containerCgroup string
			)

			BeforeEach(func() {
				containerCgroup = filepath.Join(cpuCgroupPath, handle)
				makeSubCgroup(cpuCgroupPath, handle)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

				Expect(cgroups.WriteCgroupProc(containerCgroup, command.Process.Pid)).To(Succeed())
			})

			It("does not move the container to another cgroup", func() {
				Expect(releaseErr).NotTo(HaveOccurred())
				pids, err := cgroups.GetPids(containerCgroup)
				Expect(err).NotTo(HaveOccurred())
				Expect(pids).To(ContainElement(command.Process.Pid))
			})
		})
	})
})

func makeSubCgroup(root string, path string) {
	Expect(os.MkdirAll(path, 0755)).To(Succeed())
	if cgroups.IsCgroup2UnifiedMode() {
		_, err := cgroup2.NewManager(root, "/"+path, &cgroup2.Resources{CPU: &cgroup2.CPU{}})
		Expect(err).NotTo(HaveOccurred())
	}
}

func readCPUShares(cgroupPath string) int {
	cpuSharesFile := "cpu.shares"
	if cgroups.IsCgroup2UnifiedMode() {
		cpuSharesFile = "cpu.weight"
	}
	shareBytes, err := os.ReadFile(filepath.Join(cgroupPath, cpuSharesFile))
	Expect(err).NotTo(HaveOccurred())
	shares, err := strconv.Atoi(strings.TrimSpace(string(shareBytes)))
	Expect(err).NotTo(HaveOccurred())
	return shares
}

func mountCPUcgroup(cgroupRoot string) {
	if cgroups.IsCgroup2UnifiedMode() {
		Expect(syscall.Mount("cgroup2", cgroupRoot, "tmpfs", uintptr(0), "mode=0755")).To(Succeed())

		cpuCgroup := filepath.Join(cgroupRoot, "cpu")
		Expect(os.MkdirAll(cpuCgroup, 0755)).To(Succeed())

		Expect(syscall.Mount("cgroup2", cpuCgroup, "cgroup2", 0, "")).To(Succeed())

		_, err := cgroup2.NewManager(cpuCgroup, "/", &cgroup2.Resources{})
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(syscall.Mount("cgroup", cgroupRoot, "tmpfs", uintptr(0), "mode=0755")).To(Succeed())

		cpuCgroup := filepath.Join(cgroupRoot, "cpu")
		Expect(os.MkdirAll(cpuCgroup, 0755)).To(Succeed())

		Expect(syscall.Mount("cgroup", cpuCgroup, "cgroup", uintptr(0), "cpu,cpuacct")).To(Succeed())
	}
}

func umountCgroups(cgroupRoot string) {
	cpuCgroup := filepath.Join(cgroupRoot, "cpu")
	Expect(os.RemoveAll(filepath.Join(cpuCgroup, gardencgroups.Garden))).To(Succeed())
	Expect(syscall.Unmount(cpuCgroup, 0)).To(Succeed())
	Expect(syscall.Unmount(cgroupRoot, 0)).To(Succeed())
}

func writeShares(path string, shares int) {
	cpuSharesFile := "cpu.shares"
	if cgroups.IsCgroup2UnifiedMode() {
		cpuSharesFile = "cpu.weight"
		shares = int(cgroups.ConvertCPUSharesToCgroupV2Value(uint64(shares)))
	}
	Expect(os.WriteFile(filepath.Join(path, cpuSharesFile), []byte(strconv.Itoa(shares)), 0644)).To(Succeed())
}
