package properties_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/properties"
	"github.com/cloudfoundry-incubator/guardian/properties/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	var (
		propertyManager *properties.Manager
		mapPersister    *fakes.FakeMapPersister
	)

	Describe("Manager", func() {
		BeforeEach(func() {
			mapPersister = new(fakes.FakeMapPersister)
			propertyManager = properties.NewManager(mapPersister)

			mapPersister.LoadMapStub = func(handle string) (map[string]string, error) {
				if handle == "handle" {
					return map[string]string{
						"name": "value",
					}, nil
				}

				return make(map[string]string), nil
			}

		})

		Describe("DestroyKeySpace", func() {
			It("removes key space", func() {
				mapPersister.IsMapPersistedReturns(true)
				err := propertyManager.DestroyKeySpace("handle")
				Expect(err).NotTo(HaveOccurred())

				Expect(mapPersister.DeleteMapCallCount()).To(Equal(1))
				Expect(mapPersister.DeleteMapArgsForCall((0))).To(Equal("handle"))
			})

			Context("when the keyspace is already gone", func() {
				It("succeeds (destroy should be idempotent)", func() {
					mapPersister.IsMapPersistedReturns(false)
					Expect(propertyManager.DestroyKeySpace("some-handle-that-doesnt-exist")).To(Succeed())
					Expect(mapPersister.DeleteMapCallCount()).To(Equal(0))
				})
			})
		})

		Describe("Set", func() {
			Context("when the keyspace doesn't exist", func() {
				It("saves the map with a single property", func() {
					mapPersister.IsMapPersistedReturns(false)
					Expect(propertyManager.Set("new-handle", "will", "plays-ping-pong")).To(Succeed())

					Expect(mapPersister.SaveMapCallCount()).To(Equal(1))
					handleCalled, propMap := mapPersister.SaveMapArgsForCall(0)

					Expect(handleCalled).To(Equal("new-handle"))
					Expect(propMap).To(Equal(map[string]string{
						"will": "plays-ping-pong",
					}))
				})
			})

			Context("when the keyspace exists", func() {

				BeforeEach(func() {
					mapPersister.IsMapPersistedReturns(true)
				})

				It("saves the map with the additional property", func() {
					err := propertyManager.Set("handle", "spiderman", "some-other-value")
					Expect(err).NotTo(HaveOccurred())

					Expect(mapPersister.LoadMapCallCount()).To(Equal(1))
					Expect(mapPersister.LoadMapArgsForCall(0)).To(Equal("handle"))

					Expect(mapPersister.SaveMapCallCount()).To(Equal(1))
					handleCalled, propMap := mapPersister.SaveMapArgsForCall(0)
					Expect(handleCalled).To(Equal("handle"))
					Expect(propMap).To(Equal(map[string]string{
						"name":      "value",
						"spiderman": "some-other-value",
					}))
				})

				Context("when the property already exists", func() {
					It("updates the property value", func() {
						err := propertyManager.Set("handle", "name", "some-other-value")
						Expect(err).NotTo(HaveOccurred())

						Expect(mapPersister.LoadMapCallCount()).To(Equal(1))
						Expect(mapPersister.LoadMapArgsForCall(0)).To(Equal("handle"))

						Expect(mapPersister.SaveMapCallCount()).To(Equal(1))
						handleCalled, propMap := mapPersister.SaveMapArgsForCall(0)
						Expect(handleCalled).To(Equal("handle"))
						Expect(propMap).To(Equal(map[string]string{
							"name": "some-other-value",
						}))
					})
				})

				Context("when loading the property map fails", func() {
					It("returns an appropriate error", func() {
						mapPersister.LoadMapReturns(map[string]string{}, errors.New("not-valid-handle"))
						err := propertyManager.Set("handle", "name", "some-value")
						Expect(err.Error()).To(ContainSubstring(("failed to set property for handle: handle")))
						Expect(err.Error()).To(ContainSubstring(("not-valid-handle")))
					})
				})
			})

			Context("when saving the property map fails", func() {
				It("returns an appropriate error", func() {
					mapPersister.SaveMapReturns(errors.New("failed-to-save"))
					err := propertyManager.Set("handle", "name", "some-value")

					Expect(err.Error()).To(ContainSubstring(("failed to set property for handle: handle")))
					Expect(err.Error()).To(ContainSubstring(("failed-to-save")))
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

			Context("when loading the property map fails", func() {
				It("returns an appropriate error", func() {
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("not-valid-handle"))
					_, err := propertyManager.All("handle")

					Expect(err.Error()).To(ContainSubstring(("failed to get properties for handle: handle")))
					Expect(err.Error()).To(ContainSubstring(("not-valid-handle")))
				})

				It("returns an empty map", func() {
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("not-valid-handle"))
					props, _ := propertyManager.All("handle")

					Expect(props).NotTo(BeNil())
					Expect(props).To(HaveLen(0))
				})
			})
		})

		Describe("Get", func() {
			It("returns a specific property when passed a name", func() {
				property, err := propertyManager.Get("handle", "name")

				Expect(err).NotTo(HaveOccurred())
				Expect(property).To(Equal("value"))
			})

			Context("when loading the property map fails", func() {
				It("returns the error", func() {
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("lookup-kaput"))

					_, err := propertyManager.Get("handle", "name")

					Expect(err.Error()).To(ContainSubstring(("cannot Get handle:name")))
					Expect(err.Error()).To(ContainSubstring(("lookup-kaput")))
				})
			})

			Context("when the property is not present in the map", func() {
				It("returns the error", func() {
					_, err := propertyManager.Get("handle", "will-is-not-here")
					Expect(err.Error()).To(ContainSubstring(("cannot Get handle:will-is-not-here")))
				})
			})
		})

		Describe("Remove", func() {
			It("removes properties", func() {
				props, err := propertyManager.All("handle")
				Expect(err).NotTo(HaveOccurred())

				Expect(props).To(HaveLen(1))

				err = propertyManager.Remove("handle", "name")
				Expect(err).NotTo(HaveOccurred())

				Expect(mapPersister.SaveMapCallCount()).To(Equal(1))
				handleArg, propMap := mapPersister.SaveMapArgsForCall(0)
				Expect(handleArg).To(Equal("handle"))
				Expect(propMap).To(Equal(map[string]string{}))
			})

			Context("when loading the property map fails", func() {
				It("returns the error", func() {
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("remove-kaput"))

					err := propertyManager.Remove("handle", "name")

					Expect(err.Error()).To(ContainSubstring(("cannot Remove handle:name")))
					Expect(err.Error()).To(ContainSubstring(("remove-kaput")))
				})
			})

			Context("when attempting to remove a property that doesn't exist", func() {
				It("returns a NoSuchPropertyError", func() {
					err := propertyManager.Remove("handle", "missing")
					Expect(err).To(MatchError("cannot Remove handle:missing"))
				})
			})

			Context("when the save to map fails", func() {
				It("returns the error", func() {
					mapPersister.SaveMapReturns((errors.New("save-kaput")))
					err := propertyManager.Remove("handle", "name")
					Expect(err.Error()).To(ContainSubstring(("cannot Remove handle:name")))
					Expect(err.Error()).To(ContainSubstring(("save-kaput")))
				})
			})
		})

		Describe("MatchesAll", func() {

			Context("when no map has been persisted with the given handle", func() {

				BeforeEach(func() {
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("no-such-handle"))
					mapPersister.IsMapPersistedReturns(false)
				})

				It("does not match", func() {
					match, err := propertyManager.MatchesAll("flintstones", garden.Properties{"wilma": "fred"})

					Expect(err).NotTo(HaveOccurred())
					Expect(match).To(BeFalse())
				})

				Context("and no properties are filtered upon", func() {
					It("does match", func() {
						match, err := propertyManager.MatchesAll("flintstones", garden.Properties{})

						Expect(err).NotTo(HaveOccurred())
						Expect(match).To(BeTrue())
					})
				})

			})

			Context("when the properties list is empty", func() {
				It("matches", func() {
					Expect(propertyManager.MatchesAll("", garden.Properties{})).To(BeTrue())
				})
			})

			Context("when the properties list contains a single property", func() {
				Context("which isn't in the keyspace", func() {
					It("does not match", func() {
						match, err := propertyManager.MatchesAll("", garden.Properties{"fred": "bob"})

						Expect(err).NotTo(HaveOccurred())
						Expect(match).To(BeFalse())
					})
				})

				Context("which is in the keyspace", func() {
					BeforeEach(func() {
						mapPersister.LoadMapStub = func(handle string) (map[string]string, error) {
							if handle == "flintstones" {
								return map[string]string{
									"wilma": "fred",
								}, nil
							}

							return make(map[string]string), nil
						}

						mapPersister.IsMapPersistedReturns(true)
					})

					It("matches", func() {
						match, err := propertyManager.MatchesAll("flintstones", garden.Properties{"wilma": "fred"})

						Expect(err).NotTo(HaveOccurred())
						Expect(match).To(BeTrue())
					})

					Context("with the wrong value", func() {
						It("does not match", func() {
							match, err := propertyManager.MatchesAll("flintstones", garden.Properties{"wilma": "pebbles"})

							Expect(err).NotTo(HaveOccurred())
							Expect(match).To(BeFalse())
						})
					})
				})
			})

			Context("when the properties list contains many properties", func() {
				Context("all of which are in the keyspace", func() {
					BeforeEach(func() {
						mapPersister.LoadMapStub = func(handle string) (map[string]string, error) {
							if handle == "flintstones" {
								return map[string]string{
									"wilma": "fred",
									"betty": "barney",
								}, nil
							}

							return make(map[string]string), nil
						}
						mapPersister.IsMapPersistedReturns(true)
					})

					It("matches", func() {
						match, err := propertyManager.MatchesAll("flintstones",
							garden.Properties{"wilma": "fred", "betty": "barney"})

						Expect(err).NotTo(HaveOccurred())
						Expect(match).To(BeTrue())
					})
				})

				Context("only some of which are in the namespace", func() {
					BeforeEach(func() {
						propertyManager.Set("flintstones", "wilma", "fred")
						propertyManager.Set("flintstones", "betty", "barney")
					})

					It("does not match", func() {
						match, err := propertyManager.MatchesAll("flintstones",
							garden.Properties{"wilma": "fred", "pebbles": "bambam", "betty": "barney"})

						Expect(err).NotTo(HaveOccurred())
						Expect(match).To(BeFalse())
					})
				})
			})

			Context("when loading the property map fails", func() {
				It("returns the error", func() {
					mapPersister.IsMapPersistedReturns(true)
					mapPersister.LoadMapReturns(map[string]string{}, errors.New("lookup-kaput"))
					_, err := propertyManager.MatchesAll("flintstones", garden.Properties{"key": "value"})
					Expect(err.Error()).To(ContainSubstring(("cannot MatchAll flintstones")))
					Expect(err.Error()).To(ContainSubstring(("lookup-kaput")))
				})
			})
		})
	})
})
