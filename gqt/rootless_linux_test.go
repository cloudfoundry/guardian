package gqt_test

import (
	"io"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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

	var idToStr = func(id uint32) string {
		return strconv.FormatUint(uint64(id), 10)
	}

	BeforeEach(func() {
		tmpDir := filepath.Join(
			os.TempDir(),
			fmt.Sprintf("test-garden-%s", config.Tag),
		)
		setupArgs := []string{"setup",
			"--tag", config.Tag,
			"--rootless-uid", idToStr(unprivilegedUID),
			"--rootless-gid", idToStr(unprivilegedGID)}

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

		runcRootDir, err = ioutil.TempDir(tmpDir, "runcRootDir")
		Expect(err).NotTo(HaveOccurred())
		imagePath, err = ioutil.TempDir(tmpDir, "rootlessImagePath")
		Expect(err).NotTo(HaveOccurred())

		unprivilegedUser := &syscall.Credential{Uid: unprivilegedUID, Gid: unprivilegedGID}

		Expect(exec.Command("cp", "-r", defaultTestRootFS, imagePath).Run()).To(Succeed())
		Expect(exec.Command("chown", "-R", fmt.Sprintf("%d:%d", unprivilegedUID, unprivilegedGID), runcRootDir).Run()).To(Succeed())
		Expect(exec.Command("chown", "-R", fmt.Sprintf("%d:%d", unprivilegedUID, unprivilegedGID), imagePath).Run()).To(Succeed())
		// The 'alice' user in the GARDEN_TEST_ROOTFS has a UID of 1000
		// The tests below use a uid range of 100000 -> 165536
		// 100000 + (1000 - 1) = 100999 (-1 because we map from 0)
		Expect(exec.Command("chown", "-R", "100999:100999", filepath.Join(imagePath, "rootfs", "home", "alice")).Run()).To(Succeed())

		config.ImagePluginBin = binaries.ImagePlugin
		config.NetworkPluginBin = "/bin/true"
		config.User = unprivilegedUser
		config.SkipSetup = boolptr(true)
		config.UIDMapStart = uint32ptr(100000)
		config.UIDMapLength = uint32ptr(65536)
		config.GIDMapStart = uint32ptr(100000)
		config.GIDMapLength = uint32ptr(65536)
		config.RuncRoot = runcRootDir
		config.ImagePluginExtraArgs = []string{"\"--rootfs-path\"", filepath.Join(imagePath, "rootfs")}

		client = runner.Start(config)
	})

	AfterEach(func() {
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

		// Runc validates that no mount points in rootless containers have non-zero
		// uid or gid. When we want to enable TTYs in rootless containers we will
		// probably have to address this, but for now we just set the gid to 0
		// instead of 5 (5 being the arbitrary gid we select as the owner of
		// /dev/pts).
		// In the container, this mount point appears to be owned by the
		// unprivileged host gid, rather than 0.
		It("should have devpts mounted with gid=unprivilegedGID", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			stdout := gbytes.NewBuffer()

			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "cat",
				Args: []string{"/proc/mounts"},
			}, garden.ProcessIO{
				Stdout: io.MultiWriter(stdout, GinkgoWriter),
				Stderr: GinkgoWriter,
			})
			Expect(err).ToNot(HaveOccurred())

			exitCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(stdout).To(gbytes.Say("devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=%d,mode=620,ptmxmode=666", unprivilegedGID))
		})

		Context("when the rootfs contains a read-only resolv.conf", func() {
			BeforeEach(func() {
				Expect(os.Chmod(filepath.Join(imagePath, "rootfs", "etc", "resolv.conf"), 0444)).To(Succeed())
			})

			It("succeeds anyway", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when we setup limits", func() {
			var cgroupPath, cgroupType string
			var container garden.Container
			JustBeforeEach(func() {
				currentCgroup, err := exec.Command("sh", "-c", "cat /proc/self/cgroup | head -1 | awk -F ':' '{print $3}'").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				cgroupName := strings.TrimSpace(string(currentCgroup))

				cgroupPath = fmt.Sprintf("%s/cgroups-%s/%s%s/garden-%s/%s",
					client.TmpDir, config.Tag, cgroupType, cgroupName, config.Tag, container.Handle())
			})

			BeforeEach(func() {
				var err error
				container, err = client.Create(garden.ContainerSpec{
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{
							LimitInBytes: 64 * 1024 * 1024,
						},
						Pid: garden.PidLimits{
							Max: 100,
						},
						CPU: garden.CPULimits{
							LimitInShares: 512,
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("the memory cgroup", func() {
				BeforeEach(func() {
					cgroupType = "memory"
				})

				It("creates container with the specified memory limit", func() {
					Expect(cgroupPath).To(BeADirectory())
					memLimitBytes, err := ioutil.ReadFile(filepath.Join(cgroupPath, "memory.limit_in_bytes"))
					Expect(err).NotTo(HaveOccurred())
					memLimit := strings.TrimSpace(string(memLimitBytes))
					Expect(memLimit).To(Equal("67108864"))
				})
			})

			Describe("the pids cgroup", func() {
				BeforeEach(func() {
					cgroupType = "pids"
				})

				It("creates container with the specified pid limit", func() {
					Expect(cgroupPath).To(BeADirectory())
					pidsMaxInBytes, err := ioutil.ReadFile(filepath.Join(cgroupPath, "pids.max"))
					Expect(err).NotTo(HaveOccurred())
					pidsMax := strings.TrimSpace(string(pidsMaxInBytes))
					Expect(pidsMax).To(Equal("100"))
				})
			})

			Describe("the cpu cgroup", func() {
				BeforeEach(func() {
					cgroupType = "cpu"
				})

				It("creates container with the specified cpu limits", func() {
					Expect(cgroupPath).To(BeADirectory())
					sharesBytes, err := ioutil.ReadFile(filepath.Join(cgroupPath, "cpu.shares"))
					Expect(err).NotTo(HaveOccurred())
					shares := strings.TrimSpace(string(sharesBytes))
					Expect(shares).To(Equal("512"))
				})
			})
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
