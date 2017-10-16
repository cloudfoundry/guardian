package peas_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("PeaCreator", func() {
	var (
		bundleGenerator  *depotfakes.FakeBundleGenerator
		bundleSaver      *depotfakes.FakeBundleSaver
		execPreparer     *runruncfakes.FakeExecPreparer
		containerCreator *peasfakes.FakeContainerCreator
		peaCreator       *peas.PeaCreator
		ctrBundleDir     string
		log              *lagertest.TestLogger
		generatedBundle  = goci.Bndl{Spec: specs.Spec{Version: "our-bundle"}}
		rootfsPath       = "/some/rootfs"
	)

	BeforeEach(func() {
		bundleGenerator = new(depotfakes.FakeBundleGenerator)
		bundleGenerator.GenerateReturns(generatedBundle, nil)
		bundleSaver = new(depotfakes.FakeBundleSaver)
		execPreparer = new(runruncfakes.FakeExecPreparer)
		execPreparer.PrepareReturns(&runrunc.PreparedSpec{
			Process: specs.Process{Cwd: "some-cwd"},
		}, nil)
		containerCreator = new(peasfakes.FakeContainerCreator)
		peaCreator = &peas.PeaCreator{
			BundleGenerator:  bundleGenerator,
			BundleSaver:      bundleSaver,
			ExecPreparer:     execPreparer,
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
		var (
			peaID    string
			process  garden.Process
			exitCode int
			waitErr  error
		)

		BeforeEach(func() {
			peaID = "some-pea"
		})

		JustBeforeEach(func() {
			var err error
			process, err = peaCreator.CreatePea(log, garden.ProcessSpec{
				ID:    peaID,
				Image: garden.ImageRef{URI: "raw://" + rootfsPath},
			}, garden.ProcessIO{}, ctrBundleDir)
			Expect(err).NotTo(HaveOccurred())
			exitCode, waitErr = process.Wait()
		})

		It("creates the bundle directory", func() {
			Expect(filepath.Join(ctrBundleDir, "processes", peaID)).To(BeADirectory())
		})

		It("generates a runtime spec using a hardcoded empty rootfs path", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, actualCtrBundlePath := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec).To(Equal(gardener.DesiredContainerSpec{
				Handle:     "some-pea",
				Privileged: false,
				BaseConfig: specs.Spec{Root: &specs.Root{Path: rootfsPath}},
			}))
			Expect(actualCtrBundlePath).To(Equal(ctrBundleDir))
		})

		It("saves the initial bundle to disk", func() {
			Expect(bundleSaver.SaveCallCount()).To(BeNumerically(">=", 1))
			actualBundle, actualBundlePath := bundleSaver.SaveArgsForCall(0)
			Expect(actualBundle).To(Equal(generatedBundle))
			Expect(actualBundlePath).To(Equal(filepath.Join(ctrBundleDir, "processes", peaID)))
		})

		It("prepares the rootfs", func() {
			Expect(execPreparer.PrepareCallCount()).To(Equal(1))
			_, actualBundlePath, actualProcessSpec := execPreparer.PrepareArgsForCall(0)
			Expect(actualBundlePath).To(Equal(filepath.Join(ctrBundleDir, "processes", peaID)))
			Expect(actualProcessSpec).To(Equal(garden.ProcessSpec{
				ID:    peaID,
				Image: garden.ImageRef{URI: "raw://" + rootfsPath},
			}))
		})

		It("saves the bundle to disk again, adding the process", func() {
			Expect(bundleSaver.SaveCallCount()).To(Equal(2))
			actualBundle, actualBundlePath := bundleSaver.SaveArgsForCall(1)
			expectedBundle := generatedBundle.WithProcess(specs.Process{Cwd: "some-cwd"})
			Expect(actualBundle).To(Equal(expectedBundle))
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

		Context("when container creation succeeds and user process exits with non-zero code", func() {
			BeforeEach(func() {
				cmdThatFails := exec.Command("bash", "-c", "exit 42")
				if runtime.GOOS == "windows" {
					cmdThatFails = exec.Command("cmd", "/c", "exit 42")
				}

				realExitErr := cmdThatFails.Run()
				containerCreator.CreateReturns(logging.WrappedError{Underlying: realExitErr})
			})

			It("process.Wait returns the container creation error", func() {
				Expect(waitErr).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(42))
			})
		})

		Context("when the container creation fails", func() {
			BeforeEach(func() {
				containerCreator.CreateReturns(errors.New("mango"))
			})

			It("process.Wait returns the container creation error", func() {
				Expect(waitErr).To(MatchError("mango"))
			})
		})
	})

	Describe("pea creation failing", func() {
		var (
			imageURI  string
			createErr error
		)

		BeforeEach(func() {
			imageURI = "raw://" + rootfsPath
		})

		JustBeforeEach(func() {
			_, createErr = peaCreator.CreatePea(log, garden.ProcessSpec{
				Image: garden.ImageRef{URI: imageURI},
			}, garden.ProcessIO{}, ctrBundleDir)
		})

		Context("when the rootfs URI does not have scheme: raw", func() {
			BeforeEach(func() {
				imageURI = "/some/path"
			})

			It("returns an error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("expected scheme 'raw', got ''")))
			})
		})

		Context("when the bundle generator returns an error", func() {
			BeforeEach(func() {
				bundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("banana"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("banana")))
			})
		})

		Context("when the exec preparer returns an error", func() {
			BeforeEach(func() {
				execPreparer.PrepareReturns(nil, errors.New("jackfruit"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("jackfruit")))
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
	})
})
