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

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runtime-spec/specs-go"
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
			cmd := exec.Command("runc", "create", "--bundle", bundlePath, filepath.Base(bundlePath))
			Expect(cmd.Run()).To(Succeed())
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

		It("should write the exit code to a file named exitcode in the container dir", func() {
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
			cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(128 + 9))

			Eventually(filepath.Join(processDir, "exitcode")).Should(BeAnExistingFile())
			Expect(ioutil.ReadFile(filepath.Join(processDir, "exitcode"))).To(Equal([]byte("137")))
		})

		It("should open the exit pipe and close it when it exits", func() {
			stdinPipe := filepath.Join(processDir, "stdin")
			Expect(syscall.Mkfifo(stdinPipe, 0)).To(Succeed())

			exitPipe := filepath.Join(processDir, "exit")
			Expect(syscall.Mkfifo(exitPipe, 0)).To(Succeed())

			processSpec, err := json.Marshal(&specs.Process{
				Args: []string{"/bin/sh", "-c", "cat <&0"},
				Cwd:  "/",
			})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(dadooBinPath, "exec", "runc", processDir, filepath.Base(bundlePath))
			cmd.Stdin = bytes.NewReader(processSpec)
			cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null"), mustOpen("/dev/null")}

			_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			exitFifoCh := make(chan struct{})
			go func() {
				exitFifo, err := os.Open(filepath.Join(processDir, "exit"))
				Expect(err).NotTo(HaveOccurred())

				buf := make([]byte, 1)
				exitFifo.Read(buf)
				close(exitFifoCh)
			}()

			stdin, err := os.OpenFile(filepath.Join(processDir, "stdin"), os.O_WRONLY, 0600)
			Expect(err).NotTo(HaveOccurred())

			Consistently(exitFifoCh).ShouldNot(BeClosed())
			Expect(stdin.Close()).To(Succeed()) // should cause cat <&0 to complete
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
		})

		Context("requesting a TTY", func() {
			var stdoutPipe, stderrPipe, stdinPipe, winszPipe string

			BeforeEach(func() {
				stdinPipe = filepath.Join(processDir, "stdin")
				Expect(syscall.Mkfifo(stdinPipe, 0)).To(Succeed())

				stdoutPipe = filepath.Join(processDir, "stdout")
				Expect(syscall.Mkfifo(stdoutPipe, 0)).To(Succeed())

				stderrPipe = filepath.Join(processDir, "stderr")
				Expect(syscall.Mkfifo(stderrPipe, 0)).To(Succeed())

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

				_, err = os.OpenFile(winszPipe, os.O_WRONLY, 0600)
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
							"/bin/sh", "-c", `stty -a`,
						},
						Cwd:      "/",
						Terminal: true,
					}

					encSpec, err := json.Marshal(spec)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "-rows", "17", "-cols", "13", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null")}
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

					cmd := exec.Command(dadooBinPath, "-uid", "1", "-gid", "1", "-tty", "exec", "runc", processDir, filepath.Base(bundlePath))
					cmd.ExtraFiles = []*os.File{mustOpen("/dev/null"), mustOpen("/dev/null")}
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
