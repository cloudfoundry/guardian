package gqt_test

import (
	"io"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Security", func() {
	var client *runner.RunningGarden

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("Apparmor", func() {
		Context("when the --apparmor flag is pointing to a loaded policy", func() {
			BeforeEach(func() {
				config.AppArmor = "garden-default"
			})

			Context("when running processes in unprivileged containers", func() {
				var (
					container garden.Container
					err       error
				)

				JustBeforeEach(func() {
					container, err = client.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should enforce the policy", func() {
					buffer := gbytes.NewBuffer()

					_, err = container.Run(garden.ProcessSpec{
						Path:  "cat",
						Args:  []string{"/proc/self/attr/current"},
						Image: garden.ImageRef{},
					}, garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, buffer),
						Stderr: GinkgoWriter,
					})
					Expect(err).NotTo(HaveOccurred())

					Eventually(buffer).Should(gbytes.Say("garden-default"))
				})

				Context("when running a pea", func() {
					var peaRootfs string

					BeforeEach(func() {
						peaRootfs = createPeaRootfsTar()
					})

					AfterEach(func() {
						Expect(os.RemoveAll(filepath.Dir(peaRootfs))).To(Succeed())
					})

					It("should enforce the policy", func() {
						buffer := gbytes.NewBuffer()

						_, err = container.Run(garden.ProcessSpec{
							Path:  "cat",
							Args:  []string{"/proc/self/attr/current"},
							Image: garden.ImageRef{URI: peaRootfs},
						}, garden.ProcessIO{
							Stdout: io.MultiWriter(GinkgoWriter, buffer),
							Stderr: GinkgoWriter,
						})
						Expect(err).NotTo(HaveOccurred())

						Eventually(buffer).Should(gbytes.Say("garden-default"))
					})
				})
			})

			Context("when running processes in privileged containers", func() {
				It("should not enforce the policy", func() {
					container, err := client.Create(garden.ContainerSpec{
						Privileged: true,
					})
					Expect(err).NotTo(HaveOccurred())

					buffer := gbytes.NewBuffer()

					_, err = container.Run(garden.ProcessSpec{
						Path: "cat",
						Args: []string{"/proc/self/attr/current"},
					}, garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, buffer),
						Stderr: GinkgoWriter,
					})
					Expect(err).NotTo(HaveOccurred())

					Eventually(buffer).Should(gbytes.Say("unconfined"))
				})
			})
		})

		Context("when the --apparmor flag is not set", func() {
			It("should not enforce the policy when running processes in unprivileged containers", func() {
				container, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()

				_, err = container.Run(garden.ProcessSpec{
					Path: "cat",
					Args: []string{"/proc/self/attr/current"},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, buffer),
					Stderr: GinkgoWriter,
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(buffer).Should(gbytes.Say("unconfined"))
			})

			It("should not enforce the policy when running processes in privileged containers", func() {
				container, err := client.Create(garden.ContainerSpec{
					Privileged: true,
				})
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()

				_, err = container.Run(garden.ProcessSpec{
					Path: "cat",
					Args: []string{"/proc/self/attr/current"},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, buffer),
					Stderr: GinkgoWriter,
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(buffer).Should(gbytes.Say("unconfined"))
			})
		})

		Context("when the --apparmor flag is pointing to a non-existing policy", func() {
			BeforeEach(func() {
				config.AppArmor = "non-existing-policy"
			})

			It("should return an error when creating a container", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ptrace in seccomp allow rules", func() {
		It("should allow the ptrace syscall without CAP_SYS_PTRACE", func() {
			container, err := client.Create(garden.ContainerSpec{
				Image: garden.ImageRef{
					URI: "docker://cloudfoundry/garden-fuse",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			process, err := container.Run(garden.ProcessSpec{
				Path: "strace",
				Args: []string{"echo", "hello"},
			}, garden.ProcessIO{
				Stdout: GinkgoWriter,
				Stderr: GinkgoWriter,
			})
			Expect(err).NotTo(HaveOccurred())

			status := make(chan int)
			go func() {
				exit, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				status <- exit
			}()

			Eventually(status).Should(Receive(Equal(0)))
		})
	})
})
