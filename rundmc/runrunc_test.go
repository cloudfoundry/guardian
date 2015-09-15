package rundmc_test

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
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

		runner *rundmc.RunRunc
	)

	BeforeEach(func() {
		tracker = new(fakes.FakeProcessTracker)
		pidGenerator = new(fakes.FakePidGenerator)
		commandRunner = fake_command_runner.New()
		runner = &rundmc.RunRunc{
			PidGenerator:  pidGenerator,
			Tracker:       tracker,
			CommandRunner: commandRunner,
		}
	})

	Describe("Run", func() {
		It("runs runC in the directory using process tracker", func() {
			runner.Start("some/oci/container", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, _, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args[0]).To(Equal("runc"))
			Expect(cmd.Dir).To(Equal("some/oci/container"))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
		})

		It("configures the tracker with the next id", func() {
			pidGenerator.GenerateReturns(42)
			runner.Start("some/oci/container", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			id, _, _, _, _ := tracker.RunArgsForCall(0)
			Expect(id).To(BeEquivalentTo(42))
		})
	})

	Describe("Exec", func() {
		It("runs the tracker with the next id", func() {
			pidGenerator.GenerateReturns(55)
			runner.Exec("some/oci/container", garden.ProcessSpec{}, garden.ProcessIO{})
			Expect(tracker.RunCallCount()).To(Equal(1))

			pid, _, _, _, _ := tracker.RunArgsForCall(0)
			Expect(pid).To(BeEquivalentTo(55))
		})

		It("runs 'runC exec' in the directory using process tracker", func() {
			ttyspec := &garden.TTYSpec{WindowSize: &garden.WindowSize{Rows: 1}}
			runner.Exec("some/oci/container", garden.ProcessSpec{TTY: ttyspec}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, tty, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args[:2]).To(Equal([]string{"runc", "exec"}))
			Expect(cmd.Dir).To(Equal("some/oci/container"))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
			Expect(tty).To(Equal(ttyspec))
		})

		Describe("process.json", func() {
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

			It("passes the correct env", func() {
				runner.Exec("some/oci/container", garden.ProcessSpec{
					Env: []string{"a", "b", "c"},
				}, garden.ProcessIO{})

				Expect(tracker.RunCallCount()).To(Equal(1))
				Expect(spec.Env).To(Equal([]string{"a", "b", "c"}))
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
