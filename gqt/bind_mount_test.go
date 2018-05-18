package gqt_test

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Bind mount", func() {
	var (
		client       *runner.RunningGarden
		container    garden.Container
		srcPath      string
		dstPath      string
		testFileName string
		mountMode    garden.BindMountMode
		privileged   bool
	)

	BeforeEach(func() {
		privileged = false
		container = nil
		srcPath = ""
		dstPath = ""
		mountMode = garden.BindMountModeRO
		testFileName = ""
	})

	JustBeforeEach(func() {
		client = runner.Start(config)

		var err error
		container, err = client.Create(
			garden.ContainerSpec{
				Privileged: privileged,
				BindMounts: []garden.BindMount{{
					Mode:    mountMode,
					SrcPath: srcPath,
					DstPath: dstPath,
				}},
				Network: fmt.Sprintf("10.0.%d.0/24", GinkgoParallelNode()),
			})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(srcPath)).To(Succeed())
	})

	Describe("when source is a file", func() {
		BeforeEach(func() {
			tempFile, err := ioutil.TempFile("/opt", "bindmount-srcpath-file-")
			Expect(err).NotTo(HaveOccurred())
			defer tempFile.Close()
			srcPath = tempFile.Name()
			Expect(os.Chmod(srcPath, 0777)).To(Succeed())
			dstPath = "/home/alice/afile"
			mountMode = garden.BindMountModeRO
		})

		It("all users can read the file", func() {
			readProcess := userReadFile(container, "/home/alice", "afile", "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		Context("and the file is a bind mount", func() {
			BeforeEach(func() {
				var cmd *exec.Cmd
				cmd = exec.Command("mount", "--bind", srcPath, srcPath)
				Expect(cmd.Run()).To(Succeed())
			})

			AfterEach(func() {
				unmount(srcPath)
			})

			It("all users can read the file", func() {
				readProcess := userReadFile(container, "/home/alice", "afile", "alice")
				Expect(readProcess.Wait()).To(Equal(0))
			})
		})

		Context("and the mount option is RW", func() {
			BeforeEach(func() {
				mountMode = garden.BindMountModeRW
			})
			Context("and the container is privileged", func() {
				BeforeEach(func() {
					privileged = true
				})

				It("all users can write to the file", func() {
					readProcess := userWriteToFile(container, dstPath, "alice")
					Expect(readProcess.Wait()).To(Equal(0))
				})
			})
		})
	})

	Describe("when the source is a symlink", func() {
		var tmpfsPath string

		BeforeEach(func() {
			dstPath = "/home/alice/adir"
			srcPath, tmpfsPath, testFileName = createSymlinkSource()
			mountMode = garden.BindMountModeRO
		})

		AfterEach(func() {
			unmount(tmpfsPath)
			Expect(os.RemoveAll(tmpfsPath)).To(Succeed())
		})

		It("all users can read files", func() {
			readProcess := userReadFile(container, dstPath, testFileName, "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})
	})

	Describe("when source is a dir", func() {
		BeforeEach(func() {
			dstPath = "/home/alice/adir"
			srcPath = createTestHostDir()
			srcPath, testFileName = createTestFile(srcPath)
			mountMode = garden.BindMountModeRO
		})

		It("all users can read files", func() {
			readProcess := userReadFile(container, dstPath, testFileName, "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		Context("and the dir is a bind mount", func() {
			BeforeEach(func() {
				var cmd *exec.Cmd
				cmd = exec.Command("mount", "--bind", srcPath, srcPath)
				Expect(cmd.Run()).To(Succeed())
			})

			AfterEach(func() {
				unmount(srcPath)
			})

			It("all users can read files", func() {
				readProcess := userReadFile(container, dstPath, testFileName, "alice")
				Expect(readProcess.Wait()).To(Equal(0))
			})
		})
	})

	Describe("when the source is a mountpoint with extra options", func() {
		BeforeEach(func() {
			mountOptions := []string{"-t", "tmpfs", "-o", "noexec"}
			dstPath = "/home/alice/adir"
			srcPath = createTestHostDir()
			mountSourceDirToSelf(mountOptions, srcPath)
			srcPath, testFileName = createTestFile(srcPath)
			mountMode = garden.BindMountModeRO
		})

		AfterEach(func() {
			unmount(srcPath)
		})

		It("those extra options are not lost through remounting", func() {
			mountOpts := parseMountsFile(container)

			Expect(mountOpts).To(ContainSubstring("noexec"))
			Expect(mountOpts).To(ContainSubstring("ro"))
		})

		It("all users can read files", func() {
			readProcess := userReadFile(container, dstPath, testFileName, "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		It("no user can create files", func() {
			readProcess := userWriteFile(container, dstPath, "alice")
			Expect(readProcess.Wait()).To(Equal(1))
		})
	})

	Describe("when the mount is RO", func() {
		BeforeEach(func() {
			dstPath = "/home/alice/readonly"
			mountMode = garden.BindMountModeRO
			srcPath = createTestHostDir()
			srcPath, testFileName = createTestFile(srcPath)
		})

		It("all users can read files", func() {
			readProcess := userReadFile(container, dstPath, testFileName, "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		It("no user can create files", func() {
			readProcess := userWriteFile(container, dstPath, "alice")
			Expect(readProcess.Wait()).To(Equal(1))
		})
	})

	Describe("when the mount is RW", func() {
		BeforeEach(func() {
			dstPath = "/home/alice/readwrite"
			mountMode = garden.BindMountModeRW
			srcPath = createTestHostDir()
			srcPath, testFileName = createTestFile(srcPath)
		})

		It("all users can read files", func() {
			readProcess := userReadFile(container, dstPath, testFileName, "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		Context("when privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("root user can write files", func() {
				readProcess := userWriteFile(container, dstPath, "root")
				Expect(readProcess.Wait()).To(Equal(0))
			})

			It("non-root users cannot write files", func() {
				readProcess := userWriteFile(container, dstPath, "alice")
				Expect(readProcess.Wait()).To(Equal(1))
			})
		})

		Context("when unprivileged", func() {
			BeforeEach(func() {
				privileged = false
			})

			It("no user can write files", func() {
				readProcess := userWriteFile(container, dstPath, "root")
				Expect(readProcess.Wait()).To(Equal(1))
			})
		})
	})

	Describe("when the mount is nested", func() {
		var nestedMountpoint string

		BeforeEach(func() {
			mountOptions := []string{"--bind"}
			dstPath = "/home/alice/adir"
			mountMode = garden.BindMountModeRO
			srcPath = createTestHostDir()
			mountSourceDirToSelf(mountOptions, srcPath)
			srcPath, testFileName = createTestFile(srcPath)
		})

		JustBeforeEach(func() {
			nestedMountpoint = createRWMountPointUnder(srcPath)
		})

		AfterEach(func() {
			unmount(nestedMountpoint)
			unmount(srcPath)
		})

		It("all users can read from RW nested bind mounts", func() {
			nestedPath := filepath.Join(dstPath, "nested-bind")
			readProcess := userReadFile(container, nestedPath, "nested-file", "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})

		It("all users can write to RW nested bind mounts", func() {
			nestedPath := filepath.Join(dstPath, "nested-bind")
			readProcess := userReadFile(container, nestedPath, "nested-file", "alice")
			Expect(readProcess.Wait()).To(Equal(0))
		})
	})
})

func createTestHostDir() string {
	tstHostDir, err := ioutil.TempDir("", "bind-mount-test-dir")
	Expect(err).ToNot(HaveOccurred())
	err = os.Chown(tstHostDir, 0, 0)
	Expect(err).ToNot(HaveOccurred())
	err = os.Chmod(tstHostDir, 0755)
	Expect(err).ToNot(HaveOccurred())

	return tstHostDir
}

func createTestFile(tstHostDir string) (string, string) {
	fileName := fmt.Sprintf("bind-mount-%d-test-file", GinkgoParallelNode())
	file, err := os.OpenFile(filepath.Join(tstHostDir, fileName), os.O_CREATE|os.O_RDWR, 0777)
	Expect(err).ToNot(HaveOccurred())
	Expect(file.Close()).ToNot(HaveOccurred())

	return tstHostDir, fileName
}

func mountSourceDirToSelf(mountOptions []string, tstHostDir string) {
	var cmd *exec.Cmd
	cmd = exec.Command("mount", append(mountOptions, tstHostDir, tstHostDir)...)
	Expect(cmd.Run()).To(Succeed())

	cmd = exec.Command("mount", "--make-shared", tstHostDir)
	Expect(cmd.Run()).To(Succeed())
}

func createSymlinkSource() (string, string, string) {
	tstHostDir := createTestHostDir()
	tstHostDir, fileName := createTestFile(tstHostDir)
	tmpfsPath, err := ioutil.TempDir("/opt", "")
	Expect(err).NotTo(HaveOccurred())

	Expect(os.MkdirAll(tmpfsPath, os.ModePerm)).To(Succeed())
	Expect(exec.Command("mount", "-t", "tmpfs", "-o", "size=1m", "tmpfs", tmpfsPath).Run()).To(Succeed())

	symlinkPath := filepath.Join(tmpfsPath, "symlink-to-"+filepath.Base(tstHostDir))
	Expect(os.Symlink(tstHostDir, symlinkPath)).To(Succeed())
	return symlinkPath, tmpfsPath, fileName
}

func createRWMountPointUnder(srcPath string) string {
	nestedBindPath := filepath.Join(srcPath, "nested-bind")
	Expect(os.MkdirAll(nestedBindPath, os.FileMode(0755))).To(Succeed())

	cmd := exec.Command("mount", "-t", "tmpfs", "tmpfs", nestedBindPath)
	Expect(cmd.Run()).To(Succeed())

	file, err := os.OpenFile(filepath.Join(nestedBindPath, "nested-file"), os.O_CREATE|os.O_RDWR, 0777)
	Expect(err).ToNot(HaveOccurred())
	Expect(file.Close()).ToNot(HaveOccurred())

	return nestedBindPath
}

func unmount(mountpoint string) {
	cmd := exec.Command("umount", "-f", mountpoint)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Printf("Command: umount -f [%s]\n%v", mountpoint, string(output))
	}
	Expect(err).NotTo(HaveOccurred())
}

func userReadFile(container garden.Container, dstPath, fileName, user string) garden.Process {
	filePath := filepath.Join(dstPath, fileName)
	process, _ := containerReadFile(container, filePath, user)
	return process
}

func containerReadFile(container garden.Container, filePath, user string) (garden.Process, *gbytes.Buffer) {
	output := gbytes.NewBuffer()
	process, err := container.Run(garden.ProcessSpec{
		Path: "cat",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{
		Stdout: io.MultiWriter(GinkgoWriter, output),
		Stderr: io.MultiWriter(GinkgoWriter, output),
	})
	Expect(err).ToNot(HaveOccurred())

	return process, output
}

func userWriteFile(container garden.Container, dstPath, user string) garden.Process {
	filePath := filepath.Join(dstPath, "can-touch-this")

	process, err := container.Run(garden.ProcessSpec{
		Path: "touch",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{
		Stderr: GinkgoWriter,
		Stdout: GinkgoWriter,
	})
	Expect(err).ToNot(HaveOccurred())

	return process
}

func userWriteToFile(container garden.Container, filePath, user string) garden.Process {
	process, err := container.Run(garden.ProcessSpec{
		Path: "/bin/sh",
		Args: []string{"-c", fmt.Sprintf("echo hello >> %s", filePath)},
		User: user,
	}, garden.ProcessIO{
		Stderr: GinkgoWriter,
		Stdout: GinkgoWriter,
	})
	Expect(err).ToNot(HaveOccurred())

	return process
}

func parseMountsFile(container garden.Container) string {
	process, output := containerReadFile(container, "/proc/self/mounts", "root")
	Expect(process.Wait()).To(Equal(0))

	var mountOpts string
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		mountLine := scanner.Text()
		mountInfo := strings.Split(mountLine, " ")
		mountDest := mountInfo[1]
		if strings.Contains(mountDest, "alice") {
			mountOpts = mountInfo[3]
		}
	}

	return mountOpts
}
