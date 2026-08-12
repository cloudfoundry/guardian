//lint:file-ignore SA1019 - we still specify LimitInShares to make the deprecated logic work until we get rid of the code in garden

package bundlerules_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("LimitsRule", func() {
	It("sets the provided BlockIOWeight in the bundle resources", func() {
		limits := bundlerules.Limits{
			BlockIOWeight: 100,
		}
		newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(*(newBndl.Resources().BlockIO.Weight)).To(Equal(limits.BlockIOWeight))
	})

	It("sets the correct PID limit in bundle resources", func() {
		newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
			Limits: garden.Limits{
				Pid: garden.PidLimits{Max: 1},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(newBndl.Resources().Pids.Limit).To(PointTo(Equal(int64(1))))
	})

	Context("cgroup v1", func() {
		BeforeEach(func() {
			if gardencgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
			}
		})

		It("sets the correct memory limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{LimitInBytes: 4096},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(*(newBndl.Resources().Memory.Limit)).To(BeNumerically("==", 4096))
		})

		It("limits swap to regular memory limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{LimitInBytes: 4096},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(newBndl.Resources().Memory.Swap).ToNot(BeNil())
			Expect(*(newBndl.Resources().Memory.Swap)).To(BeNumerically("==", 4096))
		})

		Context("when swap limit is disabled", func() {
			It("does not limit swap in bundle resources", func() {
				limits := bundlerules.Limits{DisableSwapLimit: true}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{LimitInBytes: 4096},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Memory.Swap).To(BeNil())
			})
		})

		It("sets the correct CPU limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					CPU: garden.CPULimits{Weight: 1},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 1))
			Expect(newBndl.Resources().CPU.Period).To(BeNil())
			Expect(newBndl.Resources().CPU.Quota).To(BeNil())
		})

		Context("when a positive cpu quota period per share is provided", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				var quotaPerShare, weight uint64 = 100, 128
				limits := bundlerules.Limits{
					CpuQuotaPerShare: quotaPerShare,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: weight},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Period)).To(BeNumerically("==", 100000))
				Expect(*(newBndl.Resources().CPU.Quota)).To(BeNumerically("==", weight*quotaPerShare))
			})
		})

		Context("when cpu quota * period per share is less than min valid cpu quota", func() {
			It("sets the min valid value of cpu quota in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 1,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Quota)).To(BeNumerically("==", 1000))
			})
		})

		Context("when a zero cpu quota period per share is provided", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 0,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 1))
				Expect(newBndl.Resources().CPU.Period).To(BeNil())
				Expect(newBndl.Resources().CPU.Quota).To(BeNil())
			})
		})

		Context("with positive cpu quota period per share and no shares", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 5,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 0))
				Expect(newBndl.Resources().CPU.Period).To(BeNil())
				Expect(newBndl.Resources().CPU.Quota).To(BeNil())
			})
		})

		Context("when LimitInShares is set", func() {
			It("sets the CPU shares", func() {
				newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{LimitInShares: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 1))
			})
		})

		Context("when both Weight and LimitInShares are set", func() {
			It("Weight has precedence ", func() {
				newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{LimitInShares: 1, Weight: 2},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 2))
			})
		})
	})

	Context("cgroup v2", func() {
		BeforeEach(func() {
			if !gardencgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
			}
		})

		Context("io.max throttling", func() {
			It("sets ThrottleReadBpsDevice when IOMaxReadBps is configured", func() {
				limits := bundlerules.Limits{
					IOMaxReadBps: 59768832,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleReadBpsDevice).NotTo(BeEmpty())
				for _, dev := range newBndl.Resources().BlockIO.ThrottleReadBpsDevice {
					Expect(dev.Rate).To(Equal(uint64(59768832)))
					Expect(dev.Major).NotTo(Equal(int64(0)))
				}
				Expect(newBndl.Resources().BlockIO.ThrottleWriteBpsDevice).To(BeEmpty())
				Expect(newBndl.Resources().BlockIO.ThrottleReadIOPSDevice).To(BeEmpty())
				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).To(BeEmpty())
			})

			It("sets ThrottleWriteBpsDevice when IOMaxWriteBps is configured", func() {
				limits := bundlerules.Limits{
					IOMaxWriteBps: 59768832,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleWriteBpsDevice).NotTo(BeEmpty())
				for _, dev := range newBndl.Resources().BlockIO.ThrottleWriteBpsDevice {
					Expect(dev.Rate).To(Equal(uint64(59768832)))
				}
				Expect(newBndl.Resources().BlockIO.ThrottleReadBpsDevice).To(BeEmpty())
			})

			It("sets ThrottleReadIOPSDevice when IOMaxReadIOPS is configured", func() {
				limits := bundlerules.Limits{
					IOMaxReadIOPS: 900,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleReadIOPSDevice).NotTo(BeEmpty())
				for _, dev := range newBndl.Resources().BlockIO.ThrottleReadIOPSDevice {
					Expect(dev.Rate).To(Equal(uint64(900)))
				}
				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).To(BeEmpty())
			})

			It("sets ThrottleWriteIOPSDevice when IOMaxWriteIOPS is configured", func() {
				limits := bundlerules.Limits{
					IOMaxWriteIOPS: 900,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).NotTo(BeEmpty())
				for _, dev := range newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice {
					Expect(dev.Rate).To(Equal(uint64(900)))
				}
			})

			It("sets all throttle devices when all io.max values are configured", func() {
				limits := bundlerules.Limits{
					IOMaxReadBps:   59768832,
					IOMaxWriteBps:  59768832,
					IOMaxReadIOPS:  900,
					IOMaxWriteIOPS: 900,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleReadBpsDevice).NotTo(BeEmpty())
				Expect(newBndl.Resources().BlockIO.ThrottleWriteBpsDevice).NotTo(BeEmpty())
				Expect(newBndl.Resources().BlockIO.ThrottleReadIOPSDevice).NotTo(BeEmpty())
				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).NotTo(BeEmpty())

				// All throttle device arrays should have the same length (one entry per real block device)
				numDevices := len(newBndl.Resources().BlockIO.ThrottleReadBpsDevice)
				Expect(numDevices).To(BeNumerically(">", 0))
				Expect(newBndl.Resources().BlockIO.ThrottleWriteBpsDevice).To(HaveLen(numDevices))
				Expect(newBndl.Resources().BlockIO.ThrottleReadIOPSDevice).To(HaveLen(numDevices))
				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).To(HaveLen(numDevices))
			})

			It("does not set any throttle devices when all io.max values are 0", func() {
				limits := bundlerules.Limits{
					IOMaxReadBps:   0,
					IOMaxWriteBps:  0,
					IOMaxReadIOPS:  0,
					IOMaxWriteIOPS: 0,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().BlockIO.ThrottleReadBpsDevice).To(BeNil())
				Expect(newBndl.Resources().BlockIO.ThrottleWriteBpsDevice).To(BeNil())
				Expect(newBndl.Resources().BlockIO.ThrottleReadIOPSDevice).To(BeNil())
				Expect(newBndl.Resources().BlockIO.ThrottleWriteIOPSDevice).To(BeNil())
			})
		})

		Context("when swap limit is disabled", func() {
			It("does not limit swap in bundle resources", func() {
				limits := bundlerules.Limits{DisableSwapLimit: true}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{LimitInBytes: 4096},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["memory.swap.max"]).To(Equal(""))
			})
		})

		It("sets the correct CPU limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					CPU: garden.CPULimits{Weight: 1},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(newBndl.Resources().Unified["cpu.weight"]).To(Equal(fmt.Sprintf("%d", gardencgroups.ConvertCPUSharesToCgroupV2Value(1))))
			Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal(""))
		})

		Context("when a positive cpu quota period per share is provided", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				var quotaPerShare, weight uint64 = 100, 128
				limits := bundlerules.Limits{
					CpuQuotaPerShare: quotaPerShare,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: weight},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal("12800 100000"))
			})
		})

		Context("when cpu quota * period per share is less than min valid cpu quota", func() {
			It("sets the min valid value of cpu quota in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 1,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal("1000 100000"))
			})
		})

		Context("when a zero cpu quota period per share is provided", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 0,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{Weight: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.weight"]).To(Equal(fmt.Sprintf("%d", gardencgroups.ConvertCPUSharesToCgroupV2Value(1))))
				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal(""))
			})
		})

		Context("with positive cpu quota period per share and no shares", func() {
			It("sets the correct CPU limit in bundle resources", func() {
				limits := bundlerules.Limits{
					CpuQuotaPerShare: 5,
				}
				newBndl, err := limits.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.weight"]).To(Equal(""))
				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal(""))
			})
		})

		Context("when LimitInShares is set", func() {
			It("sets the CPU shares", func() {
				newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{LimitInShares: 1},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.weight"]).To(Equal(fmt.Sprintf("%d", gardencgroups.ConvertCPUSharesToCgroupV2Value(1))))
				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal(""))
			})
		})

		Context("when both Weight and LimitInShares are set", func() {
			It("Weight has precedence ", func() {
				newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
					Limits: garden.Limits{
						CPU: garden.CPULimits{LimitInShares: 1, Weight: 2},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(newBndl.Resources().Unified["cpu.weight"]).To(Equal(fmt.Sprintf("%d", gardencgroups.ConvertCPUSharesToCgroupV2Value(2))))
				Expect(newBndl.Resources().Unified["cpu.max"]).To(Equal(""))
			})
		})
	})
})
