package gqt_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Destroying a Container", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		client = startGarden()
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	JustBeforeEach(func() {
		Expect(client.Destroy(container.Handle())).To(Succeed())
	})

	Context("when running a process", func() {
		var (
			process         garden.Process
			initProcPid     int
			containerRootfs string
		)

		BeforeEach(func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			initProcPid = initProcessPID(container.Handle())

			process, err = container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{
					"-c", "read x",
				},
			}, ginkgoIO)
			Expect(err).NotTo(HaveOccurred())

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			containerRootfs = info.ContainerPath
		})

		It("should kill the containers init process", func() {
			var killExitCode = func() int {
				sess, err := gexec.Start(exec.Command("kill", "-0", fmt.Sprintf("%d", initProcPid)), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				sess.Wait(1 * time.Second)
				return sess.ExitCode()
			}

			Eventually(killExitCode).Should(Equal(1))
		})

		It("should destroy the container's depot directory", func() {
			Expect(filepath.Join(client.DepotDir, container.Handle())).NotTo(BeAnExistingFile())
		})

		It("should destroy the container rootfs", func() {
			Expect(containerRootfs).NotTo(BeAnExistingFile())
		})
	})

	Context("when using a static subnet", func() {
		var (
			contIfaceName     string
			contHandle        string
			existingContainer garden.Container
		)

		BeforeEach(func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{
				Network: "177.100.10.30/24",
			})
			Expect(err).NotTo(HaveOccurred())
			contIfaceName = ethInterfaceName(container)
			contHandle = container.Handle()

			existingContainer, err = client.Create(garden.ContainerSpec{
				Network: "168.100.20.10/24",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(existingContainer.Handle())).To(Succeed())
		})

		It("should remove iptable entries", func() {
			out, err := exec.Command("iptables", "-w", "-S", "-t", "filter").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(MatchRegexp("g-%d-instance.* 177.100.10.0/24", GinkgoParallelNode()))
			Expect(string(out)).To(ContainSubstring("168.100.20.0/24"))
		})

		It("should remove virtual ethernet cards", func() {
			ifconfigExits := func() int {
				session, err := gexec.Start(exec.Command("ifconfig", contIfaceName), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				return session.Wait().ExitCode()
			}
			Eventually(ifconfigExits).ShouldNot(Equal(0))

			ifaceName := ethInterfaceName(existingContainer)
			session, err := gexec.Start(exec.Command("ifconfig", ifaceName), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
		})

		It("should remove the network bridge", func() {
			session, err := gexec.Start(
				exec.Command("ifconfig"),
				GinkgoWriter, GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			Consistently(session).ShouldNot(gbytes.Say("g%d177-100-10-0", GinkgoParallelNode()))

			session, err = gexec.Start(
				exec.Command("ifconfig"),
				GinkgoWriter, GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gbytes.Say("g%d168-100-20-0", GinkgoParallelNode()))
		})
	})
})

func ethInterfaceName(container garden.Container) string {
	buffer := gbytes.NewBuffer()
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "sh",
			Args: []string{"-c", "ifconfig | grep 'Ethernet' | cut -f 1 -d ' '"},
			User: "root",
		},
		garden.ProcessIO{
			Stdout: buffer,
			Stderr: GinkgoWriter,
		},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(proc.Wait()).To(Equal(0))

	contIfaceName := string(buffer.Contents()) // g3-abc-1

	return contIfaceName[:len(contIfaceName)-2] + "0" // g3-abc-0
}
