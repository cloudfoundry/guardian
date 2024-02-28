package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/depot"
	fakes "code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Depot", func() {
	var (
		depotDir string
		dirdepot *depot.DirectoryDepot
		logger   lager.Logger
		bundle   goci.Bndl
	)

	BeforeEach(func() {
		var err error

		depotDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		bundle = goci.Bndl{Spec: specs.Spec{Version: "some-idiosyncratic-version", Linux: &specs.Linux{}}}
		bundle = bundle.WithUIDMappings(
			specs.LinuxIDMapping{
				HostID:      14,
				ContainerID: 1,
				Size:        1,
			},
			specs.LinuxIDMapping{
				HostID:      15,
				ContainerID: 0,
				Size:        1,
			},
			specs.LinuxIDMapping{
				HostID:      16,
				ContainerID: 3,
				Size:        1,
			},
		).
			WithGIDMappings(
				specs.LinuxIDMapping{
					HostID:      42,
					ContainerID: 0,
					Size:        17,
				},
				specs.LinuxIDMapping{
					HostID:      43,
					ContainerID: 1,
					Size:        17,
				},
			)

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		dirdepot = depot.New(depotDir, nil, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depotDir)).To(Succeed())
	})

	var (
		bundleSaver  *fakes.FakeBundleSaver
		bundleLoader *fakes.FakeBundleLoader
	)

	BeforeEach(func() {
		bundleSaver = new(fakes.FakeBundleSaver)
		bundleLoader = new(fakes.FakeBundleLoader)
	})

	JustBeforeEach(func() {
		dirdepot = depot.New(depotDir, bundleSaver, bundleLoader)
	})

	Describe("lookup", func() {
		Context("when a subdirectory with the given name does not exist", func() {
			It("returns an ErrDoesNotExist", func() {
				_, err := dirdepot.Lookup(logger, "potato")
				Expect(err).To(MatchError(depot.ErrDoesNotExist))
			})
		})
	})

	Describe("created time", func() {
		It("returns the approximate creation time of the container", func() {
			_, err := dirdepot.Create(logger, "potato", bundle)
			Expect(err).NotTo(HaveOccurred())
			f, err := os.Create(filepath.Join(depotDir, "potato", "pidfile"))
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()
			time.Sleep(time.Millisecond)
			ctime, err := dirdepot.CreatedTime(logger, "potato")
			Expect(err).NotTo(HaveOccurred())
			Expect(ctime).To(BeTemporally("<", time.Now()))
			Expect(ctime).To(BeTemporally(">", time.Now().Add(-time.Millisecond*500)))
		})

		Context("when the bundle is not there", func() {
			It("fails", func() {
				_, err := dirdepot.CreatedTime(logger, "sweetpotato")
				Expect(err).To(MatchError("does not exist"))
			})
		})

		Context("when the bundle pidfile is not there", func() {
			It("fails", func() {
				_, err := dirdepot.Create(logger, "potato", bundle)
				Expect(err).NotTo(HaveOccurred())
				_, err = dirdepot.CreatedTime(logger, "potato")
				Expect(err).To(MatchError(ContainSubstring("bundle pidfile does not exist")))
			})
		})
	})

	Describe("create", func() {
		It("should create a directory", func() {
			_, err := dirdepot.Create(logger, "aardvaark", bundle)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Join(depotDir, "aardvaark")).To(BeADirectory())
		})

		It("it saves the bundle", func() {
			_, err := dirdepot.Create(logger, "aardvaark", bundle)
			Expect(err).NotTo(HaveOccurred())

			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualPath := bundleSaver.SaveArgsForCall(0)
			Expect(actualPath).To(Equal(filepath.Join(depotDir, "aardvaark")))
			Expect(actualBundle).To(Equal(bundle))
		})

		Context("when saving fails", func() {
			It("destroys the container directory", func() {
				bundleSaver.SaveReturns(errors.New("didn't work"))
				_, err := dirdepot.Create(logger, "aardvaark", bundle)
				Expect(err).To(MatchError("didn't work"))
				Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
			})
		})
	})

	Describe("destroy", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(depotDir, "potato"), 0755)).To(Succeed())
		})

		It("should destroy the container directory", func() {
			Expect(dirdepot.Destroy(logger, "potato")).To(Succeed())
			Expect(filepath.Join(depotDir, "potato")).NotTo(BeAnExistingFile())
		})

		Context("when the container directory does not exist", func() {
			It("does not error (i.e. the method is idempotent)", func() {
				Expect(dirdepot.Destroy(logger, "banana")).To(Succeed())
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(depotDir, "banana"), 0755)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(depotDir, "banana2"), 0755)).To(Succeed())
			})

			It("should return the handles", func() {
				Expect(dirdepot.Handles()).To(ConsistOf("banana", "banana2"))
			})
		})

		Context("when no handles exist", func() {
			It("should return an empty list", func() {
				Expect(dirdepot.Handles()).To(BeEmpty())
			})
		})

		Context("when the depot directory does not exist", func() {
			var invalidDepot *depot.DirectoryDepot

			BeforeEach(func() {
				invalidDepot = depot.New("rubbish", nil, nil)
			})

			It("returns an error", func() {
				_, err := invalidDepot.Handles()
				Expect(err).To(MatchError(ContainSubstring("invalid depot directory rubbish: open rubbish:")))
			})
		})
	})

	Describe("GetDir", func() {
		BeforeEach(func() {
			depotDir = "/path/to/depot"
		})

		It("returns the depot dir", func() {
			Expect(dirdepot.GetDir()).To(Equal("/path/to/depot"))
		})
	})

	Describe("Load", func() {
		var (
			loadErr    error
			bundlePath string
		)

		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(depotDir, "container-handle"), 0755)).To(Succeed())
			bundlePath = filepath.Join(depotDir, "container-handle")
		})

		JustBeforeEach(func() {
			_, loadErr = dirdepot.Load(logger, "container-handle")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(bundlePath))
		})

		It("loads the bundle", func() {
			Expect(bundleLoader.LoadCallCount()).To(Equal(1))
			actualBundlePath := bundleLoader.LoadArgsForCall(0)
			Expect(actualBundlePath).To(Equal(bundlePath))
		})

		Context("when loading the bundle fails", func() {
			BeforeEach(func() {
				bundleLoader.LoadReturns(goci.Bndl{}, errors.New("hey"))
			})

			It("fails", func() {
				Expect(loadErr).To(MatchError("hey"))
			})
		})
	})
})
