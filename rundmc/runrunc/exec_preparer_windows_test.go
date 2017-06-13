package runrunc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsExecPreparer", func() {
	var (
		prepSpec   *runrunc.PreparedSpec
		procSpec   garden.ProcessSpec
		bundlePath string
	)

	BeforeEach(func() {
		procSpec = garden.ProcessSpec{Path: "foo", Args: []string{"bar", "baz"}}
	})

	JustBeforeEach(func() {
		var err error
		execPreparer := runrunc.WindowsExecPreparer{}

		bundlePath, err = ioutil.TempDir("", "windows-preparer.bundle")
		Expect(err).NotTo(HaveOccurred())

		prepSpec, err = execPreparer.Prepare(lagertest.NewTestLogger("windows-exec-preparer"), bundlePath, procSpec)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	It("returns a PreparedSpec containing the executable and args", func() {
		Expect(prepSpec.Process.Args).To(Equal([]string{"foo", "bar", "baz"}))
	})

	It("sets the default Cwd to C:\\", func() {
		Expect(prepSpec.Process.Cwd).To(Equal("C:\\"))
	})

	Context("process spec dir does not exist", func() {
		BeforeEach(func() {
			procSpec = garden.ProcessSpec{Dir: "c:\\dir1\\dir2"}
		})

		It("is created", func() {
			Expect(filepath.Join(bundlePath, "mnt", "dir1", "dir2")).To(BeADirectory())
			Expect(prepSpec.Process.Cwd).To(Equal("c:\\dir1\\dir2"))
		})
	})
})
