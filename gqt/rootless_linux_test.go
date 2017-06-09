package gqt_test

import (
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("rootless containers", func() {
	var (
		client      *runner.RunningGarden
		runcRootDir string
		imagePath   string
	)

	BeforeEach(func() {
		tmpDir := filepath.Join(
			os.TempDir(),
			fmt.Sprintf("test-garden-%d", GinkgoParallelNode()),
		)
		tag := fmt.Sprintf("%d", GinkgoParallelNode())
		setupArgs := []string{"setup", "--tag", tag}
		cmd := exec.Command(binaries.Gdn, setupArgs...)
		cmd.Env = append(
			[]string{
				fmt.Sprintf("TMPDIR=%s", tmpDir),
				fmt.Sprintf("TEMP=%s", tmpDir),
				fmt.Sprintf("TMP=%s", tmpDir),
			},
			os.Environ()...,
		)

		var err error

		setupProcess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))

		runcRootDir, err = ioutil.TempDir("", "runcRootDir")
		Expect(err).NotTo(HaveOccurred())
		imagePath, err = ioutil.TempDir("", "rootlessImagePath")
		Expect(err).NotTo(HaveOccurred())

		unprivilegedUser := &syscall.Credential{Uid: unprivilegedUID, Gid: unprivilegedGID}

		Expect(exec.Command("cp", "-r", os.Getenv("GARDEN_TEST_ROOTFS"), imagePath).Run()).To(Succeed())
		Expect(exec.Command("chown", "-R", fmt.Sprintf("%d:%d", unprivilegedUID, unprivilegedGID), runcRootDir).Run()).To(Succeed())
		Expect(exec.Command("chown", "-R", fmt.Sprintf("%d:%d", unprivilegedUID, unprivilegedGID), imagePath).Run()).To(Succeed())
		// The 'alice' user in the GARDEN_TEST_ROOTFS has a UID of 1000
		// The tests below use a uid range of 100000 -> 165536
		// 100000 + (1000 - 1) = 100999 (-1 because we map from 0)
		Expect(exec.Command("chown", "-R", "100999:100999", filepath.Join(imagePath, "rootfs", "home", "alice")).Run()).To(Succeed())

		client = startGardenAsUser(
			unprivilegedUser,
			"--skip-setup",
			"--image-plugin", binaries.ImagePlugin,
			"--image-plugin-extra-arg", "\"--rootfs-path\"",
			"--image-plugin-extra-arg", filepath.Join(imagePath, "rootfs"),
			"--uid-map-start", "100000", // /etc/subuid entry baked into the cfgarden/garden-ci-ubuntu image
			"--uid-map-length", "65536",
			"--gid-map-start", "100000",
			"--gid-map-length", "65536",
			"--network-plugin", "/bin/true",
			"--runc-root", runcRootDir,
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
		Expect(os.RemoveAll(runcRootDir)).To(Succeed())
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("the server process", func() {
		It("can run consistently as a non-root user", func() {
			out, err := exec.Command("ps", "-U", fmt.Sprintf("%d", unprivilegedUID)).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "No process of unprivileged user was found")
			Expect(out).To(ContainSubstring(fmt.Sprintf("%d", client.Pid)))

			Consistently(func() error {
				return exec.Command("ps", "-p", strconv.Itoa(client.Pid)).Run()
			}).Should(Succeed())
		})
	})

	Describe("creating a container", func() {
		It("succeeds", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("maps uids and gids other than guardian's user", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			var stdout bytes.Buffer
			process, err := container.Run(garden.ProcessSpec{
				Path: "stat",
				Args: []string{"-c", "%U:%G", "/home/alice"},
			}, garden.ProcessIO{
				Stdout: io.MultiWriter(&stdout, GinkgoWriter),
				Stderr: GinkgoWriter,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(process.Wait()).To(Equal(0))

			Expect(stdout.String()).To(ContainSubstring("alice:alice"))
		})
	})

	Describe("running a process in a container", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the process's exit code", func() {
			processSpec := garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "exit 13"},
			}

			process, err := container.Run(processSpec, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())

			Expect(exitCode).To(Equal(13))
		})

		It("streams input to the process's stdin", func() {
			processSpec := garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "cat"},
			}
			stdin := bytes.NewBufferString("rootlessStdinFTW")
			stdout := gbytes.NewBuffer()
			processIO := garden.ProcessIO{Stdin: stdin, Stdout: stdout}

			process, err := container.Run(processSpec, processIO)
			Expect(err).ToNot(HaveOccurred())

			Eventually(stdout).Should(gbytes.Say("rootlessStdinFTW"))
			Expect(process.Wait()).To(Equal(0))
		})

		It("streams output from the process's stdout", func() {
			processSpec := garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "echo rootlessStdoutFTW"},
			}
			stdout := gbytes.NewBuffer()
			processIO := garden.ProcessIO{Stdout: stdout}

			process, err := container.Run(processSpec, processIO)
			Expect(err).ToNot(HaveOccurred())

			Eventually(stdout).Should(gbytes.Say("rootlessStdoutFTW"))
			Expect(process.Wait()).To(Equal(0))
		})

		It("streams output from the process's stderr", func() {
			processSpec := garden.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "echo rootlessStderrFTW 1>&2"},
			}
			stderr := gbytes.NewBuffer()
			processIO := garden.ProcessIO{Stderr: stderr}

			process, err := container.Run(processSpec, processIO)
			Expect(err).ToNot(HaveOccurred())

			Eventually(stderr).Should(gbytes.Say("rootlessStderrFTW"))
			Expect(process.Wait()).To(Equal(0))
		})
	})
})
