package gqt_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Destroying a Container", func() {
	var (
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		config.DebugIP = "0.0.0.0"
		config.DebugPort = intptr(8080 + GinkgoParallelNode())
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("should not leak goroutines", func() {
		numGoroutinesBefore := numGoRoutines(client)

		handle := fmt.Sprintf("goroutine-leak-test-%d", GinkgoParallelNode())
		_, err := client.Create(context.Background(), garden.ContainerSpec{
			Handle: handle,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Destroy(handle)).To(Succeed())

		Eventually(pollNumGoRoutines(client)).Should(BeNumerically("<=", numGoroutinesBefore))
	})

	It("should destroy the container's rootfs", func() {
		container, err := client.Create(context.Background(), garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		info, err := container.Info()
		Expect(err).NotTo(HaveOccurred())
		containerRootfs := info.ContainerPath

		Expect(client.Destroy(container.Handle())).To(Succeed())

		Expect(containerRootfs).NotTo(BeAnExistingFile())
	})

	It("should destroy the container's depot directory", func() {
		container, err := client.Create(context.Background(), garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(client.Destroy(container.Handle())).To(Succeed())

		Expect(filepath.Join(client.DepotDir, container.Handle())).NotTo(BeAnExistingFile())
	})

	It("should kill the container's init process", func() {
		container, err := client.Create(context.Background(), garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		initProcPid := initProcessPID(container.Handle())

		_, err = container.Run(garden.ProcessSpec{
			Path: "/bin/sh",
			Args: []string{
				"-c", "read x",
			},
		}, ginkgoIO)
		Expect(err).NotTo(HaveOccurred())

		Expect(client.Destroy(container.Handle())).To(Succeed())

		var killExitCode = func() int {
			sess, err := gexec.Start(exec.Command("kill", "-0", fmt.Sprintf("%d", initProcPid)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			sess.Wait(1 * time.Second)
			return sess.ExitCode()
		}

		Eventually(killExitCode).Should(Equal(1))
	})

	Context("when container destroy fails half way through", func() {
		var (
			container  garden.Container
			mountInUse string
		)

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(context.Background(), garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			// make image deletion fail (because device or resource busy)
			mountInUse = filepath.Join(config.StorePath, "images", container.Handle(), "mount")
			Expect(os.MkdirAll(mountInUse, 0755)).To(Succeed())
			tmpDirToMount := tempDir("", "toMount")
			Expect(exec.Command("mount", "--bind", tmpDirToMount, mountInUse).Run()).To(Succeed())
		})

		It("is still able to list the container so that destroy can be retried", func() {
			Expect(client.Destroy(container.Handle())).NotTo(Succeed())

			containers, err := client.Containers(garden.Properties{})
			Expect(err).NotTo(HaveOccurred())
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].Handle()).To(Equal(container.Handle()))

			// unmount so that destroy is successful
			Expect(exec.Command("umount", mountInUse).Run()).To(Succeed())
			Expect(client.Destroy(container.Handle())).To(Succeed())

			containers, err = client.Containers(garden.Properties{})
			Expect(err).NotTo(HaveOccurred())
			Expect(containers).To(BeEmpty())
		})
	})

	Describe("networking resources", func() {
		var (
			container         garden.Container
			networkSpec       string
			contIfaceName     string
			networkBridgeName string
		)

		JustBeforeEach(func() {
			var err error

			networkSpec = fmt.Sprintf("177.100.%d.0/24", GinkgoParallelNode())
			container, err = client.Create(context.Background(), garden.ContainerSpec{
				Network: networkSpec,
			})
			Expect(err).NotTo(HaveOccurred())
			contIfaceName = ethInterfaceName(container)

			networkBridgeName, err = container.Property("kawasaki.bridge-interface")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when destroy is called", func() {
			var iptableInstance string

			JustBeforeEach(func() {
				var err error
				iptableInstance, err = container.Property("kawasaki.iptable-inst")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.Destroy(container.Handle())).To(Succeed())
			})

			It("should remove iptable entries", func() {
				out, err := runIPTables("-S", "-t", "filter")
				Expect(err).NotTo(HaveOccurred())

				Expect(string(out)).NotTo(ContainSubstring(fmt.Sprintf("w-%d-instance-%s", GinkgoParallelNode(), iptableInstance)))
			})

			It("should remove virtual ethernet cards", func() {
				ifconfigExits := func() int {
					session, err := gexec.Start(exec.Command("ifconfig", contIfaceName), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					return session.Wait().ExitCode()
				}
				Eventually(ifconfigExits).ShouldNot(Equal(0))
			})

			It("should remove the network bridge", func() {
				Expect(checkBridgePresence(networkBridgeName)).To(BeFalse())
			})

			Context("and there was more than one containers in the same subnet", func() {
				var otherContainer garden.Container

				JustBeforeEach(func() {
					var err error

					otherContainer, err = client.Create(context.Background(), garden.ContainerSpec{
						Network: networkSpec,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					Expect(client.Destroy(otherContainer.Handle())).To(Succeed())
				})

				It("should remove the network bridge", func() {
					Expect(checkBridgePresence(networkBridgeName)).To(BeFalse())
				})
			})

			It("removes the depot", func() {
				Expect(filepath.Join(config.DepotDir, container.Handle())).NotTo(BeADirectory())
			})
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

func checkBridgePresence(networkBridgeName string) bool {
	cmd := exec.Command("ip", "link", "show", networkBridgeName)
	output, err := cmd.CombinedOutput()
	Expect(err).To(HaveOccurred())
	return !strings.Contains(string(output), "does not exist")
}
