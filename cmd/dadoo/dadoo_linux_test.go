package main_test

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/specs/specs-go"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Dadoo", func() {
	var (
		bundlePath string
		bundle     *goci.Bndl
	)

	BeforeEach(func() {
		setupCgroups()

		var err error
		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cmd := exec.Command("runc", "spec")
		cmd.Dir = bundlePath
		Expect(cmd.Run()).To(Succeed())

		loader := &goci.BndlLoader{}
		bundle, err = loader.Load(bundlePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(path.Join(bundlePath, "root"), 0755)).To(Succeed())
		Expect(syscall.Mount(os.Getenv("GARDEN_TEST_ROOTFS"), filepath.Join(bundlePath, "root"), "", uintptr(syscall.MS_BIND), "")).To(Succeed())

		bundle = bundle.
			WithProcess(specs.Process{Args: []string{"/bin/sh", "-c", "exit 12"}, Cwd: "/"}).
			WithRootFS(path.Join(bundlePath, "root"))

		SetDefaultEventuallyTimeout(5 * time.Second)
	})

	JustBeforeEach(func() {
		Expect(bundle.Save(path.Join(bundlePath))).To(Succeed())
	})

	AfterEach(func() {
		Expect(syscall.Unmount(filepath.Join(bundlePath, "root"), syscall.MNT_DETACH)).To(Succeed())
	})

	It("should return the exit code of the container process", func() {
		sess, err := gexec.Start(exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess).Should(gexec.Exit(12))
	})

	It("should write logs to the requested file", func() {
		_, err := gexec.Start(exec.Command(dadooBinPath, "-log", path.Join(bundlePath, "foo.log"), "run", "runc", bundlePath, filepath.Base(bundlePath)), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(filepath.Join(bundlePath, "foo.log")).Should(BeAnExistingFile())
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
		})

		Context("when launching fails", func() {
			BeforeEach(func() {
				bundle = bundle.WithRootFS("/path/to/nothing/at/all/potato")
			})

			It("should return runc's exit status on fd3", func() {
				cmd := exec.Command(dadooBinPath, "run", "runc", bundlePath, filepath.Base(bundlePath))
				cmd.ExtraFiles = []*os.File{
					pipeW,
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

func mustOpen(path string) io.ReadCloser {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

func setupCgroups() error {
	logger := lagertest.NewTestLogger("test")
	runner := linux_command_runner.New()

	starter := rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), path.Join(os.TempDir(), "cgroups"), runner)

	return starter.Start()
}
