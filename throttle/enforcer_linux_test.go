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
		logger        *lagertest.TestLogger
		handle        string
		cgroupRoot    string
		cpuCgroupPath string
		command       *exec.Cmd
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

				writeShares(goodContainerCgroup, 3456)

				badCgroup = filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName)
				badContainerCgroup = filepath.Join(badCgroup, handle)
				makeSubCgroup(badCgroup, handle)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

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
				Expect(badContainerShares).To(Equal(3456))
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
				goodCgroup          string
				goodContainerCgroup string
				badCgroup           string
				badContainerCgroup  string
			)

			BeforeEach(func() {
				goodCgroup = filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName)
				goodContainerCgroup = filepath.Join(goodCgroup, handle)
				makeSubCgroup(goodCgroup, handle)

				writeShares(goodContainerCgroup, 6543)

				badCgroup = filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName)
				badContainerCgroup = filepath.Join(badCgroup, handle)
				makeSubCgroup(badCgroup, handle)

				writeShares(badContainerCgroup, 3456)

				command = exec.Command("sleep", "360")
				Expect(command.Start()).To(Succeed())

				Expect(cgroups.WriteCgroupProc(badContainerCgroup, command.Process.Pid)).To(Succeed())
			})

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
				badContainerShares := readCPUShares(goodContainerCgroup)
				Expect(badContainerShares).To(Equal(6543))
			})

			It("preserves CPU shares in the bad container cgroup", func() {
				badContainerShares := readCPUShares(badContainerCgroup)
				Expect(badContainerShares).To(Equal(3456))
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
	}
	Expect(os.WriteFile(filepath.Join(path, cpuSharesFile), []byte(strconv.Itoa(shares)), 0644)).To(Succeed())
}
