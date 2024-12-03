package cgroups_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
)

var _ = Describe("Rundmc/Cgroups/Cpucgrouper", func() {
	var (
		cpuCgrouper cgroups.CPUCgrouper
		rootPath    string
	)

	BeforeEach(func() {
		runccgroups.TestMode = true
	})

	JustBeforeEach(func() {
		cpuCgrouper = cgroups.NewCPUCgrouper(rootPath)
	})

	Describe("creating the bad cgroup", func() {
		BeforeEach(func() {
			var err error
			rootPath, err = os.MkdirTemp(cgroups.Root, "garden")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(rootPath)
		})

		It("creates the bad cgroup in the correct place", func() {
			Expect(cpuCgrouper.PrepareCgroups("gingerbread!")).To(Succeed())
			path := filepath.Join(rootPath, cgroups.BadCgroupName, "gingerbread!")
			Expect(path).To(BeADirectory())
		})
	})

	Describe("deleting the bad cgroup", func() {
		var badCgroupPath string
		BeforeEach(func() {
			var err error
			rootPath, err = os.MkdirTemp("", "garden")
			Expect(err).NotTo(HaveOccurred())

			badCgroupPath = filepath.Join(rootPath, cgroups.BadCgroupName, "frenchtoast!")
			Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())
		})

		AfterEach(func() {
			os.RemoveAll(rootPath)
		})

		It("deletes the bad cgroup", func() {
			Expect(cpuCgrouper.CleanupCgroups("frenchtoast!")).To(Succeed())
			Expect(badCgroupPath).NotTo(BeADirectory())
		})
	})

	Describe("reading the CPU stats from the bad cgroup", func() {
		var badCgroupPath string

		BeforeEach(func() {
			var err error
			// not a real cgroup, so we can write to cpu.stat
			rootPath, err = os.MkdirTemp("", "garden")
			Expect(err).NotTo(HaveOccurred())

			badCgroupPath = filepath.Join(rootPath, cgroups.BadCgroupName, "pancakes!")
			Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())

			if runccgroups.IsCgroup2UnifiedMode() {
				// time in milliseconds
				Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpu.stat"), []byte("usage_usec 123\nuser_usec 456\nsystem_usec 789\n"), 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(badCgroupPath, "cgroup.procs"), []byte(""), 0755)).To(Succeed())
			} else {
				// time in nanoseconds
				Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpuacct.usage"), []byte("123"), 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpuacct.stat"), []byte("user 456\nsystem 789"), 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpuacct.usage_percpu"), []byte("0 0"), 0755)).To(Succeed())
			}
			// stats are in nanoseconds
		})

		AfterEach(func() {
			os.RemoveAll(rootPath)
		})

		It("returns the CPU usages", func() {
			usage, err := cpuCgrouper.ReadBadCgroupUsage("pancakes!")
			Expect(err).NotTo(HaveOccurred())

			if runccgroups.IsCgroup2UnifiedMode() {
				Expect(usage).To(Equal(garden.ContainerCPUStat{
					Usage:  123000,
					User:   456000,
					System: 789000,
				}))
			} else {
				// The weird values in user and system usage come from https://github.com/opencontainers/runc/blob/2186cfa3cd52b8e00b1de76db7859cacdf7b1f94/libcontainer/cgroups/fs/cpuacct.go#L19
				var clockTicks uint64 = 100
				Expect(usage).To(Equal(garden.ContainerCPUStat{
					Usage:  123,
					User:   uint64((456 * 1000000000) / clockTicks),
					System: uint64((789 * 1000000000) / clockTicks),
				}))
			}
		})

		When("reading the CPU stats fail", func() {
			BeforeEach(func() {
				if runccgroups.IsCgroup2UnifiedMode() {
					Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpu.stat"), []byte("user foo\nsystem bar"), 0755)).To(Succeed())
				} else {
					Expect(os.WriteFile(filepath.Join(badCgroupPath, "cpuacct.stat"), []byte("user foo\nsystem bar"), 0755)).To(Succeed())
				}
			})

			It("propagates the error", func() {
				_, err := cpuCgrouper.ReadBadCgroupUsage("pancakes!")
				Expect(err.Error()).To(ContainSubstring("foo"))
			})
		})

		When("the bad cgroup does not exit (because this is an ancient container that existed before cpu throttling was a thing)", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(badCgroupPath)).To(Succeed())
			})

			It("returns not exist error", func() {
				_, err := cpuCgrouper.ReadBadCgroupUsage("pancakes!")
				if runccgroups.IsCgroup2UnifiedMode() {
					// the error is not a Go NotExists error
					Expect(err.Error()).To(ContainSubstring("no such file or directory"))
				} else {
					Expect(os.IsNotExist(err)).To(BeTrue())
				}
			})
		})
	})
})
