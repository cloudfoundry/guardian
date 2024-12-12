package gqt_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Port Pool", func() {
	Context("when the port pool is exhausted by container creation", func() {
		var (
			portPoolStart int
			containers    []garden.Container
			client        *runner.RunningGarden
			expectedPort  uint32
			propsPoolDir  string
			numContainers int = 2
		)

		JustBeforeEach(func() {
			portPoolStart = GinkgoParallelProcess() * 7000
			propsPoolDir = tempDir("", "portpool")

			config.PortPoolSize = &numContainers
			config.PortPoolPropertiesPath = filepath.Join(propsPoolDir, "props.json")
			config.PortPoolStart = &portPoolStart
			containers = []garden.Container{}
			client = runner.Start(config)

			// Create containers and NetIn
			for i := 0; i < numContainers; i++ {
				container, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				hostPort, _, err := container.NetIn(0, 0)
				if i == numContainers-1 {
					expectedPort = hostPort
				}
				Expect(err).NotTo(HaveOccurred())

				containers = append(containers, container)
			}
		})

		AfterEach(func() {
			Expect(client.DestroyAndStop()).To(Succeed())
			Expect(os.RemoveAll(propsPoolDir)).To(Succeed())
		})

		It("releases ports after the container is destroyed", func() {
			_, _, err := containers[0].NetIn(0, 0)
			Expect(err).To(HaveOccurred())

			Expect(client.Destroy(containers[0].Handle())).To(Succeed())

			_, _, err = containers[1].NetIn(0, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("and then all containers are destroyed", func() {
			BeforeEach(func() {
				numContainers = 3
			})

			JustBeforeEach(func() {
				// Destroy containers in reverse order
				for i := numContainers - 1; i >= 0; i-- {
					Expect(client.Destroy(containers[i].Handle())).To(Succeed())
				}
			})

			It("should FIFO released/allocated ports", func() {
				for i := 0; i < numContainers; i++ {
					container, err := client.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())

					hostPort, _, err := container.NetIn(0, 0)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostPort).To(Equal(uint32(portPoolStart + (numContainers - 1) - i)))
				}
			})

			Context("when guardian is restarted", func() {
				BeforeEach(func() {
					numContainers = 3
				})

				JustBeforeEach(func() {
					Expect(client.Stop()).To(Succeed())
					client = runner.Start(config)
				})

				It("should persist the head of the port allocation queue", func() {
					container, err := client.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())

					hostPort, _, err := container.NetIn(0, 0)
					Expect(err).NotTo(HaveOccurred())
					Expect(expectedPort).To(Equal(uint32(hostPort)))
				})

				It("should arrange the ports in increasing order starting with the head", func() {
					for i := 0; i < numContainers; i++ {
						container, err := client.Create(garden.ContainerSpec{})
						Expect(err).NotTo(HaveOccurred())

						hostPort, _, err := container.NetIn(0, 0)
						Expect(err).NotTo(HaveOccurred())
						if i == 0 {
							Expect(expectedPort).To(Equal(uint32(hostPort)))
						} else {
							Expect(uint32(portPoolStart + i - 1)).To(Equal(hostPort))
						}
					}
				})

			})
		})

	})
})
