package throttle_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/lagertest"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

var _ = Describe("SharesBalancer", func() {
	var (
		err                error
		logger             *lagertest.TestLogger
		sharesBalancer     throttle.SharesBalancer
		memoryProvider     *throttlefakes.FakeMemoryProvider
		cgroupRoot         string
		thisTestCgroupPath string
		goodCgroupPath     string
		badCgroupPath      string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("container-metrics-test")

		var err error
		cgroupRoot, err = ioutil.TempDir("", "cgroups")
		Expect(err).NotTo(HaveOccurred())

		mountCPUcgroup(cgroupRoot)
		id, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		thisTestCgroupPath = filepath.Join(cgroupRoot, "cpu", fmt.Sprintf("balancer-test-%s", id.String()))

		goodCgroupPath = filepath.Join(thisTestCgroupPath, gardencgroups.GoodCgroupName)
		Expect(os.MkdirAll(goodCgroupPath, 0755)).To(Succeed())
		badCgroupPath = filepath.Join(thisTestCgroupPath, gardencgroups.BadCgroupName)
		Expect(os.MkdirAll(badCgroupPath, 0755)).To(Succeed())

		memoryProvider = new(throttlefakes.FakeMemoryProvider)
		memoryProvider.TotalMemoryReturns(10000*throttle.MB, nil)

		sharesBalancer = throttle.NewSharesBalancer(thisTestCgroupPath, memoryProvider, 0.5)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(thisTestCgroupPath)).To(Succeed())
		umountCgroups(cgroupRoot)
		Expect(os.RemoveAll(cgroupRoot)).To(Succeed())
	})

	JustBeforeEach(func() {
		err = sharesBalancer.Run(logger)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	When("no containers have been created yet", func() {
		It("assigns all available shares to the good cgroup", func() {
			Expect(readCPUShares(goodCgroupPath)).To(Equal(9998))
			Expect(readCPUShares(badCgroupPath)).To(Equal(2))
		})
	})

	When("a container is created", func() {
		var container *exec.Cmd

		BeforeEach(func() {
			Expect(createCgroup(goodCgroupPath, "container", 1000)).To(Succeed())
			Expect(createCgroup(badCgroupPath, "container", 1000)).To(Succeed())
			container = exec.Command("sleep", "360")
			Expect(container.Start()).To(Succeed())
		})

		AfterEach(func() {
			Expect(container.Process.Kill()).To(Succeed())
			_, err := container.Process.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

		When("the container is added to the good cgroup", func() {
			BeforeEach(func() {
				Expect(cgroups.WriteCgroupProc(filepath.Join(goodCgroupPath, "container"), container.Process.Pid)).To(Succeed())
			})

			It("keeps everything the same", func() {
				Expect(readCPUShares(goodCgroupPath)).To(Equal(9998))
				Expect(readCPUShares(badCgroupPath)).To(Equal(2))
			})
		})

		When("the container is added to the bad cgroup", func() {
			BeforeEach(func() {
				Expect(cgroups.WriteCgroupProc(filepath.Join(badCgroupPath, "container"), container.Process.Pid)).To(Succeed())
			})

			It("assigns the adjusted sum of the contained shares to the bad cgroup, the rest to the good cgroup", func() {
				Expect(readCPUShares(goodCgroupPath)).To(Equal(9500))
				Expect(readCPUShares(badCgroupPath)).To(Equal(500))
			})

			When("the container goes back to the good cgroup", func() {
				BeforeEach(func() {
					Expect(sharesBalancer.Run(logger)).To(Succeed())
					Expect(readCPUShares(badCgroupPath)).To(BeNumerically(">", 2))
					Expect(cgroups.WriteCgroupProc(filepath.Join(goodCgroupPath, "container"), container.Process.Pid)).To(Succeed())
				})

				It("assigns the container shares back to the good cgroup", func() {
					Expect(readCPUShares(goodCgroupPath)).To(Equal(9998))
					Expect(readCPUShares(badCgroupPath)).To(Equal(2))
				})
			})
		})
	})
})

func createCgroup(parentPath, name string, shares int) error {
	cgroupPath := filepath.Join(parentPath, name)

	err := os.MkdirAll(cgroupPath, 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(cgroupPath, "cpu.shares"), []byte(strconv.Itoa(shares)), 0644)
}
