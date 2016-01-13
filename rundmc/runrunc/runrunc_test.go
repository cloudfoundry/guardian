package runrunc_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("RuncRunner", func() {
	var (
		tracker       *fakes.FakeProcessTracker
		commandRunner *fake_command_runner.FakeCommandRunner
		pidGenerator  *fakes.FakeUidGenerator
		runcBinary    *fakes.FakeRuncBinary
		bundleLoader  *fakes.FakeBundleLoader
		idGetter      *fakes.FakeUserLookupper
		logger        lager.Logger

		runner *runrunc.RunRunc
	)

	BeforeEach(func() {
		tracker = new(fakes.FakeProcessTracker)
		pidGenerator = new(fakes.FakeUidGenerator)
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		bundleLoader = new(fakes.FakeBundleLoader)
		idGetter = new(fakes.FakeUserLookupper)
		logger = lagertest.NewTestLogger("test")

		runner = runrunc.New(tracker, commandRunner, pidGenerator, runcBinary, bundleLoader, idGetter)

		bundleLoader.LoadStub = func(path string) (*goci.Bndl, error) {
			bndl := &goci.Bndl{}
			bndl.Spec.Spec.Root.Path = "/rootfs/of/bundle/" + path
			return bndl, nil
		}

		runcBinary.StartCommandStub = func(path, id string) *exec.Cmd {
			return exec.Command("funC", "start", path, id)
		}

		runcBinary.ExecCommandStub = func(id, processJSONPath string) *exec.Cmd {
			return exec.Command("funC", "exec", id, processJSONPath)
		}

		runcBinary.KillCommandStub = func(id, signal string) *exec.Cmd {
			return exec.Command("funC", "kill", id, signal)
		}
	})

	Describe("Start", func() {
		It("runs the injected runC binary using process tracker", func() {
			runner.Start(logger, "some/oci/container", "handle", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args).To(Equal([]string{"funC", "start", "some/oci/container", "handle"}))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
		})

		It("configures the tracker with the a generated process guid", func() {
			pidGenerator.GenerateReturns("some-process-guid")
			runner.Start(logger, "some/oci/container", "some-handle", garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			id, _, _, _ := tracker.RunArgsForCall(0)
			Expect(id).To(BeEquivalentTo("some-process-guid"))
		})
	})

	Describe("Exec", func() {
		It("runs the tracker with the a generated process guid", func() {
			pidGenerator.GenerateReturns("another-process-guid")
			runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{}, garden.ProcessIO{})
			Expect(tracker.RunCallCount()).To(Equal(1))

			pid, _, _, _ := tracker.RunArgsForCall(0)
			Expect(pid).To(BeEquivalentTo("another-process-guid"))
		})

		It("runs exec against the injected runC binary using process tracker", func() {
			ttyspec := &garden.TTYSpec{WindowSize: &garden.WindowSize{Rows: 1}}
			runner.Exec(logger, "/some/bundle/path", "some-id", garden.ProcessSpec{TTY: ttyspec}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, io, tty := tracker.RunArgsForCall(0)
			Expect(cmd.Args[:3]).To(Equal([]string{"funC", "exec", "some-id"}))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
			Expect(tty).To(Equal(ttyspec))
		})

		Describe("the process.json passed to 'runc exec'", func() {
			var spec specs.Process

			BeforeEach(func() {
				tracker.RunStub = func(_ string, cmd *exec.Cmd, _ garden.ProcessIO, _ *garden.TTYSpec) (garden.Process, error) {
					f, err := os.Open(cmd.Args[3])
					Expect(err).NotTo(HaveOccurred())

					json.NewDecoder(f).Decode(&spec)
					return nil, nil
				}
			})

			It("passes a process.json with the correct path and args", func() {
				runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{Path: "to enlightenment", Args: []string{"infinity", "and beyond"}}, garden.ProcessIO{})
				Expect(tracker.RunCallCount()).To(Equal(1))
				Expect(spec.Args).To(Equal([]string{"to enlightenment", "infinity", "and beyond"}))
			})

			Describe("passing the correct uid and gid", func() {
				Context("when the bundle can be loaded", func() {
					BeforeEach(func() {
						idGetter.LookupReturns(9, 7, nil)
						_, err := runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).ToNot(HaveOccurred())
					})

					It("looks up the user and group IDs of the user in the right rootfs", func() {
						Expect(idGetter.LookupCallCount()).To(Equal(1))
						actualRootfsPath, actualUserName := idGetter.LookupArgsForCall(0)
						Expect(actualRootfsPath).To(Equal("/rootfs/of/bundle/some/oci/container"))
						Expect(actualUserName).To(Equal("spiderman"))
					})

					It("passes a process.json with the correct user and group ids", func() {
						Expect(spec.User).To(Equal(specs.User{UID: 9, GID: 7}))
					})
				})

				Context("when the bundle can't be loaded", func() {
					BeforeEach(func() {
						bundleLoader.LoadReturns(nil, errors.New("whoa! Hold them horses!"))
					})

					It("fails", func() {
						_, err := runner.Exec(logger, "some/oci/container", "someid",
							garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("Hold them horses")))
					})
				})

				Context("when the bundle contains an empty rootfs path", func() {
					BeforeEach(func() {
						bundleLoader.LoadReturns(&goci.Bndl{}, nil)
					})

					It("returns an appropriate error", func() {
						_, err := runner.Exec(logger, "some/oci/container", "someid",
							garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("empty rootfs path")))
						Expect(err).To(MatchError(ContainSubstring("someid")))
					})
				})

				Context("when IdGetter returns an error", func() {
					It("passes a process.json with the correct user and group ids", func() {
						idGetter.LookupReturns(0, 0, errors.New("bang"))

						_, err := runner.Exec(logger, "some/oci/container", "some-id", garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("bang")))
					})
				})
			})

			Context("when the environment already contains a PATH", func() {
				It("passes the environment variables", func() {
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env: []string{"a=1", "b=3", "c=4", "PATH=a"},
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a"}))
				})
			})

			Context("when the environment does not already contain a PATH", func() {
				It("appends a default PATH for the root user", func() {
					idGetter.LookupReturns(0, 0, nil)
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env:  []string{"a=1", "b=3", "c=4"},
						User: "root",
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4",
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}))
				})

				It("appends a default PATH for non-root users", func() {
					idGetter.LookupReturns(1000, 1000, nil)
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env:  []string{"a=1", "b=3", "c=4"},
						User: "alice",
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4",
						"PATH=/usr/local/bin:/usr/bin:/bin"}))
				})
			})
		})
	})

	Describe("Kill", func() {
		It("runs 'runc kill' in the container directory", func() {
			Expect(runner.Kill(logger, "some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"kill", "some-container", "KILL"},
			}))
		})
	})
})
