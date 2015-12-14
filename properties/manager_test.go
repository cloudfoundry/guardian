package properties_test

import (
	"github.com/cloudfoundry-incubator/guardian/properties"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	var (
		propertyManager *properties.Manager
	)

	Describe("Manager", func() {
		BeforeEach(func() {
			propertyManager = properties.NewManager()

			err := propertyManager.SetProperty("name", "value")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the properties", func() {
			props, err := propertyManager.Properties()
			Expect(err).NotTo(HaveOccurred())

			Expect(props).To(HaveLen(1))
			Expect(props).To(HaveKeyWithValue("name", "value"))
		})

		It("returns a specific property when passed a name", func() {
			property, err := propertyManager.Property("name")
			Expect(err).NotTo(HaveOccurred())
			Expect(property).To(Equal("value"))
		})

		It("removes properties", func() {
			props, err := propertyManager.Properties()
			Expect(err).NotTo(HaveOccurred())

			Expect(props).To(HaveLen(1))

			err = propertyManager.RemoveProperty("name")
			Expect(err).NotTo(HaveOccurred())

			_, err = propertyManager.Property("name")
			Expect(err).To(MatchError(properties.NoSuchPropertyError{"No such property: name"}))
		})

		Context("when the property already exists", func() {
			It("updates the property value", func() {
				err := propertyManager.SetProperty("name", "some-other-value")
				Expect(err).NotTo(HaveOccurred())

				props, err := propertyManager.Properties()
				Expect(err).NotTo(HaveOccurred())
				Expect(props).To(HaveKeyWithValue("name", "some-other-value"))
			})
		})

		Context("when attempting to remove a property that doesn't exist", func() {
			It("returns a NoSuchPropertyError", func() {
				err := propertyManager.RemoveProperty("missing")
				Expect(err).To(MatchError(properties.NoSuchPropertyError{"No such property: missing"}))
			})
		})
	})
})
