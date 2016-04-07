package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden API", func() {
	var (
		args   []string
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		args = []string{}
	})

	JustBeforeEach(func() {
		client = startGarden(args...)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("Capacity", func() {
		It("returns json representing capacity", func() {
			result, err := client.Capacity()
			Expect(err).NotTo(HaveOccurred())

			Expect(result.MemoryInBytes).To(BeNumerically(">", 0))
			Expect(result.DiskInBytes).To(BeNumerically(">", 0))
			Expect(result.MaxContainers).To(BeNumerically(">", 0))
		})

		Context("when the network pool is /24", func() {
			BeforeEach(func() {
				args = append(args, "--network-pool", "10.254.0.0/24")
			})

			It("returns the capacity of the subnet pool", func() {
				capacity, err := client.Capacity()
				Expect(err).ToNot(HaveOccurred())
				Expect(capacity.MaxContainers).To(Equal(uint64(64)))
			})

			Context("when the capacity is limited by maxContainers", func() {
				BeforeEach(func() {
					args = append(args, "--max-containers", "32")
				})

				It("returns the limited capacity from maxContainers", func() {
					capacity, err := client.Capacity()
					Expect(err).ToNot(HaveOccurred())
					Expect(capacity.MaxContainers).To(Equal(uint64(32)))
				})
			})
		})
	})

	Describe("Restart", func() {
		const (
			bridgePrefix string = "m"
			subnetName   string = "177-100-10-0"
		)

		BeforeEach(func() {
			args = append(args, "--tag", "m")
		})

		Describe("destruction of container resources", func() {
			It("destroys the remaining containers in the depotDir", func() {
				_, err := client.Create(garden.ContainerSpec{
					RootFSPath: "docker:///cloudfoundry/garden-busybox",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.ReadDir(client.DepotDir)).NotTo(BeEmpty())

				Expect(client.Stop()).To(Succeed())
				client = startGarden(args...)
				Expect(ioutil.ReadDir(client.DepotDir)).To(BeEmpty())
			})

			It("destroys the remaining containers' iptables", func() {
				_, err := client.Create(garden.ContainerSpec{
					RootFSPath: "docker:///cloudfoundry/garden-busybox",
					Network:    "177.100.10.30/24",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(client.Stop()).To(Succeed())
				client = startGarden(args...)

				out, err := exec.Command("iptables", "-w", "-S", "-t", "filter").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).NotTo(MatchRegexp("w-%d-instance.* 177.100.10.0/24", GinkgoParallelNode()))

				out, err = exec.Command("ifconfig").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				pattern := fmt.Sprintf(".*w%s%s.*", bridgePrefix, subnetName)
				Expect(string(out)).NotTo(MatchRegexp(pattern))
			})
		})

		Describe("successful operations after restart", func() {
			It("can still create container", func() {
				spec := garden.ContainerSpec{
					RootFSPath: "docker:///cloudfoundry/garden-busybox",
					Network:    "177.100.10.30/24",
				}
				_, err := client.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.Stop()).To(Succeed())
				client = startGarden(args...)
				_, err = client.Create(spec)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
