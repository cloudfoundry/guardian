package runrunc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Execer", func() {
	var (
		bundleLoader       *fakes.FakeBundleLoader
		processBuilder     *fakes.FakeProcessBuilder
		mkdirer            *fakes.FakeMkdirer
		userLookuper       *fakes.FakeUserLookupper
		processIDGenerator *fakes.FakeUidGenerator
		execRunner         *fakes.FakeExecRunner
		pidGetter          *fakes.FakePidGetter

		execer *runrunc.Execer

		logger     *lagertest.TestLogger
		bundlePath string
		id         = "some-id"
		spec       garden.ProcessSpec
		pio        = garden.ProcessIO{Stdin: bytes.NewBufferString("some-stdin")}

		user = &runrunc.ExecUser{
			Uid:  1,
			Gid:  2,
			Home: "/some/home",
		}
		bndl = goci.Bundle().
			WithUIDMappings(specs.LinuxIDMapping{
				ContainerID: 0,
				HostID:      10,
				Size:        100,
			}).
			WithGIDMappings(specs.LinuxIDMapping{
				ContainerID: 0,
				HostID:      20,
				Size:        100,
			})
		preparedProc *specs.Process
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "execer-test")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test")
		spec = garden.ProcessSpec{
			ID:   "some-process-id",
			Path: "some-program",
			User: "some-user",
			Dir:  "/some/working/dir",
			TTY:  &garden.TTYSpec{WindowSize: &garden.WindowSize{Rows: 42}},
		}

		preparedProc = &specs.Process{Cwd: "some-cwd"}

		bundleLoader = new(fakes.FakeBundleLoader)
		bundleLoader.LoadReturns(bndl, nil)
		processBuilder = new(fakes.FakeProcessBuilder)
		processBuilder.BuildProcessReturns(preparedProc)
		mkdirer = new(fakes.FakeMkdirer)
		userLookuper = new(fakes.FakeUserLookupper)
		userLookuper.LookupReturns(user, nil)
		processIDGenerator = new(fakes.FakeUidGenerator)
		execRunner = new(fakes.FakeExecRunner)
		pidGetter = new(fakes.FakePidGetter)
		pidGetter.GetPidReturns(1234, nil)

		execer = runrunc.NewExecer(
			bundleLoader,
			processBuilder,
			mkdirer,
			userLookuper,
			execRunner,
			processIDGenerator,
			pidGetter,
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Describe("successful Exec", func() {
		JustBeforeEach(func() {
			_, err := execer.Exec(logger, bundlePath, id, spec, pio)
			Expect(err).NotTo(HaveOccurred())
		})

		It("looks up the user", func() {
			Expect(userLookuper.LookupCallCount()).To(Equal(1))
			rootfsPath, username := userLookuper.LookupArgsForCall(0)
			Expect(rootfsPath).To(Equal(filepath.Join("/proc", "1234", "root")))
			Expect(username).To(Equal(spec.User))
		})

		It("loads the bundle", func() {
			Expect(bundleLoader.LoadCallCount()).To(Equal(1))
			Expect(bundleLoader.LoadArgsForCall(0)).To(Equal(bundlePath))
		})

		It("sets up the working directory", func() {
			Expect(mkdirer.MkdirAsCallCount()).To(Equal(1))
			rootfsPath, hostUID, hostGID, mode, shouldRecreate, workDir := mkdirer.MkdirAsArgsForCall(0)
			Expect(rootfsPath).To(Equal(filepath.Join("/proc", "1234", "root")))
			Expect(hostUID).To(Equal(11))
			Expect(hostGID).To(Equal(22))
			Expect(mode).To(Equal(os.FileMode(0755)))
			Expect(shouldRecreate).To(BeFalse())
			Expect(workDir).To(ConsistOf(spec.Dir))
		})

		It("builds a process", func() {
			Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
			actualBundle, actualProcessSpec, actualContainerUID, actualContainerGID := processBuilder.BuildProcessArgsForCall(0)
			Expect(actualBundle).To(Equal(bndl))
			Expect(actualProcessSpec).To(Equal(spec))
			Expect(actualContainerUID).To(Equal(user.Uid))
			Expect(actualContainerGID).To(Equal(user.Gid))
		})

		It("does not generate an ID for the process if one is specified", func() {
			Expect(processIDGenerator.GenerateCallCount()).To(Equal(0))
		})

		Context("when no process ID is specified", func() {
			BeforeEach(func() {
				spec.ID = ""
			})

			It("generates one", func() {
				Expect(processIDGenerator.GenerateCallCount()).To(Equal(1))
			})
		})

		It("creates the process directory", func() {
			Expect(filepath.Join(bundlePath, "processes", spec.ID)).To(BeADirectory())
		})

		It("runs the process", func() {
			Expect(execRunner.RunCallCount()).To(Equal(1))
			_, processID, actualProcessPath, actualSandboxHandle, actualSandboxBundlePath,
				actualPIO, actualTTY, actualProcJSON, _ := execRunner.RunArgsForCall(0)
			Expect(processID).To(Equal(spec.ID))
			Expect(actualProcessPath).To(Equal(filepath.Join(bundlePath, "processes", processID)))
			Expect(actualSandboxHandle).To(Equal(id))
			Expect(actualSandboxBundlePath).To(Equal(bundlePath))

			Expect(actualPIO).To(Equal(pio))
			Expect(actualTTY).To(BeFalse())

			actualProcJSONBytes, err := ioutil.ReadAll(actualProcJSON)
			Expect(err).NotTo(HaveOccurred())
			procJSONBytes, err := json.Marshal(preparedProc)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(actualProcJSONBytes)).To(Equal(string(procJSONBytes)))
		})

		Context("when the process has a terminal", func() {
			BeforeEach(func() {
				preparedProc.Terminal = true
			})

			It("runs the process, specifying a TTY", func() {
				Expect(execRunner.RunCallCount()).To(Equal(1))
				_, _, _, _, _, _, actualTTY, _, _ := execRunner.RunArgsForCall(0)
				Expect(actualTTY).To(BeTrue())
			})
		})

		Context("when a working directory is not specified", func() {
			BeforeEach(func() {
				spec.Dir = ""
			})

			It("defaults the workdir to the user's home when setting it up", func() {
				Expect(mkdirer.MkdirAsCallCount()).To(Equal(1))
				_, _, _, _, _, workDir := mkdirer.MkdirAsArgsForCall(0)
				Expect(workDir).To(ConsistOf(user.Home))
			})

			It("defaults the workdir to the user's home when building a process", func() {
				Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
				_, actualProcessSpec, _, _ := processBuilder.BuildProcessArgsForCall(0)
				Expect(actualProcessSpec.Dir).To(Equal(user.Home))
			})
		})
	})

	Describe("Failed Exec", func() {
		var execErr error

		JustBeforeEach(func() {
			_, execErr = execer.Exec(logger, bundlePath, id, spec, pio)
		})

		Context("when user lookup fails", func() {
			BeforeEach(func() {
				userLookuper.LookupReturns(nil, errors.New("user-lookup"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("user-lookup")))
			})
		})

		Context("when loading the bundle fails", func() {
			BeforeEach(func() {
				bundleLoader.LoadReturns(goci.Bndl{}, errors.New("load-bundle"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("load-bundle")))
			})
		})

		Context("when preparing the working directory fails", func() {
			BeforeEach(func() {
				mkdirer.MkdirAsReturns(errors.New("mkdir"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("mkdir")))
			})
		})

		Context("when the process ID is already in use", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(bundlePath, "processes", spec.ID), 0700)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError("process ID 'some-process-id' already in use"))
			})
		})

		Context("when running the process fails", func() {
			BeforeEach(func() {
				execRunner.RunReturns(nil, errors.New("run"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("run")))
			})
		})
	})
})
