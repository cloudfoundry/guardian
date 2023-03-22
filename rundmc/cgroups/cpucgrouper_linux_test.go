package cgroups_test

import (
	"io/ioutil"
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

		var err error
		rootPath, err = ioutil.TempDir("", "garden")
		Expect(err).NotTo(HaveOccurred())

		cpuCgrouper = cgroups.NewCPUCgrouper(rootPath)
	})

	Describe("creating the bad cgroup", func() {
		AfterEach(func() {
			os.RemoveAll(rootPath)
		})

		It("creates the bad cgroup in the correct place", func() {
			Expect(cpuCgrouper.CreateBadCgroup("gingerbread!")).To(Succeed())
			path := filepath.Join(rootPath, cgroups.BadCgroupName, "gingerbread!")
			Expect(path).To(BeADirectory())
		})
	})

	Describe("deleting the bad cgroup", func() {
		var badCgroupPath string

		BeforeEach(func() {
			badCgroupPath = filepath.Join(rootPath, cgroups.BadCgroupName, "frenchtoast!")
			Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())
		})

		It("deletes the bad cgroup", func() {
			Expect(cpuCgrouper.DestroyBadCgroup("frenchtoast!")).To(Succeed())
			Expect(badCgroupPath).NotTo(BeADirectory())
		})
	})

	Describe("reading the CPU stats from the bad cgroup", func() {
		var badCgroupPath string

		BeforeEach(func() {
			badCgroupPath = filepath.Join(rootPath, cgroups.BadCgroupName, "pancakes!")
			Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())

			Expect(ioutil.WriteFile(filepath.Join(badCgroupPath, "cpuacct.usage"), []byte("123"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(badCgroupPath, "cpuacct.stat"), []byte("user 456\nsystem 789"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(badCgroupPath, "cpuacct.usage_percpu"), []byte("0 0"), 0755)).To(Succeed())
		})

		It("returns the CPU usages", func() {
			usage, err := cpuCgrouper.ReadBadCgroupUsage("pancakes!")
			Expect(err).NotTo(HaveOccurred())
			// The weird values in user and system usage come from https://github.com/opencontainers/runc/blob/2186cfa3cd52b8e00b1de76db7859cacdf7b1f94/libcontainer/cgroups/fs/cpuacct.go#L19
			var clockTicks uint64 = 100
			Expect(usage).To(Equal(garden.ContainerCPUStat{
				Usage:  123,
				User:   uint64((456 * 1000000000) / clockTicks),
				System: uint64((789 * 1000000000) / clockTicks),
			}))
		})

		When("reading the CPU stats fail", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(badCgroupPath, "cpuacct.stat"), []byte("user foo\nsystem bar"), 0755)).To(Succeed())
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
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
	})
})
