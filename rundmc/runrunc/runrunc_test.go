package runrunc_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/specs"
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
		users         *fakes.FakeUserLookupper
		mkdirer       *fakes.FakeMkdirer
		bundlePath    string
		logger        *lagertest.TestLogger

		runner *runrunc.RunRunc
	)

	var rootfsPath = func(bundlePath string) string {
		return "/rootfs/of/bundle" + bundlePath
	}

	BeforeEach(func() {
		tracker = new(fakes.FakeProcessTracker)
		pidGenerator = new(fakes.FakeUidGenerator)
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		bundleLoader = new(fakes.FakeBundleLoader)
		users = new(fakes.FakeUserLookupper)
		mkdirer = new(fakes.FakeMkdirer)
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())

		runner = runrunc.New(
			tracker,
			commandRunner,
			pidGenerator,
			runcBinary,
			runrunc.NewExecPreparer(
				bundleLoader,
				users,
				mkdirer,
			),
		)

		bundleLoader.LoadStub = func(path string) (*goci.Bndl, error) {
			bndl := &goci.Bndl{}
			bndl.Spec.Spec.Root.Path = rootfsPath(path)
			return bndl, nil
		}

		users.LookupReturns(&user.ExecUser{}, nil)

		runcBinary.StartCommandStub = func(path, id string, detach bool) *exec.Cmd {
			return exec.Command("funC", "start", path, id, fmt.Sprintf("%t", detach))
		}

		runcBinary.ExecCommandStub = func(id, processJSONPath, pidFilePath string) *exec.Cmd {
			return exec.Command("funC", "exec", id, processJSONPath, "--pid-file", pidFilePath)
		}

		runcBinary.KillCommandStub = func(id, signal string) *exec.Cmd {
			return exec.Command("funC", "kill", id, signal)
		}

		runcBinary.DeleteCommandStub = func(id string) *exec.Cmd {
			return exec.Command("funC", "delete", id)
		}
	})

	Describe("Start", func() {
		It("starts the container with runC passing the detach flag", func() {
			tracker.RunReturns(new(fakes.FakeProcess), nil)
			Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

			Expect(tracker.RunCallCount()).To(Equal(1))
			_, cmd, _, _, _ := tracker.RunArgsForCall(0)

			Expect(cmd.Path).To(Equal("funC"))
			Expect(cmd.Args).To(Equal([]string{"funC", "start", bundlePath, "some-id", "true"}))
		})

		Describe("forwarding logs from runC", func() {
			var (
				errorFromStart error
				logs           string
			)

			BeforeEach(func() {
				errorFromStart = nil
				logs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`
			})

			JustBeforeEach(func() {
				tracker.RunStub = func(_ string, _ *exec.Cmd, io garden.ProcessIO, _ *garden.TTYSpec, _ string) (garden.Process, error) {
					io.Stdout.Write([]byte(logs))
					fakeProcess := new(fakes.FakeProcess)

					if errorFromStart != nil {
						fakeProcess.WaitReturns(12, errorFromStart)
					}

					return fakeProcess, nil
				}
			})

			It("sends all the logs to the logger", func() {
				Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

				runcLogs := make([]lager.LogFormat, 0)
				for _, log := range logger.Logs() {
					if log.Message == "test.start.runc" {
						runcLogs = append(runcLogs, log)
					}
				}

				Expect(runcLogs).To(HaveLen(3))
				Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
			})

			Context("when runC start fails", func() {
				BeforeEach(func() {
					errorFromStart = errors.New("exit status potato")
				})

				It("return an error including parsed logs when runC fails to starts the container", func() {
					Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc start: exit status potato: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
				})

				Context("when the log messages can't be parsed", func() {
					BeforeEach(func() {
						logs = `foo="'
					`
					})

					It("returns an error with only the exit status if the log can't be parsed", func() {
						Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc start: exit status potato"))
					})
				})
			})
		})
	})

	Describe("Exec", func() {
		It("runs exec against the injected runC binary using process tracker", func() {
			pidGenerator.GenerateReturns("another-process-guid")
			ttyspec := &garden.TTYSpec{WindowSize: &garden.WindowSize{Rows: 1}}
			runner.Exec(logger, bundlePath, "some-id", garden.ProcessSpec{TTY: ttyspec}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			pid, cmd, io, tty, _ := tracker.RunArgsForCall(0)
			Expect(pid).To(Equal("another-process-guid"))
			Expect(cmd.Args[:3]).To(Equal([]string{"funC", "exec", "some-id"}))
			Expect(io.Stdout).To(Equal(GinkgoWriter))
			Expect(tty).To(Equal(ttyspec))
		})

		It("creates the processes directory if it does not exist", func() {
			runner.Exec(logger, bundlePath, "some-id", garden.ProcessSpec{}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(path.Join(bundlePath, "processes")).To(BeADirectory())
		})

		Context("When creating the processes directory fails", func() {
			It("returns a helpful error", func() {
				Expect(ioutil.WriteFile(path.Join(bundlePath, "processes"), []byte(""), 0700)).To(Succeed())
				_, err := runner.Exec(logger, bundlePath, "some-id", garden.ProcessSpec{}, garden.ProcessIO{Stdout: GinkgoWriter})
				Expect(err).To(MatchError(MatchRegexp("mkdir .*: .*")))
			})
		})

		It("asks for the pid file to be placed in processes/$guid.pid", func() {
			pidGenerator.GenerateReturns("another-process-guid")
			runner.Exec(logger, bundlePath, "some-id", garden.ProcessSpec{}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, cmd, _, _, _ := tracker.RunArgsForCall(0)
			Expect(cmd.Args[4:]).To(Equal([]string{"--pid-file", path.Join(bundlePath, "/processes/another-process-guid.pid")}))
		})

		It("tells process tracker that it can find the pid-file at processes/$guid.pid", func() {
			pidGenerator.GenerateReturns("another-process-guid")
			runner.Exec(logger, bundlePath, "some-id", garden.ProcessSpec{}, garden.ProcessIO{Stdout: GinkgoWriter})
			Expect(tracker.RunCallCount()).To(Equal(1))

			_, _, _, _, pidFile := tracker.RunArgsForCall(0)
			Expect(pidFile).To(Equal(path.Join(bundlePath, "/processes/another-process-guid.pid")))
		})

		Describe("the process.json passed to 'runc exec'", func() {
			var spec specs.Process

			BeforeEach(func() {
				tracker.RunStub = func(_ string, cmd *exec.Cmd, _ garden.ProcessIO, _ *garden.TTYSpec, _ string) (garden.Process, error) {
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
						users.LookupReturns(&user.ExecUser{Uid: 9, Gid: 7}, nil)
						_, err := runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).ToNot(HaveOccurred())
					})

					It("looks up the user and group IDs of the user in the right rootfs", func() {
						Expect(users.LookupCallCount()).To(Equal(1))
						actualRootfsPath, actualUserName := users.LookupArgsForCall(0)
						Expect(actualRootfsPath).To(Equal(rootfsPath("some/oci/container")))
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

				Context("when User Lookup returns an error", func() {
					It("passes a process.json with the correct user and group ids", func() {
						users.LookupReturns(&user.ExecUser{Uid: 0, Gid: 0}, errors.New("bang"))

						_, err := runner.Exec(logger, "some/oci/container", "some-id", garden.ProcessSpec{User: "spiderman"}, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("bang")))
					})
				})
			})

			Context("when the user is specified in the process spec", func() {
				Context("when the environment does not contain a USER", func() {
					It("appends a default user", func() {
						runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
							User: "spiderman",
							Env:  []string{"a=1", "b=3", "c=4", "PATH=a", "HOME=/spidermanhome"},
						}, garden.ProcessIO{})

						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Env).To(ConsistOf("a=1", "b=3", "c=4", "PATH=a", "USER=spiderman", "HOME=/spidermanhome"))
					})
				})

				Context("when the environment does contain a USER", func() {
					It("appends a default user", func() {
						runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
							User: "spiderman",
							Env:  []string{"a=1", "b=3", "c=4", "PATH=a", "USER=superman"},
						}, garden.ProcessIO{})

						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a", "USER=superman"}))
					})
				})
			})

			Context("when the user is not specified in the process spec", func() {
				Context("when the environment does not contain a USER", func() {
					It("passes the environment variables", func() {
						runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
							Env: []string{"a=1", "b=3", "c=4", "PATH=a"},
						}, garden.ProcessIO{})

						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a", "USER=root"}))
					})
				})

				Context("when the environment already contains a USER", func() {
					It("passes the environment variables", func() {
						runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
							Env: []string{"a=1", "b=3", "c=4", "PATH=a", "USER=yo"},
						}, garden.ProcessIO{})

						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a", "USER=yo"}))
					})
				})
			})

			Context("when the environment already contains a PATH", func() {
				It("passes the environment variables", func() {
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env: []string{"a=1", "b=3", "c=4", "PATH=a"},
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a", "USER=root"}))
				})
			})

			Context("when the environment does not already contain a PATH", func() {
				It("appends a default PATH for the root user", func() {
					users.LookupReturns(&user.ExecUser{Uid: 0, Gid: 0}, nil)
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env:  []string{"a=1", "b=3", "c=4"},
						User: "root",
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4",
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"}))
				})

				It("appends a default PATH for non-root users", func() {
					users.LookupReturns(&user.ExecUser{Uid: 1000, Gid: 1000}, nil)
					runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env:  []string{"a=1", "b=3", "c=4"},
						User: "alice",
					}, garden.ProcessIO{})

					Expect(tracker.RunCallCount()).To(Equal(1))
					Expect(spec.Env).To(Equal([]string{"a=1", "b=3", "c=4",
						"PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"}))
				})
			})

			Context("when the container has environment variables", func() {
				var (
					processEnv   []string
					containerEnv []string
					bndl         *goci.Bndl
				)

				BeforeEach(func() {
					containerEnv = []string{"ENV_CONTAINER_NAME=garden"}
					processEnv = []string{"ENV_PROCESS_ID=1"}
				})

				JustBeforeEach(func() {
					bndl = &goci.Bndl{}
					bndl.Spec.Spec.Root.Path = "/some/rootfs/path"
					bndl.Spec.Spec.Process.Env = containerEnv
					bundleLoader.LoadReturns(bndl, nil)

					_, err := runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env: processEnv,
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("appends the process vars into container vars", func() {
					envWContainer := make([]string, len(spec.Env))
					copy(envWContainer, spec.Env)

					bndl.Spec.Spec.Process.Env = []string{}
					bundleLoader.LoadReturns(bndl, nil)

					_, err := runner.Exec(logger, "some/oci/container", "someid", garden.ProcessSpec{
						Env: processEnv,
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					Expect(envWContainer).To(Equal(append(containerEnv, spec.Env...)))
				})

				Context("and the container environment contains PATH", func() {
					BeforeEach(func() {
						containerEnv = append(containerEnv, "PATH=/test")
					})

					It("should not apply the default PATH", func() {
						Expect(spec.Env).To(Equal([]string{
							"ENV_CONTAINER_NAME=garden",
							"PATH=/test",
							"ENV_PROCESS_ID=1",
							"USER=root",
						}))
					})
				})
			})

			Describe("working directory", func() {
				Context("when the working directory is specified", func() {
					It("passes the correct cwd to the spec", func() {
						runner.Exec(
							logger, bundlePath, "someid",
							garden.ProcessSpec{Dir: "/home/dir"}, garden.ProcessIO{},
						)
						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Cwd).To(Equal("/home/dir"))
					})

					Describe("Creating the working directory", func() {
						JustBeforeEach(func() {
							users.LookupReturns(&user.ExecUser{Uid: 1012, Gid: 1013}, nil)

							_, err := runner.Exec(logger, bundlePath, "someid", garden.ProcessSpec{
								Dir: "/path/to/banana/dir",
							}, garden.ProcessIO{})
							Expect(err).NotTo(HaveOccurred())
						})

						Context("when the container is privileged", func() {
							It("creates the working directory", func() {
								Expect(mkdirer.MkdirAsCallCount()).To(Equal(1))
								path, mode, uid, gid := mkdirer.MkdirAsArgsForCall(0)
								Expect(path).To(Equal(rootfsPath(filepath.Join(bundlePath, "/path/to/banana/dir"))))
								Expect(mode).To(BeNumerically("==", 0755))
								Expect(uid).To(BeEquivalentTo(1012))
								Expect(gid).To(BeEquivalentTo(1013))
							})
						})

						Context("when the container is unprivileged", func() {
							BeforeEach(func() {
								bundleLoader.LoadStub = func(path string) (*goci.Bndl, error) {
									bndl := &goci.Bndl{}
									bndl.Spec.Spec.Root.Path = "/rootfs/of/bundle/" + path
									bndl.Spec.Linux.UIDMappings = []specs.IDMapping{{
										HostID:      1712,
										ContainerID: 1012,
										Size:        1,
									}}
									bndl.Spec.Linux.GIDMappings = []specs.IDMapping{{
										HostID:      1713,
										ContainerID: 1013,
										Size:        1,
									}}
									return bndl, nil
								}
							})

							It("creates the working directory as the mapped user", func() {
								Expect(mkdirer.MkdirAsCallCount()).To(Equal(1))
								path, mode, uid, gid := mkdirer.MkdirAsArgsForCall(0)
								Expect(path).To(Equal(rootfsPath(filepath.Join(bundlePath, "/path/to/banana/dir"))))
								Expect(mode).To(BeNumerically("==", 0755))
								Expect(uid).To(BeEquivalentTo(1712))
								Expect(gid).To(BeEquivalentTo(1713))
							})
						})
					})
				})

				Context("when the working directory is not specified", func() {
					It("defaults to the user's HOME directory", func() {
						users.LookupReturns(&user.ExecUser{Home: "/the/home/dir"}, nil)

						runner.Exec(
							logger, bundlePath, "someid",
							garden.ProcessSpec{Dir: ""}, garden.ProcessIO{},
						)

						Expect(tracker.RunCallCount()).To(Equal(1))
						Expect(spec.Cwd).To(Equal("/the/home/dir"))
					})

					It("creates the directory", func() {
						users.LookupReturns(&user.ExecUser{Uid: 1012, Gid: 1013, Home: "/some/dir"}, nil)

						_, err := runner.Exec(logger, bundlePath, "someid", garden.ProcessSpec{}, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						Expect(mkdirer.MkdirAsCallCount()).To(Equal(1))
						path, _, _, _ := mkdirer.MkdirAsArgsForCall(0)
						Expect(path).To(Equal(rootfsPath(filepath.Join(bundlePath, "/some/dir"))))
					})
				})

				Context("when the working directory creation fails", func() {
					It("returns an error", func() {
						mkdirer.MkdirAsReturns(errors.New("BOOOOOM"))
						_, err := runner.Exec(logger, bundlePath, "someid", garden.ProcessSpec{}, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("create working directory: BOOOOOM")))
					})
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

		It("returns any stderr output when 'runc kill' fails", func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{}, func(cmd *exec.Cmd) error {
				cmd.Stderr.Write([]byte("some error"))
				return errors.New("exit status banana")
			})

			Expect(runner.Kill(logger, "some-container")).To(MatchError("runc kill: exit status banana: some error"))
		})
	})

	Describe("Delete", func() {
		It("deletes the bundle with 'runc delete'", func() {
			Expect(runner.Delete(logger, "some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"delete", "some-container"},
			}))
		})
	})

	Describe("Watching for Events", func() {
		var (
			eventsCh chan bool
		)

		BeforeEach(func() {
			runcBinary.EventsCommandStub = func(handle string) *exec.Cmd {
				return exec.Command("funC-events", "events", handle)
			}
		})

		It("blows up if `runc events` returns an error", func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-events",
			}, func(cmd *exec.Cmd) error {
				return errors.New("boom")
			})

			Expect(runner.Watch(logger, "some-container", nil)).To(MatchError("start: boom"))
		})

		Context("when runc events succeeds", func() {
			BeforeEach(func() {
				eventsCh = make(chan bool, 2)
				stdoutCh := make(chan io.WriteCloser)

				go func() {
					stdoutW := <-stdoutCh
					for eventIsOOM := range eventsCh {
						t := "something-else"
						if eventIsOOM {
							t = "oom"
						}

						stdoutW.Write([]byte(fmt.Sprintf(`{
						"type": "%s"
					}`, t)))
					}

					stdoutW.Close()
				}()

				commandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "funC-events",
				}, func(cmd *exec.Cmd) error {
					stdoutCh <- cmd.Stdout.(io.WriteCloser)
					return nil
				})
			})

			AfterEach(func() {
				close(eventsCh)
			})

			It("reports an event if one happens", func() {
				notifier := new(fakes.FakeNotifier)
				go runner.Watch(logger, "some-container", notifier)

				Consistently(notifier.OnEventCallCount).Should(Equal(0))

				eventsCh <- true
				Eventually(notifier.OnEventCallCount).Should(Equal(1))
				handle, event := notifier.OnEventArgsForCall(0)
				Expect(handle).To(Equal("some-container"))
				Expect(event).To(Equal("Out of memory"))

				eventsCh <- true
				Eventually(notifier.OnEventCallCount).Should(Equal(2))
				handle, event = notifier.OnEventArgsForCall(1)
				Expect(handle).To(Equal("some-container"))
				Expect(event).To(Equal("Out of memory"))
			})

			It("does not report non-OOM events", func() {
				notifier := new(fakes.FakeNotifier)
				go runner.Watch(logger, "some-container", notifier)

				eventsCh <- false
				Consistently(notifier.OnEventCallCount).Should(Equal(0))
			})
		})
	})
})
