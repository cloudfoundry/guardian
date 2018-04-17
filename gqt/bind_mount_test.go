package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bind mount", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
		srcPath   string
		dstPath   string
	)

	Describe("When source is a file", func() {
		BeforeEach(func() {
			tempFile, err := ioutil.TempFile("/opt", "bindmount-srcpath-file-")
			Expect(err).NotTo(HaveOccurred())
			defer tempFile.Close()
			srcPath = tempFile.Name()
			Expect(os.Chmod(srcPath, 0777)).To(Succeed())
			dstPath = "/home/alice/afile"
		})

		JustBeforeEach(func() {
			var err error
			client = runner.Start(config)
			container, err = client.Create(
				garden.ContainerSpec{
					BindMounts: []garden.BindMount{{
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

		It("allows all users to read files", func() {
			readProcess := containerReadFile(container, "/home/alice", "afile", "alice")
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

			It("allows all users to read files", func() {
				readProcess := containerReadFile(container, "/home/alice", "afile", "alice")
				Expect(readProcess.Wait()).To(Equal(0))
			})
		})
	})

	Describe("When source is a dir", func() {
		var (
			privilegedContainer bool
			tmpfsPath           string
			useSymlink          bool
			bindMountMode       garden.BindMountMode   // bm: RO or RW
			bindMountOrigin     garden.BindMountOrigin // bm: Container or Host
			mountOptions        []string

			// pre-existing file for permissions testing
			testFileName string
		)

		BeforeEach(func() {
			privilegedContainer = false
			container = nil
			tmpfsPath = ""
			srcPath = ""
			dstPath = ""
			bindMountMode = garden.BindMountModeRO
			bindMountOrigin = garden.BindMountOriginHost
			testFileName = ""
			mountOptions = []string{"--bind"}
			bindMountOrigin = garden.BindMountOriginHost
		})

		JustBeforeEach(func() {
			srcPath, tmpfsPath, testFileName = createTestHostDirAndTestFile(mountOptions, useSymlink)
			client = runner.Start(config)

			var err error
			container, err = client.Create(
				garden.ContainerSpec{
					Privileged: privilegedContainer,
					BindMounts: []garden.BindMount{{
						SrcPath: srcPath,
						DstPath: dstPath,
						Mode:    bindMountMode,
						Origin:  bindMountOrigin,
					}},
					Network: fmt.Sprintf("10.0.%d.0/24", GinkgoParallelNode()),
				})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.DestroyAndStop()).To(Succeed())

			unmount(srcPath)
			Expect(os.RemoveAll(srcPath)).To(Succeed())

			if useSymlink {
				unmount(tmpfsPath)
				Expect(os.RemoveAll(tmpfsPath)).To(Succeed())
			}
		})

		Context("which is read-only", func() {
			BeforeEach(func() {
				bindMountMode = garden.BindMountModeRO
				dstPath = "/home/alice/readonly"
			})

			Context("and with privileged=true", func() {
				BeforeEach(func() {
					privilegedContainer = true
				})

				It("allows all users to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "alice")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("does not allow non-root users to write files", func() {
					writeProcess := writeFile(container, dstPath, "alice")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				It("allows root to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "root")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("does not allow root to write files", func() {
					writeProcess := writeFile(container, dstPath, "root")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				Describe("nested-mounts", func() {
					var nestedMountpoint string

					JustBeforeEach(func() {
						nestedMountpoint = createMountPointUnder(srcPath)
					})

					AfterEach(func() {
						unmount(nestedMountpoint)
					})

					It("allows all users to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows non-root to write to nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "alice")
						Expect(writeProcess.Wait()).To(Equal(0))
					})

					It("allows root to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows root to write to from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "root")
						Expect(writeProcess.Wait()).To(Equal(0))
					})
				})
			})

			Context("and with privileged=false", func() {
				BeforeEach(func() {
					privilegedContainer = false
				})

				It("allows all users to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "alice")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("allows non-root users to write files", func() {
					writeProcess := writeFile(container, dstPath, "alice")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				It("allows root to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "root")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("does not allow root to write files", func() {
					writeProcess := writeFile(container, dstPath, "root")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				Describe("nested-mounts", func() {
					var nestedMountpoint string

					JustBeforeEach(func() {
						nestedMountpoint = createMountPointUnder(srcPath)
					})

					AfterEach(func() {
						unmount(nestedMountpoint)
					})

					It("allows all users to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows non-root to write to nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "alice")
						Expect(writeProcess.Wait()).To(Equal(0))
					})

					It("allows root to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows root to write to from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "root")
						Expect(writeProcess.Wait()).To(Equal(0))
					})
				})
			})

			Context("when mount directory is a mountpoint with an extra option", func() {
				BeforeEach(func() {
					mountOptions = []string{"-t", "tmpfs", "-o", "noexec"}
				})

				It("should suceed", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "root")
					Expect(readProcess.Wait()).To(Equal(0))
				})
			})
		})

		Context("which is read-write", func() {
			BeforeEach(func() {
				bindMountMode = garden.BindMountModeRW
				dstPath = "/home/alice/readwrite"
			})

			Context("and with privileged=true", func() {
				BeforeEach(func() {
					privilegedContainer = true
				})

				It("allows all users to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "alice")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("does not allow non-root users to write files (since the mounted directory is owned by host-root)", func() {
					writeProcess := writeFile(container, dstPath, "alice")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				It("allows root to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "root")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				It("allows root to write files (as container and host root are the same)", func() {
					writeProcess := writeFile(container, dstPath, "root")
					Expect(writeProcess.Wait()).To(Equal(0))
				})

				Describe("nested-mounts", func() {
					var nestedMountpoint string

					JustBeforeEach(func() {
						nestedMountpoint = createMountPointUnder(srcPath)
					})

					AfterEach(func() {
						unmount(nestedMountpoint)
					})

					It("allows all users to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows non-root to write to nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "alice")
						Expect(writeProcess.Wait()).To(Equal(0))
					})

					It("allows root to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows root to write to from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "root")
						Expect(writeProcess.Wait()).To(Equal(0))
					})
				})
			})

			Context("and with privileged=false", func() {
				BeforeEach(func() {
					privilegedContainer = false
				})

				It("allows all users to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "alice")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				// the mounted directory is owned by host-root, so alice shouldnt be able to write
				It("does not allow non-root users to write files", func() {
					writeProcess := writeFile(container, dstPath, "alice")
					Expect(writeProcess.Wait()).ToNot(Equal(0))
				})

				It("allows root to read files", func() {
					readProcess := containerReadFile(container, dstPath, testFileName, "root")
					Expect(readProcess.Wait()).To(Equal(0))
				})

				// container and host root are not the same, and the mounted directory is
				// owned by host-root, so writes should fail.
				It("does not allow root to write files", func() {
					writeProcess := writeFile(container, dstPath, "root")
					Expect(writeProcess.Wait()).NotTo(Equal(0))
				})

				Describe("nested-mounts", func() {
					var nestedMountpoint string

					JustBeforeEach(func() {
						nestedMountpoint = createMountPointUnder(srcPath)
					})

					AfterEach(func() {
						unmount(nestedMountpoint)
					})

					It("allows all users to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows non-root to write to nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "alice")
						Expect(writeProcess.Wait()).To(Equal(0))
					})

					It("allows root to read from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						readProcess := containerReadFile(container, nestedPath, "nested-file", "alice")
						Expect(readProcess.Wait()).To(Equal(0))
					})

					It("allows root to write to from nested bind mounts", func() {
						nestedPath := filepath.Join(dstPath, "nested-bind")
						writeProcess := writeFile(container, nestedPath, "root")
						Expect(writeProcess.Wait()).To(Equal(0))
					})
				})
			})
		})

		Context("when the source is a symlink", func() {
			BeforeEach(func() {
				useSymlink = true
				dstPath = "/home/alice/symlinked"
			})

			It("allows all users to read files", func() {
				readProcess := containerReadFile(container, dstPath, testFileName, "alice")
				Expect(readProcess.Wait()).To(Equal(0))
			})
		})
	})
})

func createTestHostDirAndTestFile(mountOptions []string, useSymlink bool) (string, string, string) {
	tstHostDir, err := ioutil.TempDir("", "bind-mount-test-dir")
	Expect(err).ToNot(HaveOccurred())
	err = os.Chown(tstHostDir, 0, 0)
	Expect(err).ToNot(HaveOccurred())
	err = os.Chmod(tstHostDir, 0755)
	Expect(err).ToNot(HaveOccurred())

	var cmd *exec.Cmd
	cmd = exec.Command("mount", append(mountOptions, tstHostDir, tstHostDir)...)
	Expect(cmd.Run()).To(Succeed())

	mountCommandOutputBytes, err := exec.Command("mount").CombinedOutput()
	mountCommandOutput := string(mountCommandOutputBytes)
	Expect(err).ToNot(HaveOccurred())
	Expect(mountCommandOutput).To(ContainSubstring(tstHostDir), fmt.Sprintf("%s appears not to be mounted; 'mount' command output:\n%s", tstHostDir, mountCommandOutput))

	procSelfMountInfoBytes, err := ioutil.ReadFile("/proc/self/mountinfo")
	procSelfMountInfo := string(procSelfMountInfoBytes)
	Expect(err).ToNot(HaveOccurred())
	Expect(procSelfMountInfo).To(ContainSubstring(tstHostDir), fmt.Sprintf("%s appears not to be visible in the process mount namespace; '/proc/self/mountinfo' content:\n%s", tstHostDir, procSelfMountInfo))
	fmt.Fprintf(GinkgoWriter, "/proc/self/mountinfo content:\n%s", procSelfMountInfo)

	cmd = exec.Command("mount", "--make-shared", tstHostDir)
	mountMakeSharedCommandOutputBytes, err := cmd.CombinedOutput()
	debugMsg := fmt.Sprintf("Command: mount --make-share %s\n%s\nOutput of `mount` command:\n%v",
		tstHostDir, mountCommandOutput, string(mountMakeSharedCommandOutputBytes))
	Expect(err).ToNot(HaveOccurred(), debugMsg)

	fileName := fmt.Sprintf("bind-mount-%d-test-file", GinkgoParallelNode())
	file, err := os.OpenFile(filepath.Join(tstHostDir, fileName), os.O_CREATE|os.O_RDWR, 0777)
	Expect(err).ToNot(HaveOccurred())
	Expect(file.Close()).ToNot(HaveOccurred())

	if useSymlink {
		tmpfsPath, err := ioutil.TempDir("/opt", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(tmpfsPath, os.ModePerm)).To(Succeed())
		Expect(exec.Command("mount", "-t", "tmpfs", "-o", "size=1m", "tmpfs", tmpfsPath).Run()).To(Succeed())

		symlinkPath := filepath.Join(tmpfsPath, "symlink-to-"+filepath.Base(tstHostDir))
		Expect(os.Symlink(tstHostDir, symlinkPath)).To(Succeed())
		return symlinkPath, tmpfsPath, fileName
	}

	return tstHostDir, "", fileName
}

func createMountPointUnder(srcPath string) string {
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

func containerReadFile(container garden.Container, dstPath, fileName, user string) garden.Process {
	filePath := filepath.Join(dstPath, fileName)

	process, err := container.Run(garden.ProcessSpec{
		Path: "cat",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{})
	Expect(err).ToNot(HaveOccurred())

	return process
}

func writeFile(container garden.Container, dstPath, user string) garden.Process {
	// try to write a new file
	filePath := filepath.Join(dstPath, "checkFileAccess-file")

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
