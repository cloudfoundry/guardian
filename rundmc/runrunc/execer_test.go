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
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
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
		userLookupper      *usersfakes.FakeUserLookupper
		processIDGenerator *fakes.FakeUidGenerator
		execRunner         *fakes.FakeExecRunner
		pidGetter          *fakes.FakePidGetter

		execer *runrunc.Execer

		execErr    error
		logger     *lagertest.TestLogger
		bundlePath string
		id         = "some-id"
		spec       garden.ProcessSpec
		pio        = garden.ProcessIO{Stdin: bytes.NewBufferString("some-stdin")}

		user = &users.ExecUser{
			Uid:   1,
			Gid:   2,
			Sgids: []int{5, 6, 7},
			Home:  "/some/home",
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
		userLookupper = new(usersfakes.FakeUserLookupper)
		userLookupper.LookupReturns(user, nil)
		processIDGenerator = new(fakes.FakeUidGenerator)
		execRunner = new(fakes.FakeExecRunner)
		pidGetter = new(fakes.FakePidGetter)
		pidGetter.GetPidReturns(1234, nil)

		execer = runrunc.NewExecer(
			bundleLoader,
			processBuilder,
			mkdirer,
			userLookupper,
			execRunner,
			pidGetter,
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	testExec := func() {
		It("succeeds", func() {
			Expect(execErr).NotTo(HaveOccurred())
		})

		It("looks up the user", func() {
			Expect(userLookupper.LookupCallCount()).To(Equal(1))
			rootfsPath, username := userLookupper.LookupArgsForCall(0)
			Expect(rootfsPath).To(Equal(filepath.Join("/proc", "1234", "root")))
			Expect(username).To(Equal(spec.User))
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
			actualBundle, actualProcessSpec, actualUser := processBuilder.BuildProcessArgsForCall(0)
			Expect(actualBundle).To(Equal(bndl))
			Expect(actualProcessSpec).To(Equal(spec))
			Expect(actualUser.Uid).To(Equal(user.Uid))
			Expect(actualUser.Gid).To(Equal(user.Gid))
			Expect(actualUser.Sgids).To(Equal(user.Sgids))
		})

		It("does not generate an ID for the process if one is specified", func() {
			Expect(processIDGenerator.GenerateCallCount()).To(Equal(0))
		})

		Context("when no process ID is specified", func() {
			BeforeEach(func() {
				spec.ID = ""
			})

			It("generates one", func() {
				Expect(execRunner.RunCallCount()).To(Equal(1))
				_, processID, _, _, _, _, _ := execRunner.RunArgsForCall(0)
				Expect(processID).NotTo(Equal(""))
			})
		})

		It("runs the process", func() {
			Expect(execRunner.RunCallCount()).To(Equal(1))
			_, processID, actualSandboxHandle, actualPIO, actualTTY, actualProcJSON, _ := execRunner.RunArgsForCall(0)
			Expect(processID).To(Equal(spec.ID))
			Expect(actualSandboxHandle).To(Equal(id))

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
				_, _, _, _, actualTTY, _, _ := execRunner.RunArgsForCall(0)
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
				_, actualProcessSpec, _ := processBuilder.BuildProcessArgsForCall(0)
				Expect(actualProcessSpec.Dir).To(Equal(user.Home))
			})
		})

		Context("when user lookup fails", func() {
			BeforeEach(func() {
				userLookupper.LookupReturns(nil, errors.New("user-lookup"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("user-lookup")))
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

		Context("when running the process fails", func() {
			BeforeEach(func() {
				execRunner.RunReturns(nil, errors.New("run"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("run")))
			})
		})
	}

	Describe("Exec", func() {
		JustBeforeEach(func() {
			_, execErr = execer.Exec(logger, id, spec, pio)
		})

		It("loads the bundle", func() {
			Expect(bundleLoader.LoadCallCount()).To(Equal(1))
			_, actualId := bundleLoader.LoadArgsForCall(0)
			Expect(actualId).To(Equal(id))
		})

		Context("when loading the bundle fails", func() {
			BeforeEach(func() {
				bundleLoader.LoadReturns(goci.Bndl{}, errors.New("load-bundle"))
			})

			It("returns an error", func() {
				Expect(execErr).To(MatchError(ContainSubstring("load-bundle")))
			})
		})

		testExec()
	})

	Describe("ExecWithBndl", func() {
		JustBeforeEach(func() {
			_, execErr = execer.ExecWithBndl(logger, id, bndl, spec, pio)
		})

		testExec()
	})
})
