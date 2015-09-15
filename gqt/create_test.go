package gqt_test

import (
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

var _ = Describe("Creating a Container", func() {
	var client *runner.RunningGarden
	var container garden.Container

	Context("after creating a container", func() {
		BeforeEach(func() {
			client = startGarden()

			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a depot subdirectory based on the container handle", func() {
			Expect(container.Handle()).NotTo(BeEmpty())
			Expect(filepath.Join(client.DepotDir, container.Handle())).To(BeADirectory())
			Expect(filepath.Join(client.DepotDir, container.Handle(), "config.json")).To(BeARegularFile())
		})

		Describe("destroying the container", func() {
			var process garden.Process

			BeforeEach(func() {
				var err error
				process, err = container.Run(garden.ProcessSpec{
					Path: "/bin/sh",
					Args: []string{
						"-c", "read x",
					},
				}, ginkgoIO)

				Expect(err).NotTo(HaveOccurred())
				Expect(client.Destroy(container.Handle())).To(Succeed())
			})

			It("should kill the containers processes", func() {
				exited := make(chan struct{})
				go func() {
					process.Wait()
					close(exited)
				}()

				Eventually(exited).Should(BeClosed())
			})

			It("should destroy the container's depot directory", func() {
				Expect(filepath.Join(client.DepotDir, container.Handle())).NotTo(BeAnExistingFile())
			})
		})
	})
})
