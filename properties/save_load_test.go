package properties_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/guardian/properties"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SaveLoad", func() {
	var (
		propPath string
	)

	BeforeEach(func() {
		var err error
		propPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(propPath)).To(Succeed())
	})

	It("returns a new manager when the file is not found", func() {
		mgr, err := properties.Load("/path/does/not/exist")
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
	})

	It("save and load the state of the property manager", func() {
		mgr := properties.NewManager()
		mgr.Set("foo", "bar", "baz")

		Expect(properties.Save(path.Join(propPath, "props.json"), mgr)).To(Succeed())
		newMgr, err := properties.Load(path.Join(propPath, "props.json"))
		Expect(err).NotTo(HaveOccurred())

		val, ok := newMgr.Get("foo", "bar")
		Expect(ok).To(BeTrue())
		Expect(val).To(Equal("baz"))
	})

	It("returns an error when decoding fails", func() {
		Expect(ioutil.WriteFile(path.Join(propPath, "props.json"), []byte("{teest: banana"), 0655)).To(Succeed())

		_, err := properties.Load(path.Join(propPath, "props.json"))
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when cannot write to the file", func() {
		mgr := properties.NewManager()
		Expect(properties.Save("/path/to/non/existing.json", mgr)).To(HaveOccurred())
	})
})
