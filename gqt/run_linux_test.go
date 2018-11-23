package gqt_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
	"golang.org/x/sys/unix"
)

var _ = Describe("Run", func() {
	var client *runner.RunningGarden

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	DescribeTable("running a process",
		func(spec garden.ProcessSpec, matchers ...func(actual interface{})) {
			client = runner.Start(config)
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				spec,
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())

			for _, m := range matchers {
				m(&process{exitCode, out})
			}
		},

		Entry("with an absolute path",
			spec("/bin/sh", "-c", "echo hello; exit 12"),
			should(gbytes.Say("hello"), gexec.Exit(12)),
		),

		Entry("with a path to be found in a regular user's path",
			spec("sh", "-c", "echo potato; exit 24"),
			should(gbytes.Say("potato"), gexec.Exit(24)),
		),

		Entry("without a TTY",
			spec("test", "-t", "1"),
			should(gexec.Exit(1)),
		),

		Entry("with a TTY",
			ttySpec("test", "-t", "1"),
			should(gexec.Exit(0)),
		),
	)

	Describe("when we wait for process", func() {
		var (
			container   garden.Container
			process     garden.Process
			processPath string
		)

		JustBeforeEach(func() {
			client = runner.Start(config)
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			process, err = container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{"-c", "exit 13"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			code, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(13))

			processPath = filepath.Join(client.DepotDir, container.Handle(), "processes", process.ID())
		})

		Context("when --cleanup-process-dirs-on-wait is not set (default)", func() {
			It("does not delete the process directory", func() {
				Expect(processPath).To(BeADirectory())
			})

			Context("when we reattach", func() {
				It("can be Waited for again", func() {
					reattachedProcess, err := container.Attach(process.ID(), garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					code, err := reattachedProcess.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(code).To(Equal(13))
				})
			})
		})

		Context("when --cleanup-process-dirs-on-wait is set", func() {
			BeforeEach(func() {
				config.CleanupProcessDirsOnWait = boolptr(true)
			})

			It("deletes the proccess directory", func() {
				Expect(processPath).NotTo(BeAnExistingFile())
			})
		})
	})

	It("creates process files with the right permisssion and ownership", func() {
		client = runner.Start(config)
		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		process, err := container.Run(garden.ProcessSpec{
			Path: "sleep",
			Args: []string{"50"},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())

		processPath := filepath.Join(client.DepotDir, container.Handle(), "processes", process.ID())
		root := uint32(0)
		maximus := uint32(4294967294)
		files := []fileInfo{
			{dir: processPath, mode: "drwx------", owner: root},
			{dir: processPath, name: "exit", mode: "prw-------", owner: maximus},
			{dir: processPath, name: "stdin", mode: "prw-------", owner: maximus},
			{dir: processPath, name: "stdout", mode: "prw-------", owner: maximus},
			{dir: processPath, name: "stderr", mode: "prw-------", owner: maximus},
			{dir: processPath, name: "winsz", mode: "prw-------", owner: maximus},
		}
		for _, info := range files {
			Expect(checkFileInfo(info)).NotTo(HaveOccurred())
		}
	})

	lsofFileHandlesOnProcessPipes := func(processID string) string {

		grepProcID := exec.Command("grep", processID)
		lsof := exec.Command("lsof")

		lsofOutPipe, err := lsof.StdoutPipe()
		defer lsofOutPipe.Close()
		Expect(err).NotTo(HaveOccurred())

		stdoutBuf := gbytes.NewBuffer()
		grepProcID.Stdin = lsofOutPipe
		grepProcID.Stdout = stdoutBuf
		Expect(grepProcID.Start()).To(Succeed())

		Expect(lsof.Run()).To(Succeed())

		grepProcID.Wait()

		return string(stdoutBuf.Contents())
	}

	It("cleans up file handles when the process exits", func() {
		client = runner.Start(config)

		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		process, err := container.Run(garden.ProcessSpec{
			Path: "echo",
			Args: []string{
				"ohai",
			},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())
		Expect(process.Wait()).To(Equal(0))

		Expect(lsofFileHandlesOnProcessPipes(process.ID())).To(BeEmpty())
	})

	Describe("security", func() {
		Describe("rlimits", func() {
			It("sets requested rlimits, even if they are increased above current limit", func() {
				var old unix.Rlimit
				Expect(unix.Getrlimit(unix.RLIMIT_NOFILE, &old)).To(Succeed())

				Expect(unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{
					Max: 100000,
					Cur: 100000,
				})).To(Succeed())

				defer unix.Setrlimit(unix.RLIMIT_NOFILE, &old)

				client = runner.Start(config)
				container, err := client.Create(garden.ContainerSpec{
					Privileged: false,
				})
				Expect(err).NotTo(HaveOccurred())

				limit := uint64(100001)
				stdout := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "root",
					Path: "/bin/sh",
					Args: []string{"-c", "ulimit -a"},
					Limits: garden.ResourceLimits{
						Nofile: &limit,
					},
				}, garden.ProcessIO{
					Stdout: stdout,
					Stderr: GinkgoWriter,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(process.Wait()).To(Equal(0))
				Expect(stdout).To(gbytes.Say("file descriptors\\W+100001"))
			})
		})

		Describe("symlinks", func() {
			var (
				target, rootfs string
			)

			BeforeEach(func() {
				target = tempDir("", "symlinkstarget")

				rootfs = createRootfsTar(func(unpackedRootfs string) {
					Expect(os.Symlink(target, path.Join(unpackedRootfs, "symlink"))).To(Succeed())
				})
			})

			AfterEach(func() {
				Expect(os.RemoveAll(target)).To(Succeed())
			})

			It("does not follow symlinks into the host when creating cwd", func() {
				client = runner.Start(config)
				container, err := client.Create(garden.ContainerSpec{RootFSPath: rootfs})
				Expect(err).NotTo(HaveOccurred())

				_, err = container.Run(garden.ProcessSpec{
					Path: "non-existing-cmd",
					Args: []string{},
					Dir:  "/symlink/foo/bar",
				}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
				Expect(err).To(HaveOccurred())
				Expect(path.Join(target, "foo")).NotTo(BeADirectory())
			})
		})
	})

	Context("when container is privileged", func() {
		It("can run a process as a particular user", func() {
			client = runner.Start(config)
			container, err := client.Create(garden.ContainerSpec{
				Privileged: true,
			})
			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "whoami",
					User: "alice",
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say("alice"))
		})
	})

	Describe("PATH env variable", func() {
		var container garden.Container

		BeforeEach(func() {
			client = runner.Start(config)
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("contains the correct values", func(user, path string, env []string) {
			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", "echo $PATH"},
					User: user,
					Env:  env,
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say(path))
		},
			Entry("for a non-root user",
				"alice", `^/usr/local/bin:/usr/bin:/bin\n$`, []string{}),
			Entry("for the root user",
				"root", `^/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n$`, []string{}),
			Entry("with an env variable matching the string .*PATH.*",
				"alice", `^/usr/local/bin:/usr/bin:/bin\n$`, []string{"APATH=foo"}),
		)
	})

	Describe("USER env variable", func() {
		var container garden.Container

		BeforeEach(func() {
			client = runner.Start(config)
			var err error
			container, err = client.Create(garden.ContainerSpec{
				Env: []string{"USER=ppp", "HOME=/home/ppp"},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("contains the correct values", func(user string, env, paths []string) {
			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", "env"},
					User: user,
					Env:  env,
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			for _, path := range paths {
				Expect(out).To(gbytes.Say(path))
			}
		},
			Entry(
				"for empty user",
				"", []string{}, []string{"USER=ppp", "HOME=/home/ppp"},
			),
			Entry(
				"when we specify the USER env in processSpec",
				"alice", []string{"USER=alice", "HI=YO"}, []string{"USER=alice", "HOME=/home/ppp", "HI=YO"},
			),
			Entry(
				"with an env variable matching the string .*USER.*",
				"alice", []string{"USER=alice", "HI=YO", "AUSER=foo"}, []string{"USER=alice", "HOME=/home/ppp", "HI=YO", "AUSER=foo"},
			),
		)
	})

	Describe("dadoo exec", func() {
		Context("when runc writes a lot of stderr before exiting", func() {
			var (
				container     garden.Container
				propertiesDir string
			)

			BeforeEach(func() {
				propertiesDir = tempDir("", "props")

				config.PropertiesPath = path.Join(propertiesDir, "props.json")
				client = runner.Start(config)

				var err error
				container, err = client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				fakeRuncBinPath, err := gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_runc_stderr")
				Expect(err).NotTo(HaveOccurred())

				config.RuntimePluginBin = fakeRuncBinPath
				client = restartGarden(client, config)
			})

			AfterEach(func() {
				config.RuntimePluginBin = ""
				client = restartGarden(client, config)
				Expect(os.RemoveAll(propertiesDir)).To(Succeed())
			})

			It("does not deadlock", func(done Done) {
				_, err := container.Run(garden.ProcessSpec{
					Path: "ps",
				}, garden.ProcessIO{
					Stderr: gbytes.NewBuffer(),
				})
				Expect(err).To(MatchError(ContainSubstring("exit status 100")))

				close(done)
			}, 30.0)
		})

		It("forwards runc logs to lager when exec fails, and gives proper error messages", func() {
			config.LogLevel = "debug"
			client = runner.Start(config)
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			_, err = container.Run(garden.ProcessSpec{
				Path: "does-not-exit",
			}, garden.ProcessIO{})
			runcErrorMessage := "executable file not found"
			Expect(err).To(MatchError(ContainSubstring(runcErrorMessage)))
			Eventually(client).Should(gbytes.Say(runcErrorMessage))
		})

		It("forwards runc logs to lager when exec fails, and gives proper error messages when requesting a TTY", func() {
			config.LogLevel = "debug"
			client = runner.Start(config)
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			_, err = container.Run(garden.ProcessSpec{
				Path: "does-not-exit",
				TTY: &garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: 1,
						Rows:    1,
					},
				},
			}, garden.ProcessIO{})
			runcErrorMessage := "executable file not found"
			Expect(err).To(MatchError(ContainSubstring(runcErrorMessage)))
			Eventually(client).Should(gbytes.Say(runcErrorMessage))
		})
	})

	Describe("Signalling", func() {
		It("should forward SIGTERM to the process", func() {
			client = runner.Start(config)

			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			proc, err := container.Run(garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", `
					trap 'exit 42' TERM

					while true; do
					  echo 'sleeping'
					  sleep 1
					done
				`},
			}, garden.ProcessIO{
				Stdout: buffer,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(buffer).Should(gbytes.Say("sleeping"))

			err = proc.Signal(garden.SignalTerminate)
			Expect(err).NotTo(HaveOccurred())

			status := make(chan int)
			go func() {
				exit, err := proc.Wait()
				Expect(err).NotTo(HaveOccurred())
				status <- exit
			}()

			Eventually(status).Should(Receive(BeEquivalentTo(42)))
		})
	})

	Describe("Errors", func() {
		Context("when trying to run an executable which does not exist", func() {
			var (
				runErr              error
				binaryPath          string
				numGoRoutinesBefore int
				stackBefore         string
			)

			BeforeEach(func() {
				binaryPath = "/i/do/not/exist"
			})

			JustBeforeEach(func() {
				config.DebugIP = "0.0.0.0"
				config.DebugPort = intptr(8080 + GinkgoParallelNode())
				client = runner.Start(config)

				container, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				stackBefore, err = client.StackDump()
				Expect(err).NotTo(HaveOccurred())
				numGoRoutinesBefore = numGoRoutines(client)

				_, runErr = container.Run(garden.ProcessSpec{
					Path: binaryPath,
				}, garden.ProcessIO{})
			})

			Context("when the executable is a fully qualified path", func() {
				BeforeEach(func() {
					binaryPath = "/bin/fake"
				})

				It("returns a useful error type", func() {
					Expect(runErr).To(BeAssignableToTypeOf(garden.ExecutableNotFoundError{}))
				})
			})

			Context("when the executable should be somewhere on the $PATH", func() {
				BeforeEach(func() {
					binaryPath = "fake-path"
				})

				It("returns a useful error type", func() {
					Expect(runErr).To(BeAssignableToTypeOf(garden.ExecutableNotFoundError{}))
				})
			})

			It("should not leak go routines", func() {
				getStackDump := func() string {
					s, e := client.StackDump()
					if e != nil {
						return fmt.Sprintf("<Failed to get stack dump: %v>", e)
					}
					return s
				}

				Eventually(pollNumGoRoutines(client), time.Second*30).Should(
					Equal(numGoRoutinesBefore),
					fmt.Sprintf("possible go routine leak\n\n--- stack dump before ---\n%s\n\n--- stack dump after ---\n%s\n", stackBefore, getStackDump()),
				)
			})
		})
	})
})

var _ = Describe("Attach", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
		processID string
	)

	BeforeEach(func() {
		// we need to pass --properties-path to prevent guardian from deleting containers
		// after restarting the server
		config.PropertiesPath = path.Join(tempDir("", "props"), "props.json")
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("when the process exits after calling .Attach", func() {
		BeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			process, err := container.Run(garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "sleep 10; exit 13"},
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())
			processID = process.ID()

			client = restartGarden(client, config)
		})

		It("returns the exit code", func() {
			attachedProcess, err := container.Attach(processID, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := attachedProcess.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(13))
		})
	})

	Context("when the process exits before calling .Attach", func() {
		BeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			process, err := container.Run(garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", `
					while true; do
						echo 'sleeping'
					  sleep 1
					done
				`},
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())

			processID = process.ID()
			hostProcessDir := filepath.Join(client.DepotDir, container.Handle(), "processes", processID)
			hostPidFilePath := filepath.Join(hostProcessDir, "pidfile")

			// Finds the pid on the host.
			pidFileContent := readFileString(hostPidFilePath)

			Expect(client.Stop()).To(Succeed())

			pid, err := strconv.Atoi(pidFileContent)
			Expect(err).NotTo(HaveOccurred())

			hostProcess, err := os.FindProcess(pid)
			Expect(err).NotTo(HaveOccurred())

			Expect(hostProcess.Kill()).To(Succeed())

			client = runner.Start(config)
		})

		It("returns the exit code (and doesn't hang!)", func() {
			attachedProcess, err := container.Attach(processID, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := attachedProcess.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(137)) // 137 = exit code when a process is KILLed
		})
	})
})

func should(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).To(matcher)
		}
	}
}

func spec(path string, args ...string) garden.ProcessSpec {
	return garden.ProcessSpec{
		Path: path,
		Args: args,
	}
}

func ttySpec(path string, args ...string) garden.ProcessSpec {
	base := spec(path, args...)
	base.TTY = new(garden.TTYSpec)
	return base
}

type process struct {
	exitCode int
	buffer   *gbytes.Buffer
}

func (p *process) ExitCode() int {
	return p.exitCode
}

func (p *process) Buffer() *gbytes.Buffer {
	return p.buffer
}

type fileInfo struct {
	dir   string
	name  string
	mode  string
	owner uint32
}

func checkFileInfo(expectedInfo fileInfo) error {
	path := filepath.Join(expectedInfo.dir, expectedInfo.name)
	actualInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if actualInfo.Mode().String() != expectedInfo.mode {
		return fmt.Errorf("mode %v is not the expected %v of file %v", actualInfo.Mode(), expectedInfo.mode, path)
	}

	var stat unix.Stat_t
	if err = unix.Stat(path, &stat); err != nil {
		return err
	}

	if uid := stat.Uid; uid != expectedInfo.owner {
		return fmt.Errorf("owner %v is not the expected %v of file %v", uid, expectedInfo.owner, path)
	}
	return nil
}
