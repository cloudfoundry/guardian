package gqt_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Limits", func() {
	Describe("LimitMemory", func() {
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

		Context("with a memory limit", func() {
			BeforeEach(func() {
				var err error
				container, err = client.Create(garden.ContainerSpec{
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{
							LimitInBytes: 64 * 1024 * 1024,
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the process writes too much to /dev/shm/too-big", func() {
				It("is killed", func() {
					process, err := container.Run(garden.ProcessSpec{
						User: "root",
						Path: "dd",
						Args: []string{"if=/dev/urandom", "of=/dev/shm/too-big", "bs=1M", "count=65"},
					}, ginkgoIO,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(process.Wait()).ToNot(Equal(0))
				})
			})
		})
	})
})
