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
			container = gardener.NewContainer(nil, "some-handle", nil, nil, propertyManager)
		})

		It("delegates to the property manager for Properties", func() {
			container.Properties()
			Expect(propertyManager.AllCallCount()).To(Equal(1))
			handle := propertyManager.AllArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		It("delegates to the property manager for SetProperty", func() {
			container.SetProperty("name", "value")
			Expect(propertyManager.SetCallCount()).To(Equal(1))
			handle, prop, val := propertyManager.SetArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(prop).To(Equal("name"))
			Expect(val).To(Equal("value"))
		})

		It("delegates to the property manager for Property", func() {
			container.Property("name")
			Expect(propertyManager.GetCallCount()).To(Equal(1))
			handle, name := propertyManager.GetArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(name).To(Equal("name"))
		})

		It("delegates to the property manager for RemoveProperty", func() {
			container.RemoveProperty("name")
			Expect(propertyManager.RemoveCallCount()).To(Equal(1))
			handle, name := propertyManager.RemoveArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(name).To(Equal("name"))
		})
	})
})
