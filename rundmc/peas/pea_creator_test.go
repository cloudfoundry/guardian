package peas_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("PeaCreator", func() {
	const imageURI = "some-image-uri"

	var (
		volumizer              *peasfakes.FakeVolumizer
		peaCleaner             *gardenerfakes.FakePeaCleaner
		runcDeleter            *peasfakes.FakeRuncDeleter
		pidGetter              *peasfakes.FakePidGetter
		bindMountSourceCreator *depotfakes.FakeBindMountSourceCreator
		bundleGenerator        *depotfakes.FakeBundleGenerator
		bundleSaver            *depotfakes.FakeBundleSaver
		processBuilder         *runruncfakes.FakeProcessBuilder
		execRunner             *runruncfakes.FakeExecRunner
		privilegedGetter       *peasfakes.FakePrivilegedGetter

		peaCreator *peas.PeaCreator

		ctrHandle    string
		ctrBundleDir string
		log          *lagertest.TestLogger

		generatedBundle   = goci.Bndl{Spec: specs.Spec{Version: "our-bundle"}}
		defaultBindMounts = []garden.BindMount{{
			SrcPath: "/some/src",
			DstPath: "/some/dest",
			Mode:    garden.BindMountModeRO,
		}}
		specifiedBindMounts = []garden.BindMount{{
			SrcPath: "/some/src2",
			DstPath: "/some/dest2",
			Mode:    garden.BindMountModeRW,
		}}
		builtProcess *specs.Process
		processSpec  garden.ProcessSpec
		pio          = garden.ProcessIO{Stdin: bytes.NewBufferString("something")}
	)

	BeforeEach(func() {
		volumizer = new(peasfakes.FakeVolumizer)
		peaCleaner = new(gardenerfakes.FakePeaCleaner)
		volumizer.CreateReturns(specs.Spec{Version: "some-spec-version"}, nil)
		runcDeleter = new(peasfakes.FakeRuncDeleter)
		pidGetter = new(peasfakes.FakePidGetter)
		pidGetter.GetPidReturns(123, nil)
		bindMountSourceCreator = new(depotfakes.FakeBindMountSourceCreator)
		bindMountSourceCreator.CreateReturns(defaultBindMounts, nil)

		bundleGenerator = new(depotfakes.FakeBundleGenerator)
		bundleGenerator.GenerateReturns(generatedBundle, nil)
		bundleSaver = new(depotfakes.FakeBundleSaver)

		processBuilder = new(runruncfakes.FakeProcessBuilder)
		builtProcess = &specs.Process{Cwd: "some-cwd"}
		processBuilder.BuildProcessReturns(builtProcess)

		execRunner = new(runruncfakes.FakeExecRunner)

		privilegedGetter = new(peasfakes.FakePrivilegedGetter)
		privilegedGetter.PrivilegedReturns(false, nil)

		peaCreator = &peas.PeaCreator{
			Volumizer:              volumizer,
			PidGetter:              pidGetter,
			BindMountSourceCreator: bindMountSourceCreator,
			BundleGenerator:        bundleGenerator,
			BundleSaver:            bundleSaver,
			ProcessBuilder:         processBuilder,
			ExecRunner:             execRunner,
			PrivilegedGetter:       privilegedGetter,
			RuncDeleter:            runcDeleter,
			PeaCleaner:             peaCleaner,
		}

		var err error
		ctrHandle = "pea-creator-tests"
		ctrBundleDir, err = ioutil.TempDir("", "pea-creator-tests")
		Expect(err).NotTo(HaveOccurred())
		log = lagertest.NewTestLogger("peas-unit-tests")
		processSpec = garden.ProcessSpec{
			ID:   "some-id",
			Dir:  "/some/dir",
			User: "4:5",
			Image: garden.ImageRef{
				URI:      imageURI,
				Username: "cakeuser",
				Password: "cakepassword",
			},
			BindMounts: specifiedBindMounts,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(ctrBundleDir)).To(Succeed())
	})

	Describe("pea creation succeeding", func() {
		JustBeforeEach(func() {
			// We don't bother testing that some fake garden.Process is returned by
			// the mock ExecRunner, we leave this verification to our integration
			// tests.
			_, err := peaCreator.CreatePea(log, processSpec, pio, ctrHandle, ctrBundleDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates the bundle directory", func() {
			Expect(filepath.Join(ctrBundleDir, "processes", processSpec.ID)).To(BeADirectory())
		})

		It("checks the sandbox container's privilege", func() {
			Expect(privilegedGetter.PrivilegedCallCount()).To(Equal(1))
			actualBundlePath := privilegedGetter.PrivilegedArgsForCall(0)
			Expect(actualBundlePath).To(Equal(ctrBundleDir))
		})

		It("creates a volume", func() {
			Expect(volumizer.CreateCallCount()).To(Equal(1))
			_, actualSpec := volumizer.CreateArgsForCall(0)
			Expect(actualSpec.Handle).To(Equal(processSpec.ID))
			Expect(actualSpec.Image).To(Equal(garden.ImageRef{
				URI:      imageURI,
				Username: "cakeuser",
				Password: "cakepassword",
			}))
			Expect(actualSpec.Privileged).To(Equal(false))
		})

		It("creates the sources for bind mounts", func() {
			Expect(bindMountSourceCreator.CreateCallCount()).To(Equal(1))
			actualContainerDir, actualChown := bindMountSourceCreator.CreateArgsForCall(0)
			Expect(actualContainerDir).To(Equal(ctrBundleDir))
			Expect(actualChown).To(BeTrue())
		})

		It("passes bind mounts to bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.BindMounts).To(Equal(append(specifiedBindMounts, defaultBindMounts...)))
		})

		It("passes the processID as handle to the bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.Handle).To(Equal(processSpec.ID))
		})

		It("generates a runtime spec from the VolumeCreator's runtimeSpec", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.BaseConfig).To(Equal(specs.Spec{
				Version: "some-spec-version",
				Windows: &specs.Windows{
					Network: &specs.WindowsNetwork{
						NetworkSharedContainerName: "pea-creator-tests",
					},
				},
			}))
		})

		It("passes the container handle as cgroup path to the bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.CgroupPath).To(Equal(ctrHandle))
		})

		It("passes sandbox handle to bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.BaseConfig.Windows.Network.NetworkSharedContainerName).To(Equal(ctrHandle))
		})

		It("passes Privileged to bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrSpec.Privileged).To(Equal(false))
		})

		Context("When NestedCgroups=true", func() {
			BeforeEach(func() {
				peaCreator.NestedCgroups = true
			})

			It("passes <container-handle>/<process-id> as cgroup path to the bundle generator", func() {
				Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
				actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
				expected := filepath.Join(ctrHandle, processSpec.ID)
				Expect(actualCtrSpec.CgroupPath).To(Equal(expected))
			})
		})

		Describe("sharing namespaces", func() {
			It("shares all namespaces apart from mnt with the container", func() {
				Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
				actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
				Expect(actualCtrSpec.Namespaces).To(Equal(map[string]string{
					"mount":   "",
					"network": "/proc/123/ns/net",
					"user":    "/proc/123/ns/user",
					"ipc":     "/proc/123/ns/ipc",
					"pid":     "/proc/123/ns/pid",
					"uts":     "/proc/123/ns/uts",
				}))
			})

			Context("when sandbox container is privileged", func() {
				BeforeEach(func() {
					privilegedGetter.PrivilegedReturns(true, nil)
				})

				It("shares all namespaces apart from mnt and user with the container", func() {
					Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
					actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
					Expect(actualCtrSpec.Namespaces).To(Equal(map[string]string{
						"mount":   "",
						"network": "/proc/123/ns/net",
						"ipc":     "/proc/123/ns/ipc",
						"pid":     "/proc/123/ns/pid",
						"uts":     "/proc/123/ns/uts",
					}))
				})
			})
		})

		It("passes the ctrBundlePath to the bundle generator", func() {
			Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
			_, actualCtrBundle := bundleGenerator.GenerateArgsForCall(0)
			Expect(actualCtrBundle).To(Equal(ctrBundleDir))
		})

		It("builds a process", func() {
			Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
			actualBundle, actualProcessSpec, actualContainerUID, actualContainerGID := processBuilder.BuildProcessArgsForCall(0)
			Expect(actualBundle).To(Equal(generatedBundle))
			Expect(actualProcessSpec).To(Equal(processSpec))
			Expect(actualContainerUID).To(Equal(4))
			Expect(actualContainerGID).To(Equal(5))
		})

		It("saves the bundle (containing the built process) to disk", func() {
			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualBundlePath := bundleSaver.SaveArgsForCall(0)
			expectedBundle := generatedBundle.WithProcess(specs.Process{Cwd: "some-cwd"})
			Expect(actualBundle).To(Equal(expectedBundle))
			Expect(actualBundlePath).To(Equal(filepath.Join(ctrBundleDir, "processes", processSpec.ID)))
		})

		It("creates a runc container based on the bundle", func() {
			Eventually(execRunner.RunCallCount()).Should(Equal(1))
			_, actualProcessID, actualProcessPath, actualSandboxHandle, actualSandboxBundlePath,
				actualPio, actualTTY, actualProcJSON, _ := execRunner.RunArgsForCall(0)
			Expect(actualProcessID).To(Equal(processSpec.ID))
			Expect(actualProcessPath).To(Equal(filepath.Join(ctrBundleDir, "processes", processSpec.ID)))
			Expect(actualSandboxHandle).To(Equal(ctrHandle))
			Expect(actualSandboxBundlePath).To(Equal(ctrBundleDir))
			Expect(actualPio).To(Equal(pio))
			Expect(actualTTY).To(BeFalse())
			Expect(actualProcJSON).To(BeNil())
		})

		Context("when the runtime spec uses a TTY", func() {
			BeforeEach(func() {
				builtProcess.Terminal = true
			})

			It("runs with one", func() {
				Eventually(execRunner.RunCallCount()).Should(Equal(1))
				_, _, _, _, _, _, actualTTY, _, _ := execRunner.RunArgsForCall(0)
				Expect(actualTTY).To(BeTrue())
			})
		})

		It("cleans up the pea", func() {
			Eventually(execRunner.RunCallCount()).Should(Equal(1))
			_, _, _, _, _, _, _, _, cleanup := execRunner.RunArgsForCall(0)
			Expect(cleanup()).To(Succeed())
			Expect(peaCleaner.CleanCallCount()).To(Equal(1))
			_, processID := peaCleaner.CleanArgsForCall(0)
			Expect(processID).To(Equal(processSpec.ID))
		})

		Context("when the process spec has no ID", func() {
			BeforeEach(func() {
				processSpec.ID = ""
			})

			It("generates process ID", func() {
				processDirs, err := ioutil.ReadDir(filepath.Join(ctrBundleDir, "processes"))
				Expect(err).NotTo(HaveOccurred())
				Expect(processDirs).To(HaveLen(1))
			})
		})
		builtProcess = &specs.Process{Cwd: "some-cwd"}

		Context("when no working dir is specified", func() {
			BeforeEach(func() {
				processSpec.Dir = ""
			})

			It("defaults to /", func() {
				Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
				_, actualProcessSpec, _, _ := processBuilder.BuildProcessArgsForCall(0)
				Expect(actualProcessSpec.Dir).To(Equal("/"))
			})
		})

		Context("when no user is specified", func() {
			BeforeEach(func() {
				processSpec.User = ""
			})

			It("defaults to 0:0", func() {
				Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
				_, _, actualContainerUID, actualContainerGID := processBuilder.BuildProcessArgsForCall(0)
				Expect(actualContainerUID).To(Equal(0))
				Expect(actualContainerGID).To(Equal(0))
			})
		})

		Context("when limits are provided", func() {
			BeforeEach(func() {
				processSpec.OverrideContainerLimits = &garden.ProcessLimits{
					CPU:    garden.CPULimits{LimitInShares: 1},
					Memory: garden.MemoryLimits{LimitInBytes: 2},
				}
			})

			It("provides an explicit cgroup path to bundle generation", func() {
				Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
				actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
				Expect(actualCtrSpec.CgroupPath).To(Equal(processSpec.ID))
			})

			It("sets the memory and CPU limits, and no other limits", func() {
				Expect(bundleGenerator.GenerateCallCount()).To(Equal(1))
				actualCtrSpec, _ := bundleGenerator.GenerateArgsForCall(0)
				Expect(actualCtrSpec.Limits).To(Equal(garden.Limits{
					CPU:    processSpec.OverrideContainerLimits.CPU,
					Memory: processSpec.OverrideContainerLimits.Memory,
				}))
			})
		})
	})

	Describe("pea creation failing", func() {
		var (
			createErr error
		)

		JustBeforeEach(func() {
			_, createErr = peaCreator.CreatePea(log, processSpec, garden.ProcessIO{}, ctrHandle, ctrBundleDir)
		})

		Context("when the bind mount source creator return an error", func() {
			BeforeEach(func() {
				bindMountSourceCreator.CreateReturns(nil, errors.New("explode"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("explode")))
			})
		})

		Context("when the pid getter returns an error", func() {
			BeforeEach(func() {
				pidGetter.GetPidReturns(-1, errors.New("pickle"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("pickle")))
			})
		})

		Context("when the bundle generator returns an error", func() {
			BeforeEach(func() {
				bundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("banana"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("banana")))
			})

			It("invokes cleanup on the volumizer", func() {
				Expect(volumizer.DestroyCallCount()).To(Equal(1))
			})

			Context("and volumizer.Destroy returns an error", func() {
				BeforeEach(func() {
					volumizer.DestroyReturns(errors.New("Pikachu!"))
				})

				It("contains both error strings in the returned error", func() {
					Expect(createErr.Error()).To(ContainSubstring("Pikachu!"))
					Expect(createErr.Error()).To(ContainSubstring("banana"))
				})
			})
		})

		Context("when the bundle saver returns an error", func() {
			BeforeEach(func() {
				bundleSaver.SaveReturns(errors.New("papaya"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("papaya")))
			})

			It("invokes cleanup on the volumizer", func() {
				Expect(volumizer.DestroyCallCount()).To(Equal(1))
			})

			Context("and volumizer.Destroy returns an error", func() {
				BeforeEach(func() {
					volumizer.DestroyReturns(errors.New("Pikachu!"))
				})

				It("contains both error strings in the returned error", func() {
					Expect(createErr.Error()).To(ContainSubstring("Pikachu!"))
					Expect(createErr.Error()).To(ContainSubstring("papaya"))
				})
			})
		})

		Context("when the volume creator returns an error", func() {
			BeforeEach(func() {
				volumizer.CreateReturns(specs.Spec{}, errors.New("coconut"))
			})

			It("returns a wrapped error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("coconut")))
			})
		})

		Context("when the user is specified as a username, not a uid:gid", func() {
			BeforeEach(func() {
				processSpec.User = "frank"
			})

			It("returns an error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("frank")))
			})
		})

		Context("when the exec runner returns an error", func() {
			BeforeEach(func() {
				execRunner.RunReturns(nil, errors.New("execrunner-error"))
			})

			It("returns an error", func() {
				Expect(createErr.Error()).To(ContainSubstring("execrunner-error"))
			})

			It("does not leak the pea process dir", func() {
				Expect(filepath.Join(ctrBundleDir, "processes", "some-id")).ToNot(BeADirectory())
			})

			It("invokes cleanup on the volumizer", func() {
				Expect(volumizer.DestroyCallCount()).To(Equal(1))
			})

			Context("and volumizer.Destroy returns an error", func() {
				BeforeEach(func() {
					volumizer.DestroyReturns(errors.New("Pikachu!"))
				})

				It("contains both error strings in the returned error", func() {
					Expect(createErr.Error()).To(ContainSubstring("Pikachu!"))
					Expect(createErr.Error()).To(ContainSubstring("execrunner-error"))
				})
			})
		})

		Context("when the privileged getter returns an error", func() {
			BeforeEach(func() {
				privilegedGetter.PrivilegedReturns(false, errors.New("privileged-getter-error"))
			})

			It("returns an error", func() {
				Expect(createErr).To(MatchError(ContainSubstring("privileged-getter-error")))
			})
		})
	})
})
