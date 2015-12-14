package gardener_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/gardener/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		container       garden.Container
		propertyManager *fakes.FakePropertyManager
	)

	Describe("Properties", func() {
		BeforeEach(func() {
			propertyManager = new(fakes.FakePropertyManager)
			container = gardener.NewContainer(nil, "", nil, propertyManager)
		})

		It("delegates to the property manager for Properties", func() {
			container.Properties()
			Expect(propertyManager.PropertiesCallCount()).To(Equal(1))
		})

		It("delegates to the property manager for SetProperty", func() {
			container.SetProperty("name", "value")
			Expect(propertyManager.SetPropertyCallCount()).To(Equal(1))
		})

		It("delegates to the property manager for Property", func() {
			container.Property("name")
			Expect(propertyManager.PropertyCallCount()).To(Equal(1))
		})

		It("delegates to the property manager for RemoveProperty", func() {
			container.RemoveProperty("name")
			Expect(propertyManager.RemovePropertyCallCount()).To(Equal(1))
		})
	})
})
