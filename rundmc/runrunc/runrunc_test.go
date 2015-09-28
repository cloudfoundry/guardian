package runrunc_test

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/specs"
)

var _ = Describe("RuncRunner", func() {
	var (
		tracker       *fakes.FakeProcessTracker
		commandRunner *fake_command_runner.FakeCommandRunner
		pidGenerator  *fakes.FakePidGenerator
		runcBinary    *fakes.FakeRuncBinary

		runner *runrunc.RunRunc
	)

	BeforeEach(func() {
		tracker = new(fakes.FakeProcessTracker)
		pidGenerator = new(fakes.FakePidGenerator)
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()

		runner = runrunc.New(tracker, commandRunner, pidGenerator, runcBinary)

		runcBinary.StartCommandStub = func(path, id string) *exec.Cmd {
			return exec.Command("funC", path, id)
		}

		runcBinary.ExecCommandStub = func(id, processJSONPath string) *exec.Cmd {
			return exec.Command("funC", id, processJSONPath)
		}
	})

	Describe("Start", func() {
		It("runs the injected runC binary using process tracker", func() {
			runner.Start("some/oci/container", "handle", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, _, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args).To(Equal([]string{"funC", "some/oci/container", "handle"}))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
		})

		It("configures the tracker with the next process id", func() {
			pidGenerator.GenerateReturns(42)
			runner.Start("some/oci/container", "some-handle", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			id, _, _, _, _ := tracker.RunArgsForCall(0)
			Expect(id).To(BeEquivalentTo(42))
		})
	})

	Describe("Exec", func() {
		It("runs the tracker with the next process id", func() {
			pidGenerator.GenerateReturns(55)
			runner.Exec("some/oci/container", garden.ProcessSpec{}, garden.ProcessIO{})
			Expect(tracker.RunCallCount()).To(Equal(1))

			pid, _, _, _, _ := tracker.RunArgsForCall(0)
			Expect(pid).To(BeEquivalentTo(55))
		})

		It("runs exec against the injected runC binary using process tracker", func() {
			ttyspec := &garden.TTYSpec{WindowSize: &garden.WindowSize{Rows: 1}}
			runner.Exec("some-id", garden.ProcessSpec{TTY: ttyspec}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, tty, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args[:2]).To(Equal([]string{"funC", "some-id"}))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
			Expect(tty).To(Equal(ttyspec))
		})

		Describe("the process.json passed to 'runc exec'", func() {
			var spec specs.Process

			BeforeEach(func() {
				tracker.RunStub = func(_ uint32, cmd *exec.Cmd, _ garden.ProcessIO, _ *garden.TTYSpec, _ process_tracker.Signaller) (garden.Process, error) {
					f, err := os.Open(cmd.Args[2])
					Expect(err).NotTo(HaveOccurred())

					json.NewDecoder(f).Decode(&spec)
					return nil, nil
				}
			})

			It("passes a process.json with the correct path and args", func() {
				runner.Exec("some/oci/container", garden.ProcessSpec{Path: "to enlightenment", Args: []string{"infinity", "and beyond"}}, garden.ProcessIO{})
				Expect(tracker.RunCallCount()).To(Equal(1))
				Expect(spec.Args).To(Equal([]string{"to enlightenment", "infinity", "and beyond"}))
			})

			Context("when the environment already contains a PATH", func() {
				It("passes the environment variables", func() {
					runner.Exec("some/oci/container", garden.ProcessSpec{
						Env: []string{"a=1", "b=3", "c=4", "PATH=a"},
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a"}))
				})
			})

			Context("when the environment does not already contain a PATH", func() {
				It("appends a default PATH to the environment variables", func() {
					runner.Exec("some/oci/container", garden.ProcessSpec{
						Env: []string{"a=1", "b=3", "c=4"},
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4",
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}))
				})
			})
		})
	})

	Describe("Kill", func() {
		It("runs 'runc kill' in the container directory", func() {
			Expect(runner.Kill("some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Dir:  "some-container",
				Path: "runc",
				Args: []string{"kill"},
			}))
		})
	})
})
