package throttle_test

import (
	"encoding/json"
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
	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer"
)

var _ = Describe("Enforcer", func() {
	var (
		logger            *lagertest.TestLogger
		handle            string
		cgroupRoot        string
		cpuCgroupPath     string
		command           *exec.Cmd
		expectedCPUShares int
		runcRoot          string
		stateDir          string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("enforcer-test")
		uuid, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())
		handle = uuid.String()

		cgroupRoot, err = os.MkdirTemp("", "cgroups")
		Expect(err).NotTo(HaveOccurred())

		runcRoot, err = os.MkdirTemp("", "runc")
		Expect(err).NotTo(HaveOccurred())
		stateDir = filepath.Join(runcRoot, "some-namespace", handle)

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
		Expect(os.RemoveAll(runcRoot)).To(Succeed())
	})

	Describe("Punish", func() {
		var (
			punishErr error
		)

		JustBeforeEach(func() {
			enforcer := throttle.NewEnforcer(cpuCgroupPath, runcRoot, "some-namespace")
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
					createState(stateDir, goodContainerCgroup)
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

				It("updates the state file with new cgroup path for cgroups v2", func() {
					if !cgroups.IsCgroup2UnifiedMode() {
						Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
					}
					Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(badContainerCgroup))
				})
			})

			Context("when good cgroup has init child cgroup", func() {
				var initCgroupPath string

				BeforeEach(func() {
					makeSubCgroup(goodContainerCgroup, "init")
					initCgroupPath = filepath.Join(goodContainerCgroup, "init")
					writeShares(initCgroupPath, 7890)
					Expect(cgroups.WriteCgroupProc(initCgroupPath, command.Process.Pid)).To(Succeed())
					createState(stateDir, initCgroupPath)
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
					if cgroups.IsCgroup2UnifiedMode() {
						Expect(badContainerShares).To(Equal(int(cgroups.ConvertCPUSharesToCgroupV2Value(7890))))
					} else {
						Expect(badContainerShares).To(Equal(7890))
					}
				})

				It("updates the state file with new cgroup path for cgroups v2", func() {
					if !cgroups.IsCgroup2UnifiedMode() {
						Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
					}
					Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(badContainerCgroup))
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
				createState(stateDir, containerCgroup)

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

			It("does not update the state file with new cgroup path for cgroups v2", func() {
				if !cgroups.IsCgroup2UnifiedMode() {
					Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
				}
				Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(containerCgroup))
			})
		})
	})

	Describe("Release", func() {
		var (
			releaseErr error
		)

		JustBeforeEach(func() {
			enforcer := throttle.NewEnforcer(cpuCgroupPath, runcRoot, "some-namespace")
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
				createState(stateDir, badContainerCgroup)
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

				It("updates the state file with new cgroup path for cgroups v2", func() {
					if !cgroups.IsCgroup2UnifiedMode() {
						Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
					}
					Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(goodContainerCgroup))
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

				It("updates the state file with new cgroup path for cgroups v2", func() {
					if !cgroups.IsCgroup2UnifiedMode() {
						Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
					}
					Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(initCgroupPath))
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
				createState(stateDir, containerCgroup)

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

			It("does not update the state file with new cgroup path for cgroups v2", func() {
				if !cgroups.IsCgroup2UnifiedMode() {
					Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
				}
				Expect(readCgroupPathInState(filepath.Join(runcRoot, "some-namespace", handle))).To(Equal(containerCgroup))
			})
		})
	})
})

func makeSubCgroup(root string, path string) {
	if cgroups.IsCgroup2UnifiedMode() {
		_, err := cgroup2.NewManager(root, "/"+path, &cgroup2.Resources{CPU: &cgroup2.CPU{}})
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(os.MkdirAll(filepath.Join(root, path), 0755)).To(Succeed())
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

func readCgroupPathInState(runcStateDir string) string {
	stateBytes, err := os.ReadFile(filepath.Join(runcStateDir, "state.json"))
	Expect(err).NotTo(HaveOccurred())
	var state libcontainer.State
	err = json.Unmarshal(stateBytes, &state)
	Expect(err).NotTo(HaveOccurred())
	return state.CgroupPaths[""]
}

func createState(runcStateDir string, initialCgroupPath string) {
	Expect(os.MkdirAll(runcStateDir, 0755)).To(Succeed())
	state := libcontainer.State{}
	state.CgroupPaths = map[string]string{"": initialCgroupPath}
	data, err := json.Marshal(state)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(runcStateDir, "state.json"), data, 0755)).To(Succeed())
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
	gardenCgroupPath := filepath.Join(cpuCgroup, gardencgroups.Garden)
	if cgroups.IsCgroup2UnifiedMode() {
		cgroups.WriteFile(gardenCgroupPath, "cgroup.kill", "1")
	}
	Expect(os.RemoveAll(gardenCgroupPath)).To(Succeed())
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
