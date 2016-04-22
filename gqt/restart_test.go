package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Surviving Restarts", func() {
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

	const (
		bridgePrefix string = "m"
		subnetName   string = "177-100-10-0"
	)

	BeforeEach(func() {
		args = append(args, "--tag", "m")
	})

	Describe("destruction of container resources", func() {
		var container garden.Container

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{
				Network: "177.100.10.30/24",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Stop()).To(Succeed())
			client = startGarden(args...)
		})

		Context("when the destroy-containers-on-startup flag is passed", func() {
			BeforeEach(func() {
				args = append(args, "--destroy-containers-on-startup")
			})

			It("destroys the remaining containers in the depotDir", func() {
				Expect(ioutil.ReadDir(client.DepotDir)).To(BeEmpty())
			})

			It("destroys the remaining containers' iptables", func() {
				out, err := exec.Command("iptables", "-w", "-S", "-t", "filter").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).NotTo(MatchRegexp("w-m-instance.* 177.100.10.0/24"))
			})

			It("destroys the remaining containers' bridges", func() {
				out, err := exec.Command("ifconfig").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				pattern := fmt.Sprintf(".*w%s%s.*", bridgePrefix, subnetName)
				Expect(string(out)).NotTo(MatchRegexp(pattern))
			})

			It("kills the container processes", func() {
				processes, err := exec.Command("ps", "aux").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				Expect(string(processes)).NotTo(ContainSubstring(fmt.Sprintf("run runc /tmp/test-garden-%d/containers/%s", GinkgoParallelNode(), container.Handle())))
			})
		})

		Context("when the destroy-containers-on-startup flag is not passed", func() {
			It("does not destroy the remaining containers in the depotDir", func() {
				Expect(filepath.Join(client.DepotDir, container.Handle())).To(BeADirectory())
			})

			It("does not kill the container processes", func() {
				processes, err := exec.Command("ps", "aux").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				Expect(string(processes)).To(ContainSubstring(fmt.Sprintf("run runc /tmp/test-garden-%d/containers/%s", GinkgoParallelNode(), container.Handle())))
			})

			It("can still run processes in surviving containers", func() {
				out := gbytes.NewBuffer()
				proc, err := container.Run(
					garden.ProcessSpec{
						Path: "/bin/sh",
						Args: []string{"-c", "echo hello; exit 12"},
					},
					garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, out),
						Stderr: io.MultiWriter(GinkgoWriter, out),
					})
				Expect(err).NotTo(HaveOccurred())

				exitCode, err := proc.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(exitCode).To(Equal(12))
				Expect(out).To(gbytes.Say("hello"))
			})

			It("can still destroy the container", func() {
				Expect(client.Destroy(container.Handle())).To(Succeed())
			})
		})
	})

	Describe("successful operations after restart", func() {
		It("can still create container", func() {
			spec := garden.ContainerSpec{
				Network: "177.100.10.30/24",
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
