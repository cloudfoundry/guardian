package groot_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StoreNamespacer", func() {
	var (
		storePath       string
		storeNamespacer *groot.StoreNamespacer
		uidMappings     []groot.IDMappingSpec
		gidMappings     []groot.IDMappingSpec
	)

	BeforeEach(func() {
		var err error
		storePath, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(storePath, store.MetaDirName), 0700)).To(Succeed())
	})

	JustBeforeEach(func() {
		storeNamespacer = groot.NewStoreNamespacer(storePath)
	})

	Describe("Read", func() {
		var (
			expectedMappings groot.IDMappings
			namespaceFile    string
		)
		BeforeEach(func() {
			namespaceFile = filepath.Join(storePath, store.MetaDirName, "namespace.json")
			mappings := []byte(`{"uid-mappings":["0:1000:1","1:100000:10"],"gid-mappings":["0:2000:1","1:200000:10"]}`)
			Expect(os.WriteFile(namespaceFile, mappings, 0700)).To(Succeed())

			expectedMappings = groot.IDMappings{
				UIDMappings: []groot.IDMappingSpec{
					{
						HostID:      1000,
						NamespaceID: 0,
						Size:        1,
					},
					{
						HostID:      100000,
						NamespaceID: 1,
						Size:        10,
					},
				},
				GIDMappings: []groot.IDMappingSpec{
					{
						HostID:      2000,
						NamespaceID: 0,
						Size:        1,
					},
					{
						HostID:      200000,
						NamespaceID: 1,
						Size:        10,
					},
				},
			}
		})

		It("reads from the namespace file", func() {
			mappingsFromFile, err := storeNamespacer.Read()
			Expect(err).NotTo(HaveOccurred())

			Expect(mappingsFromFile).To(Equal(expectedMappings))
		})

		Context("when it fails to read the namespace file", func() {
			BeforeEach(func() {
				storePath = "invalid-path"
			})

			It("returns an error", func() {
				_, err := storeNamespacer.Read()
				Expect(err).To(MatchError(ContainSubstring("reading namespace file")))
			})
		})

		Context("when the mappings file contains invalid json", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(namespaceFile, []byte("junk"), 0600)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := storeNamespacer.Read()
				Expect(err).To(MatchError(ContainSubstring("invalid namespace file")))
			})
		})

		Context("when the mappings file contains", func() {
			Context("invalid uid mappings", func() {
				BeforeEach(func() {
					badUidMappings := []byte(`{"uid-mappings":["1000:1","10"],"gid-mappings":["0:2000:1","1:200000:10"]}`)
					Expect(os.WriteFile(namespaceFile, badUidMappings, 0600)).To(Succeed())
				})

				It("returns an error", func() {
					_, err := storeNamespacer.Read()
					Expect(err).To(MatchError(ContainSubstring("invalid uid mappings format")))
				})
			})

			Context("invalid gid mappings", func() {
				BeforeEach(func() {
					badGidMappings := []byte(`{"gid-mappings":["1000:1","10"],"uid-mappings":["0:2000:1","1:200000:10"]}`)
					Expect(os.WriteFile(namespaceFile, badGidMappings, 0600)).To(Succeed())
				})

				It("returns an error", func() {
					_, err := storeNamespacer.Read()
					Expect(err).To(MatchError(ContainSubstring("invalid gid mappings format")))
				})
			})
		})
	})

	Describe("ApplyMappings", func() {
		BeforeEach(func() {
			uidMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 10},
				groot.IDMappingSpec{HostID: 1000, NamespaceID: 0, Size: 1},
			}

			gidMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 200000, NamespaceID: 1, Size: 10},
				groot.IDMappingSpec{HostID: 2000, NamespaceID: 0, Size: 1},
			}
		})

		Context("when there is no namespace file", func() {
			It("creates the correct namespace file", func() {
				err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
				Expect(err).NotTo(HaveOccurred())

				namespaceFile := filepath.Join(storePath, store.MetaDirName, "namespace.json")
				Expect(namespaceFile).To(BeAnExistingFile())

				contents, err := os.ReadFile(namespaceFile)
				Expect(err).NotTo(HaveOccurred())

				var namespaces map[string][]string
				Expect(json.Unmarshal(contents, &namespaces)).To(Succeed())

				Expect(namespaces["uid-mappings"]).To(Equal([]string{"0:1000:1", "1:100000:10"}))
				Expect(namespaces["gid-mappings"]).To(Equal([]string{"0:2000:1", "1:200000:10"}))
			})

			Context("when it fails to create the namespace file", func() {
				BeforeEach(func() {
					storePath = "invalid-path"
				})

				It("returns an error", func() {
					err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
					Expect(err).To(MatchError(ContainSubstring("creating namespace file")))
				})
			})
		})

		Context("when there's a namespace file", func() {
			BeforeEach(func() {
				mappings := []byte(`{"uid-mappings":["0:1000:1","1:100000:10"],"gid-mappings":["0:2000:1","1:200000:10"]}`)
				Expect(os.WriteFile(filepath.Join(storePath, store.MetaDirName, "namespace.json"), mappings, 0700)).To(Succeed())
			})

			It("succeeds when the namespaces are the same", func() {
				Expect(storeNamespacer.ApplyMappings(uidMappings, gidMappings)).To(Succeed())
			})

			Context("when uid mapping doesn't match", func() {
				BeforeEach(func() {
					uidMappings[0].HostID = 8888
				})

				It("returns an error", func() {
					err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
					Expect(err).To(MatchError(ContainSubstring("provided UID mappings do not match those already configured in the store")))
				})
			})

			Context("when gid mapping doesn't match", func() {
				BeforeEach(func() {
					gidMappings[0].HostID = 8888
				})

				It("returns an error", func() {
					err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
					Expect(err).To(MatchError(ContainSubstring("provided GID mappings do not match those already configured in the store")))
				})
			})

			Context("when it fails to read the namespace file", func() {
				BeforeEach(func() {
					imageJsonPath := filepath.Join(storePath, store.MetaDirName, "namespace.json")
					Expect(os.Remove(imageJsonPath)).To(Succeed())
					Expect(os.Mkdir(imageJsonPath, 0755)).To(Succeed())
				})

				It("returns an error", func() {
					err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
					Expect(err).To(MatchError(ContainSubstring("reading namespace file")))
				})
			})
		})

		Context("when it fails to create the namespace file", func() {
			BeforeEach(func() {
				storePath = "invalid-path"
			})

			It("returns an error", func() {
				err := storeNamespacer.ApplyMappings(uidMappings, gidMappings)
				Expect(err).To(MatchError(ContainSubstring("creating namespace file")))
			})
		})
	})
})
