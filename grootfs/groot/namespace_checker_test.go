package groot_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StoreNamespaceChecker", func() {
	var (
		storePath        string
		namespaceChecker *groot.StoreNamespaceChecker
		uidMappings      []groot.IDMappingSpec
		gidMappings      []groot.IDMappingSpec
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(storePath, store.MetaDirName), 0700)).To(Succeed())
	})

	JustBeforeEach(func() {
		namespaceChecker = groot.NewNamespaceChecker(storePath)
	})

	Describe("Check", func() {
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

		It("returns true when there's no namespace file", func() {
			check, err := namespaceChecker.Check(uidMappings, gidMappings)
			Expect(err).ToNot(HaveOccurred())
			Expect(check).To(BeTrue())
		})

		It("creates the correct namespace file", func() {
			_, err := namespaceChecker.Check(uidMappings, gidMappings)
			Expect(err).NotTo(HaveOccurred())

			namespaceFile := filepath.Join(storePath, store.MetaDirName, "namespace.json")
			Expect(namespaceFile).To(BeAnExistingFile())

			contents, err := ioutil.ReadFile(namespaceFile)
			Expect(err).NotTo(HaveOccurred())

			var namespaces map[string][]string
			Expect(json.Unmarshal(contents, &namespaces)).To(Succeed())

			Expect(namespaces["uid-mappings"]).To(Equal([]string{"0:1000:1", "1:100000:10"}))
			Expect(namespaces["gid-mappings"]).To(Equal([]string{"0:2000:1", "1:200000:10"}))
		})

		Context("when there's a namespace file", func() {
			BeforeEach(func() {
				mappings := []byte(`{"uid-mappings":["0:1000:1","1:100000:10"],"gid-mappings":["0:2000:1","1:200000:10"]}`)
				Expect(ioutil.WriteFile(filepath.Join(storePath, store.MetaDirName, "namespace.json"), mappings, 0700)).To(Succeed())
			})

			It("returns true when the namespaces are the same", func() {
				check, err := namespaceChecker.Check(uidMappings, gidMappings)
				Expect(err).ToNot(HaveOccurred())
				Expect(check).To(BeTrue())
			})

			Context("when uid mapping doesn't match", func() {
				BeforeEach(func() {
					uidMappings[0].HostID = 8888
				})

				It("returns false when the namepsaces are different", func() {
					check, err := namespaceChecker.Check(uidMappings, gidMappings)
					Expect(err).ToNot(HaveOccurred())
					Expect(check).To(BeFalse())
				})
			})

			Context("when gid mapping doesn't match", func() {
				BeforeEach(func() {
					gidMappings[0].HostID = 8888
				})

				It("returns false when the namepsaces are different", func() {
					check, err := namespaceChecker.Check(uidMappings, gidMappings)
					Expect(err).ToNot(HaveOccurred())
					Expect(check).To(BeFalse())
				})
			})

			Context("when it fails to read the namespace file", func() {
				BeforeEach(func() {
					imageJsonPath := filepath.Join(storePath, store.MetaDirName, "namespace.json")
					Expect(os.Remove(imageJsonPath)).To(Succeed())
					Expect(os.Mkdir(imageJsonPath, 0755)).To(Succeed())
				})

				It("returns an error", func() {
					_, err := namespaceChecker.Check(uidMappings, gidMappings)
					Expect(err).To(MatchError(ContainSubstring("reading namespace file")))
				})
			})
		})

		Context("when it fails to create the namespace file", func() {
			BeforeEach(func() {
				storePath = "invalid-path"
			})

			It("returns an error", func() {
				_, err := namespaceChecker.Check(uidMappings, gidMappings)
				Expect(err).To(MatchError(ContainSubstring("creating namespace file")))
			})
		})
	})
})
