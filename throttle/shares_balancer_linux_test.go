package throttle_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
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
		cgroupRoot, err = os.MkdirTemp("", "cgroups")
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

	Context("when memory is less than the total sum of bad shares", func() {
		var container1, container2 *exec.Cmd

		BeforeEach(func() {
			memoryProvider.TotalMemoryReturns(999*throttle.MB, nil)

			createCgroup(badCgroupPath, "container1", 1000)
			createCgroup(badCgroupPath, "container2", 1000)

			container1 = exec.Command("sleep", "360")
			Expect(container1.Start()).To(Succeed())
			Expect(cgroups.WriteCgroupProc(filepath.Join(badCgroupPath, "container1"), container1.Process.Pid)).To(Succeed())

			container2 = exec.Command("sleep", "360")
			Expect(container2.Start()).To(Succeed())
			Expect(cgroups.WriteCgroupProc(filepath.Join(badCgroupPath, "container2"), container2.Process.Pid)).To(Succeed())
		})

		AfterEach(func() {
			Expect(container1.Process.Kill()).To(Succeed())
			_, err := container1.Process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(container2.Process.Kill()).To(Succeed())
			_, err = container2.Process.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

		It("assigns the adjusted sum of the contained shares to the bad cgroup, the rest to the good cgroup", func() {
			Expect(readCPUShares(goodCgroupPath)).To(Equal(262144))
			Expect(readCPUShares(badCgroupPath)).To(Equal(1000))
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

	return os.WriteFile(filepath.Join(cgroupPath, "cpu.shares"), []byte(strconv.Itoa(shares)), 0644)
}
