package gardener_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		container garden.Container
	)

	Describe("Properties", func() {
		BeforeEach(func() {
			container = gardener.NewContainer(nil, "", nil)

			err := container.SetProperty("name", "value")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the container properties", func() {
			properties, err := container.Properties()
			Expect(err).NotTo(HaveOccurred())

			Expect(properties).To(HaveLen(1))
			Expect(properties).To(HaveKeyWithValue("name", "value"))
		})

		It("returns a specific property when passed a name", func() {
			property, err := container.Property("name")
			Expect(err).NotTo(HaveOccurred())
			Expect(property).To(Equal("value"))
		})

		It("removes properties", func() {
			properties, err := container.Properties()
			Expect(err).NotTo(HaveOccurred())

			Expect(properties).To(HaveLen(1))

			err = container.RemoveProperty("name")
			Expect(err).NotTo(HaveOccurred())

			_, err = container.Property("name")
			Expect(err).To(MatchError(gardener.NoSuchPropertyError{"No such property: name"}))
		})

		Context("when the property already exists", func() {
			It("updates the property value", func() {
				err := container.SetProperty("name", "some-other-value")
				Expect(err).NotTo(HaveOccurred())

				properties, err := container.Properties()
				Expect(err).NotTo(HaveOccurred())
				Expect(properties).To(HaveKeyWithValue("name", "some-other-value"))
			})
		})

		Context("when attempting to remove a property that doesn't exist", func() {
			It("returns a NoSuchPropertyError", func() {
				err := container.RemoveProperty("missing")
				Expect(err).To(MatchError(gardener.NoSuchPropertyError{"No such property: missing"}))
			})
		})
	})
})
