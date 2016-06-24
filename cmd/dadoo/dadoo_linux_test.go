package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Dadoo", func() {
	var (
		bundlePath string
		bundle     goci.Bndl
	)

	BeforeEach(func() {
		setupCgroups()

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
			WithUIDMappings(specs.IDMapping{HostID: 1, ContainerID: 0, Size: 100}).
			WithGIDMappings(specs.IDMapping{HostID: 1, ContainerID: 0, Size: 100})

		SetDefaultEventuallyTimeout(10 * time.Second)
	})

	JustBeforeEach(func() {
		Expect(bundle.Save(path.Join(bundlePath))).To(Succeed())
	})

	AfterEach(func() {
		// Note: We're not umounting the tmpfs here as it can cause a bug in AUFS
		// to surface and lock up the VM running the test
		os.RemoveAll(filepath.Join(bundlePath, "root"))
		os.RemoveAll(bundlePath)
	})

	Describe("Exec", func() {
		var (
			processDir string
		)

		BeforeEach(func() {
			bundle = bundle.WithProcess(specs.Process{Args: []string{"/bin/sh", "-c", "sleep 9999"}, Cwd: "/"})
			processDir = filepath.Join(bundlePath, "processes", "abc")
			Expect(os.MkdirAll(processDir, 0777)).To(Succeed())
		})

		JustBeforeEach(func() {
			pipeR, pipeW, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
			cmd.ExtraFiles = []*os.File{pipeW, devNull(), devNull()}

			_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			buff := make([]byte, 1)
			_, err = pipeR.Read(buff)
			Expect(err).NotTo(HaveOccurred())
			Expect(buff[0]).To(BeEquivalentTo(0))
		})

		It("should return the exit code of the container process", func() {
			processSpec, err := json.Marshal(&specs.Process{
				Args: []string{"/bin/sh", "-c", "exit 24"},
				Cwd:  "/",
			})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
			cmd.Stdin = bytes.NewReader(processSpec)
			cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(24))
		})

		It("should not destroy the container when the exec process exits", func() {
			processSpec, err := json.Marshal(&specs.Process{
				Args: []string{"/bin/sh", "-c", "exit 24"},
				Cwd:  "/",
			})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
			cmd.Stdin = bytes.NewReader(processSpec)
			cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(24))

			Consistently(func() *gexec.Session {
				sess, err := gexec.Start(exec.Command("runc", "state", filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				return sess.Wait()
			}).Should(gexec.Exit(0))
		})

		Context("using named pipes for stdin/out/err", func() {
			var stdinPipe, stdoutPipe, stderrPipe string

			BeforeEach(func() {
				stdoutPipe = filepath.Join(processDir, "stdout")
				Expect(syscall.Mkfifo(stdoutPipe, 0)).To(Succeed())

				stderrPipe = filepath.Join(processDir, "stderr")
				Expect(syscall.Mkfifo(stderrPipe, 0)).To(Succeed())

				stdinPipe = filepath.Join(processDir, "stdin")
				Expect(syscall.Mkfifo(stdinPipe, 0)).To(Succeed())
			})

			It("should write the container's output to the named pipes inside the process dir", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "cat <&0"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(encSpec)
				_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				stdinP, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdoutP, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				stdinP.WriteString("hello")
				Expect(stdinP.Close()).To(Succeed())

				stdout := make([]byte, len("hello"))
				_, err = stdoutP.Read(stdout)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(stdout)).To(Equal("hello"))
			})

			It("should remove the process directory when it exits", func() {
				spec := specs.Process{
					Args: []string{"/bin/sh", "-c", "cat <&0"},
					Cwd:  "/",
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.Stdin = bytes.NewReader(encSpec)
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

				_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				stdinP, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				Eventually(processDir).Should(BeADirectory())
				Consistently(processDir).Should(BeADirectory())

				Expect(stdinP.Close()).To(Succeed()) // close stdin so process exits

				Eventually(processDir).ShouldNot(BeADirectory())
			})
		})

		Context("requesting a TTY", func() {
			var stdoutPipe, stderrPipe, stdinPipe string

			BeforeEach(func() {
				stdinPipe = filepath.Join(processDir, "stdin")
				Expect(syscall.Mkfifo(stdinPipe, 0)).To(Succeed())

				stdoutPipe = filepath.Join(processDir, "stdout")
				Expect(syscall.Mkfifo(stdoutPipe, 0)).To(Succeed())

				stderrPipe = filepath.Join(processDir, "stderr")
				Expect(syscall.Mkfifo(stderrPipe, 0)).To(Succeed())
			})

			It("should connect the process to a TTY", func() {
				spec := specs.Process{
					Args:     []string{"/bin/sh", "-c", `test -t 1`},
					Cwd:      "/",
					Terminal: true,
				}

				encSpec, err := json.Marshal(spec)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
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

				cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}
				cmd.Stdin = bytes.NewReader(encSpec)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				stdin, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				stdout, err := os.Open(stdoutPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Open(stderrPipe)
				Expect(err).NotTo(HaveOccurred())

				_, err = stdin.WriteString("banana\n")
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				data, err := ioutil.ReadAll(stdout)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("x=banana"))
			})

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
						for i in $(seq 10); do
						  sleep 1
						  stty -a
							echo ------------------------------
						done
					`,
						},
						Cwd:      "/",
						Terminal: true,
					}

					winszR, winszW, err := os.Pipe()
					Expect(err).NotTo(HaveOccurred())
					defer winszW.Close()

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), winszR}
					cmd.Stdin = bytes.NewReader(encSpec)

					json.NewEncoder(winszW).Encode(dadoo.TtySize{
						Rows: 17,
						Cols: 13,
					})

					_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					winszR.Close()

					_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					stdout, err := os.Open(stdoutPipe)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.Open(stderrPipe)
					Expect(err).NotTo(HaveOccurred())

					time.Sleep(5 * time.Second)
					data, err := ioutil.ReadAll(stdout)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(ContainSubstring("rows 17; columns 13;"))
				})

				It("should update window size", func() {
					spec := specs.Process{
						Args: []string{
							"/bin/sh",
							"-c",
							`
						trap "stty -a" SIGWINCH

						echo hello
						# continuously block so that the trap can keep firing
						for i in $(seq 10); do
						  sleep 1&
						  wait
						done
					`,
						},
						Cwd:      "/",
						Terminal: true,
					}

					winszR, winszW, err := os.Pipe()
					Expect(err).NotTo(HaveOccurred())
					defer winszW.Close()

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), winszR}
					cmd.Stdin = bytes.NewReader(encSpec)

					json.NewEncoder(winszW).Encode(dadoo.TtySize{
						Rows: 17,
						Cols: 13,
					})

					_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					winszR.Close()

					_, err = os.OpenFile(stdinPipe, os.O_WRONLY, 0600)
					Expect(err).NotTo(HaveOccurred())

					stdout, err := os.Open(stdoutPipe)
					Expect(err).NotTo(HaveOccurred())

					_, err = os.Open(stderrPipe)
					Expect(err).NotTo(HaveOccurred())

					buf := make([]byte, len("hello"))
					stdout.Read(buf)
					Expect(string(buf)).To(Equal("hello"))
					json.NewEncoder(winszW).Encode(dadoo.TtySize{
						Rows: 53,
						Cols: 60,
					})
					data, err := ioutil.ReadAll(stdout)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(ContainSubstring("rows 53; columns 60;"))
				})
			})
		})
	})

	Describe("Run", func() {
		It("should return the exit code of the container process", func() {
			cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
			cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(12))
		})

		It("should delete the container state correctly when it exits", func() {
			sess, err := gexec.Start(exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit())

			state, err := gexec.Start(exec.Command("runc", "state", filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(state).Should(gexec.Exit(1))
		})

		Describe("returning runc's exit code on fd3", func() {
			var pipeR, pipeW *os.File

			BeforeEach(func() {
				var err error
				pipeR, pipeW, err = os.Pipe()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when launching succeeds", func() {
				It("should return 0 on fd3", func() {
					cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{
						pipeW,
						devNull(),
						devNull(),
					}

					_, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					fd3 := make(chan byte)
					go func() {
						b := make([]byte, 1)
						pipeR.Read(b)

						fd3 <- b[0]
					}()

					Eventually(fd3).Should(Receive(BeEquivalentTo(0)))
				})

				Context("when running a long-running command", func() {
					BeforeEach(func() {
						bundle = bundle.WithProcess(specs.Process{
							Args: []string{
								"/bin/sh", "-c", "sleep 60",
							},
							Cwd: "/",
						})
					})

					It("should be able to be watched by WaitWatcher", func() {
						cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
						cmd.ExtraFiles = []*os.File{
							pipeW,
							devNull(),
							devNull(),
						}

						sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						fd3 := make(chan byte)
						go func() {
							b := make([]byte, 1)
							pipeR.Read(b)

							fd3 <- b[0]
						}()
						Eventually(fd3).Should(Receive(BeEquivalentTo(0)))

						ww := &dadoo.WaitWatcher{}
						ch, err := ww.Wait(filepath.Join(bundlePath, "exit.sock"))
						Expect(err).NotTo(HaveOccurred())
						Consistently(ch).ShouldNot(BeClosed())

						killCmd := exec.Command("runc", "kill", filepath.Base(bundlePath), "KILL")
						killCmd.Dir = bundlePath
						killSess, err := gexec.Start(killCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						Eventually(killSess).Should(gexec.Exit(0))

						Eventually(ch).Should(BeClosed())
						Expect(sess).NotTo(gexec.Exit(0))
					})
				})
			})

			Context("when launching fails", func() {
				BeforeEach(func() {
					bundle = bundle.WithRootFS("/path/to/nothing/at/all/potato")
				})

				It("should return runc's exit status on fd3", func() {
					cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{
						pipeW,
						devNull(),
						devNull(),
					}

					_, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					fd3 := make(chan byte)
					go func() {
						b := make([]byte, 1)
						pipeR.Read(b)

						fd3 <- b[0]
					}()

					Eventually(fd3).Should(Receive(BeEquivalentTo(1)))
				})
			})

			It("it exits 2 and writes an error to fd3 if runc start fails", func() {
				cmd := exec.Command(dadooBinPath, "run", "some-binary-that-doesnt-exist", bundlePath, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{
					pipeW,
					devNull(),
					devNull(),
				}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(2))

				fd3 := make(chan byte)
				go func() {
					b := make([]byte, 1)
					pipeR.Read(b)

					fd3 <- b[0]
				}()

				Eventually(fd3).Should(Receive(BeEquivalentTo(2)))
			})
		})
	})
})

func mustOpen(path string) *os.File {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

func setupCgroups() error {
	logger := lagertest.NewTestLogger("test")
	runner := linux_command_runner.New()

	starter := rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), path.Join(os.TempDir(), fmt.Sprintf("cgroups-%d", GinkgoParallelNode())), runner)

	return starter.Start()
}

func devNull() *os.File {
	f, err := os.OpenFile("/dev/null", os.O_APPEND, 0700)
	Expect(err).NotTo(HaveOccurred())
	return f
}
