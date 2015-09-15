package rundmc_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Depot", func() {
	var (
		tmpDir     string
		fakeBundle *fakes.FakeBundleCreator
		depot      *rundmc.DirectoryDepot
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		fakeBundle = new(fakes.FakeBundleCreator)
		depot = &rundmc.DirectoryDepot{
			Dir:           tmpDir,
			BundleCreator: fakeBundle,
		}
	})

	Describe("lookup", func() {
		Context("when a subdirectory with the given name does not exist", func() {
			It("returns an ErrDoesNotExist", func() {
				_, err := depot.Lookup("potato")
				Expect(err).To(MatchError(rundmc.ErrDoesNotExist))
			})
		})

		Context("when a subdirectory with the given name exists", func() {
			It("returns the absolute path of the directory", func() {
				os.Mkdir(filepath.Join(tmpDir, "potato"), 0700)
				Expect(depot.Lookup("potato")).To(Equal(filepath.Join(tmpDir, "potato")))
			})
		})
	})

	Describe("create", func() {
		It("should create a directory", func() {
			Expect(depot.Create("aardvaark")).To(Succeed())
			Expect(filepath.Join(tmpDir, "aardvaark")).To(BeADirectory())
		})

		It("should serialize the a container config to the directory", func() {
			Expect(depot.Create("aardvaark")).To(Succeed())
			Expect(fakeBundle.CreateCallCount()).To(Equal(1))
			Expect(fakeBundle.CreateArgsForCall(0)).To(Equal(path.Join(tmpDir, "aardvaark")))
		})

		It("destroys the container directory if creation fails", func() {
			fakeBundle.CreateReturns(errors.New("didn't work"))
			Expect(depot.Create("aardvaark")).NotTo(Succeed())
			Expect(filepath.Join(tmpDir, "aardvaark")).NotTo(BeADirectory())
		})
	})

	Describe("destroy", func() {
		It("should destroy the container directory", func() {
			Expect(os.MkdirAll(filepath.Join(tmpDir, "potato"), 0755)).To(Succeed())
			Expect(depot.Destroy("potato")).To(Succeed())
			Expect(filepath.Join(tmpDir, "potato")).NotTo(BeAnExistingFile())
		})

		Context("when the container directory does not exist", func() {
			It("does not error (i.e. the method is idempotent)", func() {
				Expect(depot.Destroy("potato")).To(Succeed())
			})
		})
	})
})
