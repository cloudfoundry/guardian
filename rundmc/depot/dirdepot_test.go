package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	fakes "code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Depot", func() {
	var (
		depotDir        string
		bundleSaver     *fakes.FakeBundleSaver
		bundleGenerator *fakes.FakeBundleGenerator
		dirdepot        *depot.DirectoryDepot
		logger          lager.Logger
		bndle           goci.Bndl
		spec            gardener.DesiredContainerSpec
	)

	BeforeEach(func() {
		var err error

		depotDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		spec = gardener.DesiredContainerSpec{Handle: "some-idiosyncratic-handle"}
		bndle = goci.Bndl{Spec: specs.Spec{Version: "some-idiosyncratic-version", Linux: &specs.Linux{}}}
		bndle = bndle.WithUIDMappings(
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

		bundleSaver = new(fakes.FakeBundleSaver)
		bundleGenerator = new(fakes.FakeBundleGenerator)
		dirdepot = depot.New(depotDir, bundleGenerator, bundleSaver)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depotDir)).To(Succeed())
	})

	Describe("lookup", func() {
		Context("when a subdirectory with the given name does not exist", func() {
			It("returns an ErrDoesNotExist", func() {
				_, err := dirdepot.Lookup(logger, "potato")
				Expect(err).To(MatchError(depot.ErrDoesNotExist))
			})
		})

		Context("when a subdirectory with the given name exists", func() {
			It("returns the absolute path of the directory", func() {
				os.Mkdir(filepath.Join(depotDir, "potato"), 0700)
				Expect(dirdepot.Lookup(logger, "potato")).To(Equal(filepath.Join(depotDir, "potato")))
			})
		})
	})

	Describe("create", func() {
		It("should create a directory", func() {
			Expect(dirdepot.Create(logger, "aardvaark", spec)).To(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark")).To(BeADirectory())
		})

		It("it saves the bundle", func() {
			bundleGenerator.GenerateReturns(bndle, nil)
			Expect(dirdepot.Create(logger, "aardvaark", spec)).To(Succeed())

			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualPath := bundleSaver.SaveArgsForCall(0)
			Expect(actualPath).To(Equal(filepath.Join(depotDir, "aardvaark")))
			Expect(actualBundle).To(Equal(bndle))
		})

		It("generates the bundle", func() {
			bundleGenerator.GenerateReturns(bndle, nil)
			Expect(dirdepot.Create(logger, "aardvaark", spec)).To(Succeed())

			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualDesiredSpec, actualContainerDir := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualDesiredSpec).To(Equal(spec))
			Expect(actualContainerDir).To(Equal(filepath.Join(depotDir, "aardvaark")))
		})

		Context("when creation fails", func() {
			It("destroys the container directory if creation fails", func() {
				bundleSaver.SaveReturns(errors.New("didn't work"))
				Expect(dirdepot.Create(logger, "aardvaark", spec)).NotTo(Succeed())
				Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
			})
		})
		Context("when generation fails", func() {
			It("destroys the container directory if creation fails", func() {
				bundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("didn't work"))
				Expect(dirdepot.Create(logger, "aardvaark", spec)).NotTo(Succeed())
				Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
			})
		})
	})

	Describe("destroy", func() {
		It("should destroy the container directory", func() {
			Expect(os.MkdirAll(filepath.Join(depotDir, "potato"), 0755)).To(Succeed())
			Expect(dirdepot.Destroy(logger, "potato")).To(Succeed())
			Expect(filepath.Join(depotDir, "potato")).NotTo(BeAnExistingFile())
		})

		Context("when the container directory does not exist", func() {
			It("does not error (i.e. the method is idempotent)", func() {
				Expect(dirdepot.Destroy(logger, "potato")).To(Succeed())
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				Expect(dirdepot.Create(logger, "banana", spec)).To(Succeed())
				Expect(dirdepot.Create(logger, "banana2", spec)).To(Succeed())
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
				invalidDepot = depot.New("rubbish", bundleGenerator, bundleSaver)
			})

			It("returns an error", func() {
				_, err := invalidDepot.Handles()
				Expect(err).To(MatchError(ContainSubstring("invalid depot directory rubbish: open rubbish:")))
			})
		})
	})
})
