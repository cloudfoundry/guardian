package throttle_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("SharesBalancer", func() {
	var (
		// logger         *lagertest.TestLogger
		cpuCgroupPath  string
		goodCgroupPath string
		badCgroupPath  string
	)

	BeforeEach(func() {
		// logger = lagertest.NewTestLogger("container-metrics-test")
		cgroupRoot, err := ioutil.TempDir("", "cgroups")
		Expect(err).NotTo(HaveOccurred())

		mountCPUcgroup(cgroupRoot)
		cpuCgroupPath = filepath.Join(cgroupRoot, "cpu")
		goodCgroupPath = filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName)
		Expect(os.MkdirAll(goodCgroupPath, 0755)).To(Succeed())
		badCgroupPath = filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName)
		Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())
	})

	When("no containers have been created yet", func() {
		It("assigns all available shares to the good cgroup", func() {
			goodCgroupShares := readCPUShares(goodCgroupPath)
			badCgroupShares := readCPUShares(badCgroupPath)
			Expect(goodCgroupShares).To(Equal(2))
			Expect(badCgroupShares).To(Equal(2))
		})
	})
})
