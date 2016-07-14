package properties_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/properties"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	var (
		propertyManager *properties.Manager
	)

	BeforeEach(func() {
		propertyManager = properties.NewManager()
		propertyManager.Set("handle", "name", "value")
	})

	Describe("DestroyKeySpace", func() {
		It("removes key space", func() {
			err := propertyManager.DestroyKeySpace("handle")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the keyspace is already gone", func() {
			It("succeeds (destroy should be idempotent)", func() {
				Expect(propertyManager.DestroyKeySpace("some-handle-that-doesnt-exist")).To(Succeed())
			})
		})
	})

	Describe("All", func() {
		It("returns the properties", func() {
			props, err := propertyManager.All("handle")
			Expect(err).NotTo(HaveOccurred())

			Expect(props).To(HaveLen(1))
			Expect(props).To(HaveKeyWithValue("name", "value"))
		})
	})

	Describe("Get", func() {
		It("returns a specific property when passed a name", func() {
			property, ok := propertyManager.Get("handle", "name")

			Expect(ok).To(BeTrue())
			Expect(property).To(Equal("value"))
		})
	})

	Describe("Remove", func() {
		It("removes properties", func() {
			props, err := propertyManager.All("handle")
			Expect(err).NotTo(HaveOccurred())

			Expect(props).To(HaveLen(1))

			err = propertyManager.Remove("handle", "name")
			Expect(err).NotTo(HaveOccurred())

			_, ok := propertyManager.Get("handle", "name")
			Expect(ok).To(BeFalse())
		})

		Context("when attempting to remove a property that doesn't exist", func() {
			It("returns a NoSuchPropertyError", func() {
				err := propertyManager.Remove("handle", "missing")
				Expect(err).To(MatchError("cannot Remove handle:missing"))
			})
		})
	})

	Describe("Set", func() {
		Context("when the property already exists", func() {
			It("updates the property value", func() {
				propertyManager.Set("handle", "name", "some-other-value")

				props, err := propertyManager.All("handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(props).To(HaveKeyWithValue("name", "some-other-value"))
			})
		})
	})

	Describe("MatchesAll", func() {
		Context("when the properties list is empty", func() {
			It("matches", func() {
				Expect(propertyManager.MatchesAll("", garden.Properties{})).To(BeTrue())
			})
		})

		Context("when the properties list contains a single property", func() {
			Context("which isn't in the keyspace", func() {
				It("does not match", func() {
					match := propertyManager.MatchesAll("", garden.Properties{"fred": "bob"})
					Expect(match).To(BeFalse())
				})
			})

			Context("...which is in the keyspace", func() {
				BeforeEach(func() {
					propertyManager.Set("flintstones", "wilma", "fred")
				})

				It("matches", func() {
					match := propertyManager.MatchesAll("flintstones", garden.Properties{"wilma": "fred"})
					Expect(match).To(BeTrue())
				})

				Context("with the wrong value", func() {
					It("does not match", func() {
						match := propertyManager.MatchesAll("flintstones", garden.Properties{"wilma": "pebbles"})
						Expect(match).To(BeFalse())
					})
				})
			})
		})

		Context("when the properties list contains many properties", func() {
			Context("all of which are in the keyspace", func() {
				BeforeEach(func() {
					propertyManager.Set("flintstones", "wilma", "fred")
					propertyManager.Set("flintstones", "betty", "barney")
				})

				It("matches", func() {
					match := propertyManager.MatchesAll("flintstones",
						garden.Properties{"wilma": "fred", "betty": "barney"})
					Expect(match).To(BeTrue())
				})
			})

			Context("only some of which are in the namespace", func() {
				BeforeEach(func() {
					propertyManager.Set("flintstones", "wilma", "fred")
					propertyManager.Set("flintstones", "betty", "barney")
				})

				It("does not match", func() {
					match := propertyManager.MatchesAll("flintstones",
						garden.Properties{"wilma": "fred", "pebbles": "bambam", "betty": "barney"})
					Expect(match).To(BeFalse())
				})
			})
		})
	})

	Describe("MarshalJSON", func() {
		It("can be saved and restored from JSON", func() {
			mgr := properties.NewManager()
			mgr.Set("foo", "bar", "baz")
			mgr.Set("bar", "baz", "foo")

			tmp, err := ioutil.TempFile("", "jsonprops")
			Expect(err).NotTo(HaveOccurred())

			Expect(json.NewEncoder(tmp).Encode(mgr)).To(Succeed())
			Expect(tmp.Close()).To(Succeed())

			tmp, err = os.Open(tmp.Name())
			Expect(err).NotTo(HaveOccurred())

			var roundtripped properties.Manager
			Expect(json.NewDecoder(tmp).Decode(&roundtripped)).To(Succeed())

			val, ok := roundtripped.Get("foo", "bar")
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal("baz"))
		})
	})

})
