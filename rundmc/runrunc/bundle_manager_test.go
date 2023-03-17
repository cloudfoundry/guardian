package runrunc_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("BundleManager", func() {
	var (
		bundleManager    *BundleManager
		fakeDepot        *runruncfakes.FakeDepot
		fakeProcessDepot *runruncfakes.FakeProcessDepot
		logger           *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeDepot = new(runruncfakes.FakeDepot)
		fakeProcessDepot = new(runruncfakes.FakeProcessDepot)
		bundleManager = NewBundleManager(fakeDepot, fakeProcessDepot)
	})

	Describe("BundleInfo", func() {
		var (
			bundlePath string
			bundle     goci.Bndl
			err        error
		)

		BeforeEach(func() {
			fakeDepot.LookupReturns("/the/bundle/path", nil)
			fakeDepot.LoadReturns(goci.Bndl{Spec: specs.Spec{Version: "my-bundle"}}, nil)
		})

		JustBeforeEach(func() {
			bundlePath, bundle, err = bundleManager.BundleInfo(logger, "my-container")
		})

		It("returns the bundle for the specified container", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(Equal("/the/bundle/path"))
			Expect(bundle.Spec.Version).To(Equal("my-bundle"))

			Expect(fakeDepot.LookupCallCount()).To(Equal(1))
			lookupLogger, lookupHandle := fakeDepot.LookupArgsForCall(0)
			Expect(lookupLogger).To(Equal(logger))
			Expect(lookupHandle).To(Equal("my-container"))

			Expect(fakeDepot.LoadCallCount()).To(Equal(1))
			loadLogger, loadHandle := fakeDepot.LoadArgsForCall(0)
			Expect(loadLogger).To(Equal(logger))
			Expect(loadHandle).To(Equal("my-container"))
		})

		When("the container does not exist", func() {
			BeforeEach(func() {
				fakeDepot.LookupReturns("", depot.ErrDoesNotExist)
			})

			It("returns a garden.ContainerNotFoundError", func() {
				Expect(err).To(Equal(garden.ContainerNotFoundError{Handle: "my-container"}))
			})
		})

		When("looking up the bundle path fails", func() {
			BeforeEach(func() {
				fakeDepot.LookupReturns("", errors.New("lookup-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("lookup-error"))
			})
		})

		When("loading the bundle path fails", func() {
			BeforeEach(func() {
				fakeDepot.LoadReturns(goci.Bndl{}, errors.New("load-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("load-error"))
			})
		})
	})

	Describe("ContainerHandles", func() {
		var (
			bundleIDs []string
			err       error
		)

		BeforeEach(func() {
			fakeDepot.HandlesReturns([]string{"banana", "banana2"}, nil)
		})

		JustBeforeEach(func() {
			bundleIDs, err = bundleManager.ContainerHandles()
		})

		It("returns the list of bundleIDs", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(bundleIDs).To(ConsistOf("banana", "banana2"))
		})

		When("getting the list of bundleIDs from the depot fails", func() {
			BeforeEach(func() {
				fakeDepot.HandlesReturns(nil, errors.New("handles-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("handles-error"))
			})
		})
	})

	Describe("ContainerPeaHandles", func() {
		var (
			processesDir string
			peasErr      error
			peas         []string
		)

		BeforeEach(func() {
			var err error
			processesDir, err = ioutil.TempDir("", "processesDir")
			Expect(err).NotTo(HaveOccurred())

			peaDir := filepath.Join(processesDir, "pea-handle")
			Expect(os.MkdirAll(peaDir, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(peaDir, "config.json"), []byte("don't care"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(peaDir, "pidfile"), []byte("9988"), 0755)).To(Succeed())

			nonPeaDir := filepath.Join(processesDir, "non-pea-handle")
			Expect(os.MkdirAll(nonPeaDir, 0755)).To(Succeed())

			fakeProcessDepot.ListProcessDirsReturns([]string{peaDir, nonPeaDir}, nil)
		})

		JustBeforeEach(func() {
			peas, peasErr = bundleManager.ContainerPeaHandles(logger, "some-handle")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(processesDir)).To(Succeed())
		})

		It("returns the pea handles", func() {
			Expect(peasErr).NotTo(HaveOccurred())
			Expect(peas).To(ConsistOf("pea-handle"))
		})

		Context("when listing process dirs fails", func() {
			BeforeEach(func() {
				fakeProcessDepot.ListProcessDirsReturns([]string{}, errors.New("boohoo"))
			})

			It("propagates the error", func() {
				Expect(peasErr).To(MatchError("boohoo"))
			})
		})

		Context("when the pidfile does not exist in the process directory", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(processesDir, "pea-handle", "pidfile"))).To(Succeed())
			})

			It("does not return the pea handle", func() {
				Expect(peas).To(BeEmpty())
			})
		})

		Context("when config.json does not exist in the process directory", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(processesDir, "pea-handle", "config.json"))).To(Succeed())
			})

			It("does not return the pea handle", func() {
				Expect(peas).To(BeEmpty())
			})
		})
	})

	Describe("RemoveBundle", func() {
		var err error

		JustBeforeEach(func() {
			err = bundleManager.RemoveBundle(nil, "testHandle")
		})

		It("removes the bundle forim the depot", func() {
			Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
			_, handle := fakeDepot.DestroyArgsForCall(0)
			Expect(handle).To(Equal("testHandle"))
		})

		When("destroying the bundle errors", func() {
			BeforeEach(func() {
				fakeDepot.DestroyReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("boom"))
			})
		})
	})
})
