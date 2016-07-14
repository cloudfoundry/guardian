package gqt_test

import (
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Security", func() {
	var (
		client *runner.RunningGarden
		args   []string
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

	Describe("Apparmor", func() {
		Context("when the --apparmor flag is pointing to a loaded policy", func() {
			BeforeEach(func() {
				args = append(args, "--apparmor", "garden-default")
			})

			It("should enforce the policy when running processes in unprivileged containers", func() {
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

				Eventually(buffer).Should(gbytes.Say("garden-default"))
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
				args = append(args, "--apparmor", "non-existing-policy")
			})

			It("should return an error when creating a container", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
