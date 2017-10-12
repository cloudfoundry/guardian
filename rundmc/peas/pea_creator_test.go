package peas_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("PeaCreator", func() {
	var (
		bundleGenerator  *depotfakes.FakeBundleGenerator
		bundleSaver      *depotfakes.FakeBundleSaver
		containerCreator *peasfakes.FakeContainerCreator
		peaCreator       *peas.PeaCreator
		ctrBundleDir     string
		log              *lagertest.TestLogger
		generatedBundle  = goci.Bndl{Spec: specs.Spec{Version: "our-bundle"}}
	)

	BeforeEach(func() {
		bundleGenerator = new(depotfakes.FakeBundleGenerator)
		bundleGenerator.GenerateReturns(generatedBundle, nil)
		bundleSaver = new(depotfakes.FakeBundleSaver)
		containerCreator = new(peasfakes.FakeContainerCreator)
		peaCreator = &peas.PeaCreator{
			BundleGenerator:  bundleGenerator,
			BundleSaver:      bundleSaver,
			ContainerCreator: containerCreator,
		}
		var err error
		ctrBundleDir, err = ioutil.TempDir("", "pea-creator-tests")
		Expect(err).NotTo(HaveOccurred())
		log = lagertest.NewTestLogger("peas-unit-tests")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(ctrBundleDir)).To(Succeed())
	})

	Describe("pea creation succeeding", func() {
		var peaID string
		var process garden.Process

		BeforeEach(func() {
			peaID = "some-pea"
		})

		JustBeforeEach(func() {
			var err error
			process, err = peaCreator.CreatePea(log, garden.ProcessSpec{ID: peaID}, ctrBundleDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates the bundle directory", func() {
			Expect(filepath.Join(ctrBundleDir, "processes", peaID)).To(BeADirectory())
		})

		It("generates a runtime spec using a hardcoded empty rootfs path", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec).To(Equal(gardener.DesiredContainerSpec{
				Handle:     "some-pea",
				Privileged: false,
				BaseConfig: specs.Spec{Root: &specs.Root{Path: peas.RootfsPath}},
			}))
		})

		It("creates the hardcoded empty rootfs dir, writeable by container root", func() {
			fileInfo, err := os.Stat(peas.RootfsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(fileInfo.Mode() & os.ModePerm).To(Equal(os.FileMode(0777)))
		})

		It("saves the bundle to disk", func() {
			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualBundlePath := bundleSaver.SaveArgsForCall(0)
			Expect(actualBundle).To(Equal(generatedBundle))
			Expect(actualBundlePath).To(Equal(filepath.Join(ctrBundleDir, "processes", peaID)))
		})

		It("creates a runc container based on the bundle", func() {
			Expect(containerCreator.CreateCallCount()).To(Equal(1))
			_, actualBundlePath, actualContainerID, _ := containerCreator.CreateArgsForCall(0)
			Expect(actualBundlePath).To(Equal(filepath.Join(ctrBundleDir, "processes", peaID)))
			Expect(actualContainerID).To(Equal(peaID))
		})

		It("returns process with expected ID", func() {
			Expect(process.ID()).To(Equal(peaID))
		})

		Context("when the process spec has no ID", func() {
			BeforeEach(func() {
				peaID = ""
			})

			It("generates process ID", func() {
				processDirs, err := ioutil.ReadDir(filepath.Join(ctrBundleDir, "processes"))
				Expect(err).NotTo(HaveOccurred())
				Expect(processDirs).To(HaveLen(1))
			})
		})
	})

	Describe("pea creation failing", func() {
		var createErr error

		JustBeforeEach(func() {
			_, createErr = peaCreator.CreatePea(log, garden.ProcessSpec{}, ctrBundleDir)
		})

		Context("when the bundle generator returns an error", func() {
			BeforeEach(func() {
				bundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("banana"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("banana")))
			})
		})

		Context("when the bundle saver returns an error", func() {
			BeforeEach(func() {
				bundleSaver.SaveReturns(errors.New("papaya"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("papaya")))
			})
		})

		Context("when the container creator returns an error", func() {
			BeforeEach(func() {
				containerCreator.CreateReturns(errors.New("durian"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("durian")))
			})
		})
	})
})
