package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
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
		subnetName string = "177-100-10-0"
	)

	Describe("destruction of container resources", func() {
		var (
			container     garden.Container
			hostNetInPort uint32
			externalIP    string
			propertiesDir string
			existingProc  garden.Process
		)

		BeforeEach(func() {
			var err error
			propertiesDir, err = ioutil.TempDir("", "props")
			Expect(err).NotTo(HaveOccurred())
			args = append(args, "--properties-path", path.Join(propertiesDir, "props.json"))
		})

		JustBeforeEach(func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{
				Network: "177.100.10.30/24",
			})
			Expect(err).NotTo(HaveOccurred())

			hostNetInPort, _, err = container.NetIn(hostNetInPort, 8080)
			Expect(err).NotTo(HaveOccurred())

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			externalIP = info.ExternalIP

			out := gbytes.NewBuffer()
			existingProc, err = container.Run(
				garden.ProcessSpec{
					Path: "/bin/sh",
					Args: []string{"-c", "while true; do echo hello; sleep 1; done;"},
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Stop()).To(Succeed())
			client = startGarden(args...)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(propertiesDir)).To(Succeed())
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
				Expect(string(out)).NotTo(MatchRegexp(fmt.Sprintf("w-%d-instance.* 177.100.10.0/24", GinkgoParallelNode())))
			})

			It("destroys the remaining containers' bridges", func() {
				out, err := exec.Command("ifconfig").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				pattern := fmt.Sprintf(".*w%d%s.*", GinkgoParallelNode(), subnetName)
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

			It("can reattach to processes that are still running", func() {
				out := gbytes.NewBuffer()
				procId := existingProc.ID()
				process, err := container.Attach(procId, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(out).Should(gbytes.Say("hello"))

				Expect(process.Signal(garden.SignalKill)).To(Succeed())

				exited := make(chan struct{})
				go func() {
					process.Wait()
					close(exited)
				}()

				Eventually(exited).Should(BeClosed())
			})

			It("can still destroy the container", func() {
				Expect(client.Destroy(container.Handle())).To(Succeed())
			})

			It("should still be able to access the internet", func() {
				Expect(checkConnection(container, "8.8.8.8", 53)).To(Succeed())
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
