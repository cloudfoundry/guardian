package execrunner_test

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	fakes "code.cloudfoundry.org/guardian/rundmc/execrunner/execrunnerfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("DirectExecRunner", func() {
	var (
		cmdRunner    *fake_command_runner.FakeCommandRunner
		processIDGen *runruncfakes.FakeUidGenerator
		fileWriter   *fakes.FakeFileWriter
		execRunner   *execrunner.DirectExecRunner
		runtimePath  = "container-runtime-path"

		spec      *runrunc.PreparedSpec
		process   garden.Process
		runErr    error
		processID string
	)

	BeforeEach(func() {
		cmdRunner = fake_command_runner.New()
		processIDGen = new(runruncfakes.FakeUidGenerator)
		fileWriter = new(fakes.FakeFileWriter)
		execRunner = &execrunner.DirectExecRunner{
			RuntimePath:   runtimePath,
			CommandRunner: cmdRunner,
			ProcessIDGen:  processIDGen,
			FileWriter:    fileWriter,
		}
		processID = ""
	})

	JustBeforeEach(func() {
		spec = &runrunc.PreparedSpec{Process: specs.Process{Cwd: "idiosyncratic-string"}}
		process, runErr = execRunner.Run(
			lagertest.NewTestLogger("execrunner-windows"),
			processID, spec,
			"a-bundle", "a-process-directory", "handle",
			nil, garden.ProcessIO{},
		)
	})

	It("does not error", func() {
		Expect(runErr).NotTo(HaveOccurred())
	})

	It("runs the runtime plugin", func() {
		Expect(cmdRunner.StartedCommands()).To(HaveLen(1))
		Expect(cmdRunner.StartedCommands()[0].Path).To(Equal(runtimePath))
		Expect(cmdRunner.StartedCommands()[0].Args).To(ConsistOf(
			runtimePath,
			"--debug",
			"--log", MatchRegexp(".*"),
			"exec",
			"-d",
			"-p", filepath.Join("a-process-directory", process.ID(), "spec.json"),
			"--pid-file", MatchRegexp(".*"),
			"handle",
		))
	})

	It("writes the process spec", func() {
		Expect(fileWriter.WriteFileCallCount()).To(Equal(1))
		actualPath, actualContents, _ := fileWriter.WriteFileArgsForCall(0)
		Expect(actualPath).To(Equal(filepath.Join("a-process-directory", process.ID(), "spec.json")))
		actualSpec := &runrunc.PreparedSpec{}
		Expect(json.Unmarshal(actualContents, actualSpec)).To(Succeed())
		Expect(actualSpec).To(Equal(spec))
	})

	Context("when no process ID is passed", func() {
		BeforeEach(func() {
			processIDGen.GenerateReturns("some-generated-id")
		})

		It("uses a generated process ID", func() {
			Expect(process.ID()).To(Equal("some-generated-id"))
		})
	})

	Context("when a process ID is passed", func() {
		BeforeEach(func() {
			processID = "frank"
		})

		It("uses it", func() {
			Expect(process.ID()).To(Equal("frank"))
		})
	})

	Context("when the runtime plugin can't be started", func() {
		BeforeEach(func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
				return errors.New("oops")
			})
		})

		It("returns an error", func() {
			Expect(runErr).To(MatchError("execing runtime plugin: oops"))
		})
	})

	Context("when writing the process spec fails", func() {
		BeforeEach(func() {
			fileWriter.WriteFileReturns(errors.New("This is because you don't give Morty Smith good grades!"))
		})

		It("returns an error", func() {
			Expect(runErr).To(MatchError("writing process spec: This is because you don't give Morty Smith good grades!"))
		})
	})
})
