package bundlerules_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

		Expect(newBndl.Resources().Pids.Limit).To(BeNumerically("==", 1))
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

		It("sets the correct memory limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{LimitInBytes: 4096},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(newBndl.Resources().Unified["memory.max"]).To(Equal("4096"))
		})

		It("limits swap to regular memory limit in bundle resources", func() {
			newBndl, err := bundlerules.Limits{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{LimitInBytes: 4096},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(newBndl.Resources().Unified["memory.swap.max"]).To(Equal("4096"))
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
