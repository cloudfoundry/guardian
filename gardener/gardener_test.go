package gardener_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/gardener/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gardener", func() {
	var (
		containerizer *fakes.FakeContainerizer
		gdnr          *gardener.Gardener
	)

	BeforeEach(func() {
		containerizer = new(fakes.FakeContainerizer)
		gdnr = &gardener.Gardener{
			Containerizer: containerizer,
		}
	})

	Describe("creating a container", func() {
		It("asks the containerizer to create a container", func() {
			_, err := gdnr.Create(garden.ContainerSpec{
				Handle: "bob",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(containerizer.CreateCallCount()).To(Equal(1))
			Expect(containerizer.CreateArgsForCall(0)).To(Equal(gardener.DesiredContainerSpec{
				Handle: "bob",
			}))
		})
	})
})
