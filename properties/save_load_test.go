package properties_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/properties"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SaveLoad", func() {
	var (
		workDir  string
		propPath string
	)

	BeforeEach(func() {
		workDir = tempDir("", "")
		propPath = filepath.Join(workDir, "props.json")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(workDir)).To(Succeed())
	})

	It("returns a new manager when the file is not found", func() {
		mgr, err := properties.Load("/path/does/not/exist")
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
	})

	It("returns a new manager when the file is empty", func() {
		Expect(ioutil.WriteFile(propPath, []byte{}, 0755)).To(Succeed())
		mgr, err := properties.Load(propPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
	})

	It("save and load the state of the property manager", func() {
		mgr := properties.NewManager()
		mgr.Set("foo", "bar", "baz")

		Expect(properties.Save(propPath, mgr)).To(Succeed())
		newMgr, err := properties.Load(propPath)
		Expect(err).NotTo(HaveOccurred())

		val, ok := newMgr.Get("foo", "bar")
		Expect(ok).To(BeTrue())
		Expect(val).To(Equal("baz"))
	})

	It("returns an error when decoding fails", func() {
		writeFileString(propPath, "{teest: banana", 0655)

		_, err := properties.Load(propPath)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when cannot write to the file", func() {
		mgr := properties.NewManager()
		Expect(properties.Save("/path/to/non/existing.json", mgr)).To(HaveOccurred())
	})
})
