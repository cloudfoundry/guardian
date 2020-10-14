package gqt_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"golang.org/x/sys/unix"
)

var _ = Describe("Run", func() {
	var (
		client        *runner.RunningGarden
		container     garden.Container
		processSpec   garden.ProcessSpec
		containerSpec garden.ContainerSpec
		out           *gbytes.Buffer
		exitCode      int
		propsDir      string
		processID     string
		processPath   string
	)

	BeforeEach(func() {
		out = gbytes.NewBuffer()
		containerSpec = garden.ContainerSpec{}
		processSpec = makeSpec("/bin/sh", "-c", "echo hello; exit 12")
		propsDir = tempDir("", "props")
		// we need to pass --properties-path to prevent guardian from deleting containers
		// after restarting the server
		config.PropertiesPath = path.Join(propsDir, "props.json")
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(propsDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		client = runner.Start(config)

		var err error
		container, err = client.Create(containerSpec)
		Expect(err).NotTo(HaveOccurred())

		proc, err := container.Run(
			processSpec,
			garden.ProcessIO{
				Stdout: io.MultiWriter(GinkgoWriter, out),
				Stderr: io.MultiWriter(GinkgoWriter, out),
			})
		Expect(err).NotTo(HaveOccurred())

		exitCode, err = proc.Wait()
		Expect(err).NotTo(HaveOccurred())

		processID = proc.ID()
		processPath = filepath.Join(client.DepotDir, container.Handle(), "processes", processID)
	})

	It("execs the process", func() {
		Expect(out).To(gbytes.Say("hello"))
		Expect(exitCode).To(Equal(12))
	})

	Context("with a command present in the PATH", func() {
		BeforeEach(func() {
			processSpec = makeSpec("sh", "-c", "echo potato; exit 24")
		})

		It("execs the process", func() {
			Expect(out).To(gbytes.Say("potato"))
			Expect(exitCode).To(Equal(24))
		})
	})

	Describe("TTY", func() {
		BeforeEach(func() {
			processSpec = makeSpec("test", "-t", "1")
		})

		It("does not allocate a TTY", func() {
			Expect(exitCode).To(Equal(1))
		})

		Context("when a TTY is requested", func() {
			BeforeEach(func() {
				processSpec.TTY = new(garden.TTYSpec)
			})

			It("allocates a TTY", func() {
				Expect(exitCode).To(Equal(0))
			})

			It("does not leak FIFOs", func() {
				Eventually(func() string {
					return lsofFileHandlesOnProcessPipes(processID)
				}, "1m").Should(BeEmpty())
			})
		})
	})

	Describe("IO", func() {
		BeforeEach(func() {
			processSpec = makeSpec("sh", "-c", "echo foo > /dev/stdout")
		})

		It("can write to /dev/stdout", func() {
			Expect(exitCode).To(Equal(0))
			Expect(out).To(gbytes.Say("foo"))
		})
	})

	It("does not delete the process directory", func() {
		skipIfContainerdForProcesses("There is no processes directory in the depot when running processes with containerd")
		Expect(processPath).To(BeADirectory())
	})

	It("can re-attach to the process and Wait on it", func() {
		reattachedProcess, err := container.Attach(processID, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())

		code, err := reattachedProcess.Wait()
		Expect(err).NotTo(HaveOccurred())
		Expect(code).To(Equal(12))
	})

	Context("when --cleanup-process-dirs-on-wait is set", func() {
		BeforeEach(func() {
			config.CleanupProcessDirsOnWait = boolptr(true)
		})

		It("deletes the process directory", func() {
			skipIfContainerdForProcesses("There is no processes directory in the depot when running processes with containerd")
			Expect(processPath).NotTo(BeAnExistingFile())
		})

		FIt("deletes the process metadata", func() {
			skipIfRunDmcForProcesses("Processes not managed by containerd")
			fmt.Println(">>>>>>>>>>>>>>>>>>>>", processID)
			Eventually(func() error {
				_, err := container.Attach(processID, garden.ProcessIO{})
				return err
			}, "10s").ShouldNot(Succeed())

			_, err := container.Attach(processID, garden.ProcessIO{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(garden.ProcessNotFoundError{}))
		})
	})

	It("creates process files with the right permission and ownership", func() {
		skipIfContainerdForProcesses("There is no processes directory in the depot when running processes with containerd")

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

	It("cleans up file handles when the process exits", func() {
		Expect(lsofFileHandlesOnProcessPipes(processID)).To(BeEmpty())
	})

	Describe("security", func() {
		Describe("rlimits", func() {
			var old unix.Rlimit

			BeforeEach(func() {
				Expect(unix.Getrlimit(unix.RLIMIT_NOFILE, &old)).To(Succeed())
				Expect(unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{
					Max: 100000,
					Cur: 100000,
				})).To(Succeed())

				processSpec.Args = []string{"-c", "ulimit -a"}
				limit := uint64(100001)
				processSpec.Limits = garden.ResourceLimits{
					Nofile: &limit,
				}
			})

			AfterEach(func() {
				unix.Setrlimit(unix.RLIMIT_NOFILE, &old)
			})

			It("sets requested rlimits, even if they are increased above current limit", func() {
				Expect(out).To(gbytes.Say("file descriptors\\W+100001"))
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
				containerSpec.RootFSPath = rootfs
				processSpec.Dir = "/symlink/foo/bar"
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Dir(rootfs))).To(Succeed())
				Expect(os.RemoveAll(target)).To(Succeed())
			})

			It("does not follow symlinks into the host when creating cwd", func() {
				Expect(path.Join(target, "foo")).NotTo(BeADirectory())
			})
		})

		Describe("sgids", func() {
			When("gdn process has supplementary groups", func() {
				BeforeEach(func() {
					config.User = &syscall.Credential{
						Uid:    0,
						Gid:    0,
						Groups: []uint32{42},
					}
					processSpec = makeSpec("id")
				})

				It("does not propagate them to the container process", func() {
					Expect(out).NotTo(gbytes.Say(`\b42\b`))
				})

				It("uses supplementary groups from /etc/group for the user", func() {
					// busybox has root in the wheel supplementary group
					Expect(out).To(gbytes.Say(`\b10\b`))
				})
			})
		})
	})

	Context("when container is privileged", func() {
		BeforeEach(func() {
			containerSpec.Privileged = true
			processSpec = makeSpec("whoami")
			processSpec.User = "alice"
		})

		It("can run a process as a particular user", func() {
			Expect(out).To(gbytes.Say("alice"))
		})
	})

	Describe("PATH env variable", func() {
		BeforeEach(func() {
			processSpec.Args = []string{"-c", "echo $PATH"}
		})

		It("includes the `sbin` folders", func() {
			Expect(out).To(gbytes.Say(`^/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n$`))
		})

		Context("for a non-root user", func() {
			BeforeEach(func() {
				processSpec.User = "alice"
			})

			It("does not include the `sbin` folders", func() {
				Expect(out).To(gbytes.Say(`^/usr/local/bin:/usr/bin:/bin\n$`))
			})
		})

		Context("with an environment variable containing `PATH` in the name", func() {
			BeforeEach(func() {
				processSpec.Env = []string{"APATH=foo"}
			})

			It("ignores it", func() {
				Expect(out).To(gbytes.Say(`^/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n$`))
			})
		})
	})

	Describe("USER env variable", func() {
		BeforeEach(func() {
			containerSpec.Env = []string{"USER=ppp", "HOME=/home/ppp"}
			processSpec.Args = []string{"-c", "env"}
		})

		It("is inherited from the container spec", func() {
			Expect(out).To(gbytes.Say("USER=ppp"))
			Expect(out).To(gbytes.Say("HOME=/home/ppp"))
		})

		Context("when the USER env var is not set in the container spec", func() {
			BeforeEach(func() {
				containerSpec.Env = []string{}
				processSpec.User = "alice"
			})

			It("sets the value using the process spec user", func() {
				Expect(out).To(gbytes.Say("USER=alice"))
				Expect(out).To(gbytes.Say("HOME=/home/alice"))
			})
		})

		Context("when the user is set in the process spec", func() {
			BeforeEach(func() {
				processSpec.User = "alice"
			})

			It("maintains the value from the container spec", func() {
				Expect(out).To(gbytes.Say("USER=ppp"))
				Expect(out).To(gbytes.Say("HOME=/home/ppp"))
			})
		})

		Context("when the USER env var is set in the process spec", func() {
			BeforeEach(func() {
				processSpec.Env = []string{"USER=alice"}
			})

			It("gets overridden", func() {
				Expect(out).To(gbytes.Say("USER=alice"))
				Expect(out).To(gbytes.Say("HOME=/home/ppp"))
			})
		})

		Context("when both the user and the USER env var are set in the process spec", func() {
			BeforeEach(func() {
				processSpec.User = "alice"
				processSpec.Env = []string{"USER=bob"}
			})

			It("gets overridden", func() {
				Expect(out).To(gbytes.Say("USER=bob"))
				Expect(out).To(gbytes.Say("HOME=/home/ppp"))
			})
		})

		Context("when there is an env var containing `USER` in the name", func() {
			BeforeEach(func() {
				processSpec.Env = []string{"NOT_USER=alice"}
			})

			It("is not affected", func() {
				Expect(out).To(gbytes.Say("USER=ppp"))
				Expect(out).To(gbytes.Say("HOME=/home/ppp"))
			})
		})
	})

	Describe("environment", func() {
		BeforeEach(func() {
			containerSpec.Env = []string{"ONE=1"}
			processSpec.Args = []string{"-c", "env"}
		})

		It("is inherited from the container spec", func() {
			Expect(out).To(gbytes.Say("ONE=1"))
		})

		Context("when it is specified in the process spec", func() {
			BeforeEach(func() {
				processSpec.Env = []string{"TWO=2"}
			})

			It("is merged with the environment in the container spec", func() {
				Expect(out).To(gbytes.Say("ONE=1"))
				Expect(out).To(gbytes.Say("TWO=2"))
			})
		})

		Context("when an environment variable is set in both the container spec and the process spec", func() {
			BeforeEach(func() {
				processSpec.Env = []string{"ONE=42"}
			})

			It("uses the value from the process spec", func() {
				Expect(out).To(gbytes.Say("ONE=42"))
			})
		})
	})

	Describe("dadoo exec", func() {
		BeforeEach(func() {
			config.LogLevel = "debug"
		})

		Context("when runc writes a lot of stderr before exiting", func() {
			var propertiesDir string

			BeforeEach(func() {
				skipIfContainerdForProcesses("When running processes with containerd we are not calling runc directly.")
			})

			JustBeforeEach(func() {
				config.RuntimePluginBin = binaries.FakeRuncStderr
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
			_, err := container.Run(garden.ProcessSpec{
				Path: "does-not-exit",
			}, garden.ProcessIO{})

			Expect(err).To(MatchError(ContainSubstring("executable file not found")))
			Eventually(client).Should(gbytes.Say("executable file not found"))
		})

		It("forwards runc logs to lager when exec fails, and gives proper error messages when requesting a TTY", func() {
			_, err := container.Run(garden.ProcessSpec{
				Path: "does-not-exit",
				TTY:  &garden.TTYSpec{},
			}, garden.ProcessIO{})

			Expect(err).To(MatchError(ContainSubstring("executable file not found")))
			Eventually(client).Should(gbytes.Say("executable file not found"))
		})
	})

	Describe("Signalling", func() {
		It("should forward SIGTERM to the process", func() {
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
				binaryPath = "/bin/does-not-exist"
			})

			JustBeforeEach(func() {
				config.DebugIP = "0.0.0.0"
				config.DebugPort = intptr(8080 + GinkgoParallelNode())
				client = restartGarden(client, config)

				var err error
				stackBefore, err = client.StackDump()
				Expect(err).NotTo(HaveOccurred())
				numGoRoutinesBefore = numGoRoutines(client)

				_, runErr = container.Run(garden.ProcessSpec{
					Path: binaryPath,
				}, garden.ProcessIO{})
			})

			It("returns a useful error type", func() {
				Expect(runErr).To(BeAssignableToTypeOf(garden.ExecutableNotFoundError{}))
			})

			Context("when the executable should be somewhere on the $PATH", func() {
				BeforeEach(func() {
					binaryPath = "does-not-exist"
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

				Eventually(pollNumGoRoutines(client), time.Minute).Should(
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
		propsDir  string
	)

	BeforeEach(func() {
		propsDir = tempDir("", "props")
		// we need to pass --properties-path to prevent guardian from deleting containers
		// after restarting the server
		config.PropertiesPath = path.Join(propsDir, "props.json")
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(propsDir)).To(Succeed())
	})

	Context("when attaching to a running process", func() {
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

		Context("when signalling the process we attached to", func() {
			var (
				attachedProcess garden.Process
				signal          garden.Signal
			)

			BeforeEach(func() {
				var err error
				attachedProcess, err = container.Attach(processID, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err := attachedProcess.Signal(signal)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when signalling with SIGTERM", func() {
				BeforeEach(func() {
					signal = garden.SignalTerminate
				})

				It("returns signal number + 128", func() {
					exitCode, err := attachedProcess.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(128 + 15))
				})
			})

			Context("when signalling with SIGKILL", func() {
				BeforeEach(func() {
					signal = garden.SignalKill
				})

				It("returns signal number + 128", func() {
					exitCode, err := attachedProcess.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(128 + 9))
				})
			})
		})
	})

	Context("when attaching to an exited process", func() {
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

			var pidString string
			if isContainerdForProcesses() {
				pidString = getContainerdProcessPid("ctr", config.ContainerdSocket, container.Handle(), process.ID())
			} else {
				hostProcessDir := filepath.Join(client.DepotDir, container.Handle(), "processes", processID)
				hostPidFilePath := filepath.Join(hostProcessDir, "pidfile")
				pidString = readFileString(hostPidFilePath)
			}

			Expect(client.Stop()).To(Succeed())

			pid, err := strconv.Atoi(pidString)
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

func makeSpec(path string, args ...string) garden.ProcessSpec {
	return garden.ProcessSpec{
		Path: path,
		Args: args,
	}
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

func lsofFileHandlesOnProcessPipes(processID string) string {
	grepProcID := exec.Command("grep", processID)
	lsof := exec.Command("lsof")

	lsofOutPipe, err := lsof.StdoutPipe()
	Expect(err).NotTo(HaveOccurred())
	defer lsofOutPipe.Close()

	stdoutBuf := gbytes.NewBuffer()
	grepProcID.Stdin = lsofOutPipe
	grepProcID.Stdout = stdoutBuf
	Expect(grepProcID.Start()).To(Succeed())

	Expect(lsof.Run()).To(Succeed())

	_ = grepProcID.Wait()

	return string(stdoutBuf.Contents())
}
