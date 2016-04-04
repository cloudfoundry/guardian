package properties_test

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/cloudfoundry-incubator/guardian/properties"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MapPersister", func() {
	var (
		tmpDir    string
		filePath  string
		persister *properties.FilesystemPersister
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		filePath = path.Join(tmpDir, "properties.json")

		persister = &properties.FilesystemPersister{PersistenceDir: tmpDir}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("LoadMap", func() {
		var (
			props map[string]string
			err   error
		)

		It("should parse the provided file", func() {
			Expect(ioutil.WriteFile(filePath, []byte(`{
				"key": "value"
			}`), 0660)).To(Succeed())

			props, err = persister.LoadMap("properties.json")
			Expect(err).NotTo(HaveOccurred())

			Expect(props["key"]).To(Equal("value"))
		})

		Context("when the file does not exist", func() {

			BeforeEach(func() {
				props, err = persister.LoadMap("banana")
			})

			It("should return a wrapped error", func() {
				Expect(err).To(MatchError(ContainSubstring("opening file")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})

			It("should return an empty map", func() {
				Expect(props).NotTo(BeNil())
				Expect(props).To(HaveLen(0))
			})
		})

		Context("when the file contents are invalid", func() {

			BeforeEach(func() {
				Expect(ioutil.WriteFile(filePath, []byte(`{
				"spiderman": `), 0660)).To(Succeed())

				props, err = persister.LoadMap("properties.json")
			})

			It("should return a wrapped error", func() {
				Expect(err).To(MatchError(ContainSubstring("parsing file")))
				Expect(err).To(MatchError(ContainSubstring("unexpected EOF")))
			})

			It("should return an empty map", func() {
				Expect(props).NotTo(BeNil())
				Expect(props).To(HaveLen(0))
			})
		})
	})

	Describe("SaveMap", func() {

		var props = map[string]string{
			"will": "i am",
		}

		Context("when the file does not exist", func() {
			It("should write the file", func() {
				Expect(persister.SaveMap("properties.json", props)).To(Succeed())

				contents, err := ioutil.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(ContainSubstring("\"will\":\"i am\""))
			})
		})

		Context("when the file already exists", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filePath, []byte(`{ "spiderman": `), 0660)).To(Succeed())
			})

			It("should overwrite the file", func() {
				Expect(persister.SaveMap("properties.json", props)).To(Succeed())

				contents, err := ioutil.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(ContainSubstring("\"will\":\"i am\""))
				Expect(string(contents)).NotTo(ContainSubstring("spiderman"))
			})
		})

		Context("when the file cannot be created", func() {
			It("should return a sensible error", func() {
				err := persister.SaveMap("/path/to/my/basement/", props)
				Expect(err).To(MatchError(ContainSubstring("creating file")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})
	})

	Describe("DeleteMap", func() {
		It("Should delete the persisted file", func() {
			Expect(ioutil.WriteFile(filePath, []byte(`{
				"key": "value"
			}`), 0660)).To(Succeed())

			Expect(persister.DeleteMap("properties.json")).To(Succeed())
			Expect(filePath).NotTo(BeAnExistingFile())
		})

		Context("when the file does not exist", func() {
			It("should return a sensible error", func() {
				err := persister.DeleteMap("non-existent-spiderman.json")
				Expect(err).To(MatchError(ContainSubstring("deleting file")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})
	})

	Describe("IsMapPersisted", func() {
		Context("when the file exists", func() {
			It("should return true", func() {
				_, err := os.Create(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(persister.IsMapPersisted("properties.json")).To(BeTrue())
			})
		})

		Context("when the file does not exist", func() {
			It("should return false", func() {
				Expect(persister.IsMapPersisted("properties.json")).To(BeFalse())
			})
		})
	})
})
