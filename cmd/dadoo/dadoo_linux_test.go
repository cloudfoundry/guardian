package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Dadoo", func() {
	var (
		cgroupsRoot string
		bundlePath  string
		bundle      goci.Bndl
		bundleSaver = &goci.BundleSaver{}
		runcRoot    string
	)

	BeforeEach(func() {
		cgroupsRoot = filepath.Join(os.TempDir(), fmt.Sprintf("dadoo-tests-cgroups-%d", GinkgoParallelNode()))
		Expect(setupCgroups(cgroupsRoot)).To(Succeed())

		runcRoot = "/run/runc"

		var err error
		bundlePath, err = ioutil.TempDir("", "dadoobundlepath")
		Expect(err).NotTo(HaveOccurred())

		Expect(syscall.Mount("tmpfs", bundlePath, "tmpfs", 0, "")).To(Succeed())

		cmd := exec.Command("runc", "spec")
		cmd.Dir = bundlePath
		Expect(cmd.Run()).To(Succeed())

		loader := &goci.BndlLoader{}
		bundle, err = loader.Load(bundlePath)
		Expect(err).NotTo(HaveOccurred())

		cp, err := gexec.Start(exec.Command("cp", "-a", os.Getenv("GARDEN_TEST_ROOTFS"), filepath.Join(bundlePath, "root")), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(cp, "2m").Should(gexec.Exit(0))

		chown, err := gexec.Start(exec.Command("chown", "-R", "1:1", filepath.Join(bundlePath, "root")), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(chown, "2m").Should(gexec.Exit(0))

		bundle = bundle.
			WithProcess(specs.Process{Args: []string{"/bin/sh", "-c", "exit 12"}, Cwd: "/"}).
			WithRootFS(path.Join(bundlePath, "root")).
			WithNamespace(goci.UserNamespace).
			WithUIDMappings(specs.LinuxIDMapping{HostID: 1, ContainerID: 0, Size: 100}).
			WithGIDMappings(specs.LinuxIDMapping{HostID: 1, ContainerID: 0, Size: 100})

		SetDefaultEventuallyTimeout(10 * time.Second)
	})

	JustBeforeEach(func() {
		Expect(bundleSaver.Save(bundle, path.Join(bundlePath))).To(Succeed())
	})

	AfterEach(func() {
		// Note: We're not umounting the tmpfs here as it can cause a bug in AUFS
		// to surface and lock up the VM running the test
		//syscall.Unmount(bundlePath, 0x2) TODO: is it safe to add this unmount back in now?
		os.RemoveAll(filepath.Join(bundlePath, "root"))
		os.RemoveAll(bundlePath)
	})

	Describe("Exec", func() {
		var (
			processDir                                  string
			runcLogFile                                 *os.File
			stdinPipe, stdoutPipe, stderrPipe, exitPipe string
		)

		openIOPipes := func() {
			_, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.OpenFile(stdoutPipe, os.O_RDONLY, 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.OpenFile(stderrPipe, os.O_RDONLY, 0600)
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			var err error

			bundle = bundle.WithProcess(specs.Process{Args: []string{"/bin/sh", "-c", "sleep 9999"}, Cwd: "/"})
			processDir = filepath.Join(bundlePath, "processes", "abc")
			Expect(os.MkdirAll(processDir, 0777)).To(Succeed())

			runcLogFilePath := filepath.Join(processDir, "exec.log")
			runcLogFile, err = os.Create(runcLogFilePath)
			Expect(err).NotTo(HaveOccurred())

			stdoutPipe = filepath.Join(processDir, "stdout")
			Expect(syscall.Mkfifo(stdoutPipe, 0)).To(Succeed())

			stderrPipe = filepath.Join(processDir, "stderr")
			Expect(syscall.Mkfifo(stderrPipe, 0)).To(Succeed())

			stdinPipe = filepath.Join(processDir, "stdin")
			Expect(syscall.Mkfifo(stdinPipe, 0)).To(Succeed())

			exitPipe = filepath.Join(processDir, "exit")
			Expect(syscall.Mkfifo(exitPipe, 0)).To(Succeed())
		})

		JustBeforeEach(func() {
			// hangs if GinkgoWriter is attached
			cmd := exec.Command("runc", "--root", runcRoot, "create", "--no-new-keyring", "--bundle", bundlePath, filepath.Base(bundlePath))
			Expect(cmd.Run()).To(Succeed())
		})

		AfterEach(func() {
			runcLogFileContents, err := ioutil.ReadAll(runcLogFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(runcLogFile.Close()).To(Succeed())
			fmt.Print(string(runcLogFileContents))

			cmd := exec.Command("runc", "--root", runcRoot, "delete", filepath.Base(bundlePath))
			Expect(cmd.Run()).To(Succeed())
		})

		Context("not requesting a TTY", func() {
			It("should return the exit code of the container process", func() {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "exit 24"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()

				Eventually(sess).Should(gexec.Exit(24))
			})

			It("should write the exit code to a file named exitcode in the container dir", func() {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "exit 24"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()
				Eventually(sess).Should(gexec.Exit(24))

				Eventually(filepath.Join(processDir, "exitcode")).Should(BeAnExistingFile())
				Expect(ioutil.ReadFile(filepath.Join(processDir, "exitcode"))).To(Equal([]byte("24")))
			})

			It("if the process is signalled the exitcode should be 128 + the signal number", func() {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "kill -9 $$"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()
				Eventually(sess).Should(gexec.Exit(128 + 9))

				Eventually(filepath.Join(processDir, "exitcode")).Should(BeAnExistingFile())
				Expect(ioutil.ReadFile(filepath.Join(processDir, "exitcode"))).To(Equal([]byte("137")))
			})

			It("should open the exit pipe and close it when it exits", func() {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "cat <&0"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

				_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				stdin, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdoutPipe, os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stderrPipe, os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				exitFifoCh := make(chan struct{})
				go func() {
					exitFifo, err := os.Open(filepath.Join(processDir, "exit"))
					Expect(err).NotTo(HaveOccurred())

					buf := make([]byte, 1)
					exitFifo.Read(buf)
					close(exitFifoCh)
				}()

				Consistently(exitFifoCh).ShouldNot(BeClosed())
				Expect(stdin.Close()).To(Succeed())
				Eventually(exitFifoCh).Should(BeClosed())
			})

			It("should not destroy the container when the exec process exits", func() {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "exit 24"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()
				Eventually(sess).Should(gexec.Exit(24))

				Consistently(func() *gexec.Session {
					sess, err := gexec.Start(exec.Command("runc", "state", filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					return sess.Wait()
				}).Should(gexec.Exit(0))
			})

			It("should write to the sync pipe when streaming pipes are open", func(done Done) {
				processSpec, err := json.Marshal(&specs.Process{
					Args: []string{"/bin/sh", "-c", "echo hello-world; exit 24"},
					Cwd:  "/",
				})
				Expect(err).NotTo(HaveOccurred())

				syncPipeR, syncPipeW, err := os.Pipe()
				Expect(err).NotTo(HaveOccurred())
				defer syncPipeR.Close()
				defer syncPipeW.Close()

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(processSpec)
				cmd.ExtraFiles = []*os.File{
					mustOpen("/dev/null"),
					runcLogFile,
					syncPipeW,
				}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()

				_, err = os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				// This is a weak assertion that there is a sync message when the pipes are open
				// but does not tell us anything about the timing between the two unfortunately
				syncMsg := make([]byte, 1)
				_, err = syncPipeR.Read(syncMsg)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(24))

				close(done)
			}, 10.0)

			Context("when the -runc-root flag is passed", func() {
				BeforeEach(func() {
					runcRoot = "/tmp/runc"
				})

				It("uses the provided value as the runc root dir", func() {
					processSpec, err := json.Marshal(&specs.Process{
						Args: []string{"/bin/sh", "-c", "exit 0"},
						Cwd:  "/",
					})
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-runc-root", runcRoot, "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.Stdin = bytes.NewReader(processSpec)
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}

					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					openIOPipes()

					matches, err := filepath.Glob("/tmp/runc/*/state.json")
					Expect(err).NotTo(HaveOccurred())

					Expect(len(matches)).To(Equal(1))

					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			It("should write the container's output to the named pipes inside the process dir", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "cat <&0"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()

				stdinP, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdoutP, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				stdinP.WriteString("hello")
				Expect(stdinP.Close()).To(Succeed())

				stdout := make([]byte, len("hello"))
				_, err = stdoutP.Read(stdout)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(stdout)).To(Equal("hello"))
			})

			It("ensures the user process is allowed to write to stdout", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				openIOPipes()

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdoutP, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				stdoutContents := make([]byte, len("hello"))
				_, err = stdoutP.Read(stdoutContents)
				Expect(err).NotTo(HaveOccurred())

				stdoutP.Close()

				Consistently(process.ExitCode, time.Second, time.Millisecond*100).Should(Equal(-1), "expected process to stay alive")
			})
		})

		Context("requesting a TTY", func() {
			var winszPipe string

			BeforeEach(func() {
				winszPipe = filepath.Join(processDir, "winsz")
				Expect(syscall.Mkfifo(winszPipe, 0)).To(Succeed())
			})

			It("should connect the process to a TTY", func() {
				spec := specs.Process{
					Args:     []string{"/bin/sh", "-c", `test -t 1`},
					Cwd:      "/",
					Terminal: true,
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(winszPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
			})

			It("should forward IO", func() {
				spec := specs.Process{
					Args:     []string{"/bin/sh", "-c", `read x; echo "x=$x"`},
					Cwd:      "/",
					Terminal: true,
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				stdin, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdout, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(winszPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = stdin.WriteString("banana\n")
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				data, err := ioutil.ReadAll(stdout)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("x=banana"))
			})

			It("executes the process with a raw tty with onlcr set", func() {
				spec := specs.Process{
					Args: []string{
						"/bin/sh",
						"-c",
						"while true; do stty -a && sleep 1; done",
					},
					Cwd:      "/",
					Terminal: true,
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)

				_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdout, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(winszPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()
				pipeR, pipeW := io.Pipe()
				go io.Copy(pipeW, stdout)
				go io.Copy(buffer, pipeR)

				Eventually(buffer).Should(gbytes.Say(" onlcr"))
				Consistently(buffer, "3s").ShouldNot(gbytes.Say("-onlcr"))
			}, 5.0)

			Context("when defining the window size", func() {
				It("should set initial window size", func() {
					spec := specs.Process{
						Args: []string{
							"/bin/sh",
							"-c",
							`
							# The mechanism that is used to set TTY size (ioctl) is
							# asynchronous. Hence, stty does not return the correct result
							# right after the process is launched.
							sleep 1
							stty -a
						`,
						},
						Cwd:      "/",
						Terminal: true,
						ConsoleSize: specs.Box{
							Height: 17,
							Width:  13,
						},
					}

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
					cmd.Stdin = bytes.NewReader(encSpec)

					_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					stdout, err := os.Open(stdoutPipe)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.Open(stderrPipe)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.OpenFile(winszPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					data, err := ioutil.ReadAll(stdout)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(ContainSubstring("rows 17; columns 13;"))
				}, 5.0)

				It("should update window size", func() {
					spec := specs.Process{
						Args: []string{
							"/bin/sh",
							"-c",
							`
						trap "stty -a" SIGWINCH

						echo hello
						# continuously block so that the trap can keep firing
						for i in $(seq 3); do
						  sleep 1&
						  wait
						done
					`,
						},
						Cwd:      "/",
						Terminal: true,
					}

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
					cmd.Stdin = bytes.NewReader(encSpec)

					_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					stdout, err := os.Open(stdoutPipe)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.Open(stderrPipe)
					Expect(err).NotTo(HaveOccurred())

					winszW, err := os.OpenFile(winszPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					buf := make([]byte, len("hello"))
					stdout.Read(buf)
					Expect(string(buf)).To(Equal("hello"))

					json.NewEncoder(winszW).Encode(&garden.WindowSize{
						Rows:    53,
						Columns: 60,
					})

					data, err := ioutil.ReadAll(stdout)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(ContainSubstring("rows 53; columns 60;"))
				})
			})

			Context("receiving the TTY master via unix socket", func() {
				var (
					encSpec []byte
				)

				BeforeEach(func() {
					spec := specs.Process{
						Args:     []string{"true"},
						Terminal: true,
					}

					var err error
					encSpec, err = json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

				})

				Context("when the path to the parent socket dir is too long", func() {
					var longerThanAllowedSocketPath []byte

					BeforeEach(func() {
						// MaxSocketDirPathLength is defined in main_linux.go as 80
						longerThanAllowedSocketPath = make([]byte, 81, 81)

						for i, _ := range longerThanAllowedSocketPath {
							longerThanAllowedSocketPath[i] = 'a'
						}
					})

					It("exits with 2", func() {
						dadooCmd := exec.Command(dadooBinPath, "-tty", "-socket-dir-path", string(longerThanAllowedSocketPath), "exec", "runc", processDir, filepath.Base(bundlePath))
						dadooCmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
						dadooCmd.Stdin = bytes.NewReader(encSpec)

						stdout := gbytes.NewBuffer()
						dadooSession, err := gexec.Start(dadooCmd, io.MultiWriter(stdout, GinkgoWriter), GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						openIOPipes()

						Eventually(dadooSession).Should(gexec.Exit(2))
						Eventually(stdout).Should(gbytes.Say(fmt.Sprintf("value for --socket-dir-path cannot exceed 80 characters in length")))
					})
				})

				Context("when tty setup fails", func() {
					var (
						fakeRuncBinPath string
					)
					BeforeEach(func() {
						var err error
						fakeRuncBinPath, err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo/fake_runc")
						Expect(err).NotTo(HaveOccurred())
					})

					It("kills the process and exits with 2", func() {
						dadooCmd := exec.Command(dadooBinPath, "-tty", "-socket-dir-path", os.TempDir(), "exec", fakeRuncBinPath, processDir, filepath.Base(bundlePath))
						dadooCmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
						dadooCmd.Stdin = bytes.NewReader(encSpec)

						stdout := gbytes.NewBuffer()
						dadooSession, err := gexec.Start(dadooCmd, io.MultiWriter(stdout, GinkgoWriter), GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						openIOPipes()

						pidFilePath := filepath.Join(processDir, "pidfile")
						Eventually(func() error {
							_, err := os.Stat(pidFilePath)
							return err
						}).ShouldNot(HaveOccurred())

						pidBytes, err := ioutil.ReadFile(pidFilePath)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() error {
							pidCmd := exec.Command("ps", "-p", string(pidBytes))
							pidCmd.Stdout = GinkgoWriter
							pidCmd.Stderr = GinkgoWriter
							return pidCmd.Run()
						}).ShouldNot(Succeed())

						Eventually(dadooSession).Should(gexec.Exit(2))
						Eventually(stdout).Should(gbytes.Say("incorrect number of bytes read"))
					})
				})
			})

			Context("when the winsz pipe doesn't exist", func() {
				BeforeEach(func() {
					Expect(os.Remove(winszPipe)).To(Succeed())
				})

				It("exits with 2", func() {
					spec := specs.Process{
						Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
						Cwd:  "/",
					}

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
					cmd.Stdin = bytes.NewReader(encSpec)
					process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())
					_, err = os.OpenFile(stdoutPipe, os.O_RDONLY, 0600)
					Expect(err).NotTo(HaveOccurred())
					_, err = os.OpenFile(stderrPipe, os.O_RDONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					Eventually(process).Should(gexec.Exit(2))
					Eventually(process).Should(gbytes.Say("open %s: no such file or directory", winszPipe))
				})
			})
		})

		Context("when the stdin pipe doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(stdinPipe)).To(Succeed())
			})

			It("exits with 2", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(process).Should(gexec.Exit(2))
				Eventually(process).Should(gbytes.Say("open %s: no such file or directory", stdinPipe))
			})
		})

		Context("when the stdout pipe doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(stdoutPipe)).To(Succeed())
			})

			It("exits with 2", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				Eventually(process).Should(gexec.Exit(2))
				Eventually(process).Should(gbytes.Say("open %s: no such file or directory", stdoutPipe))
			})
		})

		Context("when the stderr pipe doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(stderrPipe)).To(Succeed())
			})

			It("exits with 2", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.OpenFile(stdoutPipe, os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				Eventually(process).Should(gexec.Exit(2))
				Eventually(process).Should(gbytes.Say("open %s: no such file or directory", stderrPipe))
			})
		})

		Context("when the exit code pipe doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(exitPipe)).To(Succeed())
			})

			It("exits with 2", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "while true; do echo hello; sleep 0.1; done"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), runcLogFile, mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)
				process, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.OpenFile(stdoutPipe, os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.OpenFile(stderrPipe, os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				Eventually(process).Should(gexec.Exit(2))
				Eventually(process).Should(gbytes.Say("open %s: no such file or directory", exitPipe))
			})
		})
	})
})

func mustOpen(path string) *os.File {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

func setupCgroups(cgroupsRoot string) error {
	logger := lagertest.NewTestLogger("test")
	runner := linux_command_runner.New()

	starter := rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsRoot, runner)

	return starter.Start()
}

func devNull() *os.File {
	f, err := os.OpenFile("/dev/null", os.O_APPEND, 0700)
	Expect(err).NotTo(HaveOccurred())
	return f
}
