package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Depot", func() {
	var (
		tmpDir     string
		fakeBundle *fakes.FakeBundleCreator
		dirdepot   *depot.DirectoryDepot
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		fakeBundle = new(fakes.FakeBundleCreator)
		dirdepot = depot.New(tmpDir)
	})

	Describe("lookup", func() {
		Context("when a subdirectory with the given name does not exist", func() {
			It("returns an ErrDoesNotExist", func() {
				_, err := dirdepot.Lookup("potato")
				Expect(err).To(MatchError(depot.ErrDoesNotExist))
			})
		})

		Context("when a subdirectory with the given name exists", func() {
			It("returns the absolute path of the directory", func() {
				os.Mkdir(filepath.Join(tmpDir, "potato"), 0700)
				Expect(dirdepot.Lookup("potato")).To(Equal(filepath.Join(tmpDir, "potato")))
			})
		})
	})

	Describe("create", func() {
		It("should create a directory", func() {
			Expect(dirdepot.Create("aardvaark", fakeBundle)).To(Succeed())
			Expect(filepath.Join(tmpDir, "aardvaark")).To(BeADirectory())
		})

		It("should serialize the a container config to the directory", func() {
			Expect(dirdepot.Create("aardvaark", fakeBundle)).To(Succeed())
			Expect(fakeBundle.SaveCallCount()).To(Equal(1))
			Expect(fakeBundle.SaveArgsForCall(0)).To(Equal(path.Join(tmpDir, "aardvaark")))
		})

		It("destroys the container directory if creation fails", func() {
			fakeBundle.SaveReturns(errors.New("didn't work"))
			Expect(dirdepot.Create("aardvaark", fakeBundle)).NotTo(Succeed())
			Expect(filepath.Join(tmpDir, "aardvaark")).NotTo(BeADirectory())
		})
	})

	Describe("destroy", func() {
		It("should destroy the container directory", func() {
			Expect(os.MkdirAll(filepath.Join(tmpDir, "potato"), 0755)).To(Succeed())
			Expect(dirdepot.Destroy("potato")).To(Succeed())
			Expect(filepath.Join(tmpDir, "potato")).NotTo(BeAnExistingFile())
		})

		Context("when the container directory does not exist", func() {
			It("does not error (i.e. the method is idempotent)", func() {
				Expect(dirdepot.Destroy("potato")).To(Succeed())
			})
		})
	})
})
