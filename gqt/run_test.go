package gqt_test

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Run", func() {
	var client *runner.RunningGarden

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	DescribeTable("running a process",
		func(spec garden.ProcessSpec, matchers ...func(actual interface{})) {
			client = startGarden()
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

	It("cleans up any files by the time the process exits", func() {
		client = startGarden()
		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		before := filesInDir(filepath.Join(client.DepotDir, container.Handle()))

		process, err := container.Run(garden.ProcessSpec{
			Path: "echo",
			Args: []string{
				"ohai",
			},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())
		Expect(process.Wait()).To(Equal(0))

		after := filesInDir(filepath.Join(client.DepotDir, container.Handle()))

		Expect(after).To(ConsistOf(before))
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
		client = startGarden()

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
				var old syscall.Rlimit
				Expect(syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old)).To(Succeed())

				Expect(syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{
					Max: 100000,
					Cur: 100000,
				})).To(Succeed())

				defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)

				client = startGarden()
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
				var err error
				rootfs, err = ioutil.TempDir("", "symlinks")
				Expect(err).NotTo(HaveOccurred())

				Expect(os.Mkdir(filepath.Join(rootfs, "tmp"), 0777)).To(Succeed())

				target, err = ioutil.TempDir("", "symlinkstarget")
				Expect(err).NotTo(HaveOccurred())

				Expect(os.Symlink(target, path.Join(rootfs, "symlink"))).To(Succeed())
			})

			It("does not follow symlinks into the host when creating cwd", func() {
				client = startGarden()
				container, err := client.Create(garden.ContainerSpec{RootFSPath: rootfs})
				Expect(err).NotTo(HaveOccurred())

				_, err = container.Run(garden.ProcessSpec{
					Path: "echo",
					Args: []string{"hello"},
					Dir:  "/symlink/foo/bar",
				}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
				Expect(err).To(HaveOccurred()) // echo won't be in the rootfs
				Expect(path.Join(target, "foo")).NotTo(BeADirectory())
			})
		})
	})

	Context("when container is privileged", func() {
		It("can run a process as a particular user", func() {
			client = startGarden()
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
			client = startGarden()
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
			client = startGarden()
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
		It("forwards runc logs to lager when exec fails, and gives proper error messages", func() {
			client = startGarden("--log-level", "debug")
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			_, err = container.Run(garden.ProcessSpec{
				Path: "does-not-exit",
			}, garden.ProcessIO{})
			Expect(err).To(MatchError(ContainSubstring("executable file not found")))

			Eventually(client).Should(gbytes.Say(`execrunner.runc`))
		})

		It("forwards runc logs to lager when exec fails, and gives proper error messages when requesting a TTY", func() {
			client = startGarden("--log-level", "debug")
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
			Expect(err).To(MatchError(ContainSubstring("executable file not found")))

			Eventually(client).Should(gbytes.Say(`execrunner.runc`))
		})

		Describe("Signalling", func() {
			It("should forward SIGTERM to the process", func() {
				client = startGarden()

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
	})

	Describe("Signalling", func() {
		It("should forward SIGTERM to the process", func(done Done) {
			client = startGarden()

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

			Expect(proc.Wait()).To(Equal(42))

			close(done)
		}, 20.0)
	})
})

var _ = Describe("Attach", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
		processID string
		args      []string
	)

	BeforeEach(func() {
		// we need to pass --properties-path to prevent guardian from deleting containers
		// after restarting the server
		propertiesDir, err := ioutil.TempDir("", "props")
		Expect(err).NotTo(HaveOccurred())
		args = []string{"--properties-path", path.Join(propertiesDir, "props.json")}

		client = startGarden(args...)
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

			restartGarden(client, args...)
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
			pidBytes, err := ioutil.ReadFile(hostPidFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Stop()).To(Succeed())

			pid, err := strconv.Atoi(string(pidBytes))
			Expect(err).NotTo(HaveOccurred())

			hostProcess, err := os.FindProcess(pid)
			Expect(err).NotTo(HaveOccurred())

			Expect(hostProcess.Kill()).To(Succeed())

			client = startGarden(args...)
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

func shouldNot(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).ToNot(matcher)
		}
	}
}

func spec(path string, args ...string) garden.ProcessSpec {
	return garden.ProcessSpec{
		Path: path,
		Args: args,
	}
}

func filesInDir(path string) []string {
	files := make([]string, 0)
	Expect(filepath.Walk(path, func(p string, i os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if i.Mode()&os.ModeDir == os.ModeDir {
			return nil
		}

		files = append(files, p)

		return nil
	})).To(Succeed())
	return files
}

func ttySpec(path string, args ...string) garden.ProcessSpec {
	base := spec(path, args...)
	base.TTY = &garden.TTYSpec{}
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
