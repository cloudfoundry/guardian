package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
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
		depotDir               string
		bundleSaver            *fakes.FakeBundleSaver
		bundleGenerator        *fakes.FakeBundleGenerator
		bindMountSourceCreator *fakes.FakeBindMountSourceCreator
		dirdepot               *depot.DirectoryDepot
		logger                 lager.Logger
		bndle                  goci.Bndl
		desiredContainerSpec   spec.DesiredContainerSpec
	)

	BeforeEach(func() {
		var err error

		depotDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		desiredContainerSpec = spec.DesiredContainerSpec{Handle: "some-idiosyncratic-handle", Privileged: false}
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
		bindMountSourceCreator = new(fakes.FakeBindMountSourceCreator)
		dirdepot = depot.New(depotDir, bundleGenerator, bundleSaver, bindMountSourceCreator)
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
	})

	Describe("create", func() {
		It("should create a directory", func() {
			Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).To(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark")).To(BeADirectory())
		})

		It("creates bind mount sources", func() {
			Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).To(Succeed())
			Expect(bindMountSourceCreator.CreateCallCount()).To(Equal(1))
			actualContainerDir, actualChown := bindMountSourceCreator.CreateArgsForCall(0)
			Expect(actualContainerDir).To(Equal(filepath.Join(depotDir, "aardvaark")))
			Expect(actualChown).To(BeTrue())
		})

		Context("when bind mount creation fails", func() {
			It("destroys the container directory", func() {
				bindMountSourceCreator.CreateReturns(nil, errors.New("didn't work"))
				Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).NotTo(Succeed())
				Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
			})
		})

		It("generates the bundle", func() {
			mounts := []garden.BindMount{{
				DstPath: "/some/dest",
				SrcPath: "/some/src",
				Mode:    garden.BindMountModeRW,
			}}
			bindMountSourceCreator.CreateReturns(mounts, nil)
			bundleGenerator.GenerateReturns(bndle, nil)
			Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).To(Succeed())

			// Change expectation
			desiredContainerSpec.BindMounts = append(desiredContainerSpec.BindMounts, mounts...)

			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualDesiredSpec, actualContainerDir := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualDesiredSpec).To(Equal(desiredContainerSpec))
			Expect(actualContainerDir).To(Equal(filepath.Join(depotDir, "aardvaark")))
		})

		Context("when generation fails", func() {
			It("destroys the container directory", func() {
				bundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("didn't work"))
				Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).NotTo(Succeed())
				Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
			})
		})

		It("it saves the bundle", func() {
			bundleGenerator.GenerateReturns(bndle, nil)
			Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).To(Succeed())

			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualPath := bundleSaver.SaveArgsForCall(0)
			Expect(actualPath).To(Equal(filepath.Join(depotDir, "aardvaark")))
			Expect(actualBundle).To(Equal(bndle))
		})

		Context("when saving fails", func() {
			It("destroys the container directory", func() {
				bundleSaver.SaveReturns(errors.New("didn't work"))
				Expect(dirdepot.Create(logger, "aardvaark", desiredContainerSpec)).NotTo(Succeed())
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
				Expect(dirdepot.Create(logger, "banana", desiredContainerSpec)).To(Succeed())
				Expect(dirdepot.Create(logger, "banana2", desiredContainerSpec)).To(Succeed())
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
				invalidDepot = depot.New("rubbish", bundleGenerator, bundleSaver, bindMountSourceCreator)
			})

			It("returns an error", func() {
				_, err := invalidDepot.Handles()
				Expect(err).To(MatchError(ContainSubstring("invalid depot directory rubbish: open rubbish:")))
			})
		})
	})

	Describe("GetDir", func() {
		It("returns the depot dir", func() {
			dirDepot := depot.New("/path/to/depot", nil, nil, nil)
			Expect(dirDepot.GetDir()).To(Equal("/path/to/depot"))
		})
	})
})
