package runrunc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Execer", func() {
	var (
		bundleLoader       *fakes.FakeBundleLoader
		processBuilder     *fakes.FakeProcessBuilder
		processIDGenerator *fakes.FakeUidGenerator
		execRunner         *fakes.FakeExecRunner

		execer *runrunc.Execer

		execErr    error
		logger     *lagertest.TestLogger
		bundlePath string
		id         = "some-id"
		spec       garden.ProcessSpec
		pio        = garden.ProcessIO{Stdin: bytes.NewBufferString("some-stdin")}

		user = users.ExecUser{
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
		processIDGenerator = new(fakes.FakeUidGenerator)
		execRunner = new(fakes.FakeExecRunner)

		execer = runrunc.NewExecer(
			bundleLoader,
			processBuilder,
			execRunner,
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	testExec := func() {
		It("succeeds", func() {
			Expect(execErr).NotTo(HaveOccurred())
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
			_, execErr = execer.Exec(logger, id, spec, user, pio)
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
			_, execErr = execer.ExecWithBndl(logger, id, bndl, spec, user, pio)
		})

		testExec()
	})
})
