package unpacker_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("Tar unpacker", func() {
	var (
		tarUnpacker        *unpacker.TarUnpacker
		logger             lager.Logger
		baseImagePath      string
		stream             io.ReadWriteCloser
		targetPath         string
		storeDir           string
		storeDirFile       *os.File
		whiteoutDevicePath string
		filepathToTar      string
		deviceNumber       uint64
	)

	BeforeEach(func() {
		var err error

		storeDir, err = os.MkdirTemp("", "store-")
		Expect(err).NotTo(HaveOccurred())

		targetPath, err = os.MkdirTemp("", "target-")
		Expect(err).NotTo(HaveOccurred())

		baseImagePath, err = os.MkdirTemp("", "baseimage-")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test-store")

		whiteoutDevicePath = filepath.Join(storeDir, overlayxfs.WhiteoutDevice)
		deviceNumber = unix.Mkdev(123, 456)
		Expect(unix.Mknod(whiteoutDevicePath, syscall.S_IFCHR, int(deviceNumber))).To(Succeed())

		storeDirFile, err = os.Open(storeDir)
		Expect(err).NotTo(HaveOccurred())

		filepathToTar = "."
	})

	JustBeforeEach(func() {
		mappings := []groot.IDMappingSpec{
			{HostID: 1000, NamespaceID: 0, Size: 1},
			{HostID: 11, NamespaceID: 1, Size: 900},
			{HostID: 2001, NamespaceID: 1001, Size: 900},
		}
		tarUnpacker = unpacker.NewTarUnpacker(
			unpacker.NewOverlayWhiteoutHandler(storeDirFile),
			unpacker.NewIDTranslator(mappings, mappings),
		)

		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", baseImagePath, filepathToTar), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, 10*time.Second).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(storeDirFile.Close()).To(Succeed())
		Expect(os.RemoveAll(targetPath)).To(Succeed())
		Expect(os.RemoveAll(storeDir)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	Describe("regular files", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(path.Join(baseImagePath, "a_file"), []byte("hello-world"), 0o600)).To(Succeed())
		})

		It("creates regular files", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(targetPath, "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := os.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Describe("unpacked bytes count", func() {
			BeforeEach(func() {
				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "1mb")), "count=1", "bs=1M")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "3mb")), "count=3", "bs=1M")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "1k")), "count=1", "bs=1K")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("returns the total size that was unpacked", func() {
				totalUnpacked, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(totalUnpacked).To(Equal(base_image_puller.UnpackOutput{BytesWritten: 1024*1024 + 1024*1024*3 + 1024 + 11, OpaqueWhiteouts: []string{}}))
			})
		})

		Describe("when BaseDirectory is provided", func() {
			It("creates the files inside that directory", func() {
				Expect(os.MkdirAll(filepath.Join(targetPath, "hello/world"), 0o755)).To(Succeed())
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:        stream,
					TargetPath:    targetPath,
					BaseDirectory: "/hello/world",
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "/hello/world", "a_file")
				Expect(filePath).To(BeARegularFile())
			})
		})

		Describe("file ownership", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(filepath.Join(baseImagePath, "groot_file"), []byte{}, 0o755)).To(Succeed())
				Expect(os.Chown(filepath.Join(baseImagePath, "groot_file"), 1000, 1000)).To(Succeed())
			})

			It("preserves it", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "groot_file")
				Expect(filePath).To(BeARegularFile())
				stat, err := os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
			})

			Context("when there are files owned by multiple users", func() {
				BeforeEach(func() {
					Expect(os.WriteFile(filepath.Join(baseImagePath, "200_file"), []byte{}, 0o755)).To(Succeed())
					Expect(os.Chown(filepath.Join(baseImagePath, "200_file"), 200, 200)).To(Succeed())

					Expect(os.WriteFile(filepath.Join(baseImagePath, "1200_file"), []byte{}, 0o755)).To(Succeed())
					Expect(os.Chown(filepath.Join(baseImagePath, "1200_file"), 1200, 1200)).To(Succeed())
				})

				It("maps their ownership", func() {
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:     stream,
						TargetPath: targetPath,
					})
					Expect(err).NotTo(HaveOccurred())

					filePath := path.Join(targetPath, "a_file")
					Expect(filePath).To(BeARegularFile())
					stat, err := os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))

					filePath = path.Join(targetPath, "200_file")
					Expect(filePath).To(BeARegularFile())
					stat, err = os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10 + 200)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10 + 200)))

					filePath = path.Join(targetPath, "1200_file")
					Expect(filePath).To(BeARegularFile())
					stat, err = os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(2000 + 1200)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(2000 + 1200)))

					filePath = path.Join(targetPath, "groot_file")
					Expect(filePath).To(BeARegularFile())
					stat, err = os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
				})
			})
		})

		Context("when parent directories are not individual entries in the source tar", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(path.Join(baseImagePath, "parentdir1", "parentdir2"), 0o755)).To(Succeed())
				Expect(os.WriteFile(path.Join(baseImagePath, "parentdir1", "parentdir2", "a_file"), []byte("hello-world"), 0o600)).To(Succeed())

				// results in a tar of the format:
				// parentdir1/parentdir2/a_file
				filepathToTar = filepath.Join("parentdir1", "parentdir2", "a_file")
			})

			It("succeeds in creating regular files", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "parentdir1", "parentdir2", "a_file")
				Expect(filePath).To(BeARegularFile())

				contents, err := os.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal("hello-world"))
			})

			It("applies default permissions of root:root and 0755 to the parent dirs", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				parentDirPaths := []string{
					path.Join(targetPath, "parentdir1"),
					path.Join(targetPath, "parentdir1", "parentdir2"),
				}

				for _, parentDirPath := range parentDirPaths {
					Expect(parentDirPath).To(BeADirectory())

					info, err := os.Stat(parentDirPath)
					Expect(err).NotTo(HaveOccurred())

					uid := info.Sys().(*syscall.Stat_t).Uid
					gid := info.Sys().(*syscall.Stat_t).Gid

					Expect(info.Mode().String()).To(Equal("drwxr-xr-x"))
					Expect(uid).To(Equal(uint32(1000)), parentDirPath)
					Expect(gid).To(Equal(uint32(1000)), parentDirPath)
				}
			})
		})
	})

	Describe("directories", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(path.Join(baseImagePath, "subdir"), 0o700)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "subdir", "subdir2"), 0o777)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, "subdir", "subdir2", "another_file"), []byte("goodbye-world"), 0o600)).To(Succeed())
		})

		It("creates files in subdirectories", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(targetPath, "subdir", "subdir2", "another_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := os.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("goodbye-world"))
		})

		Context("directory ownership", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(filepath.Join(baseImagePath, "groot_dir"), 0o755)).To(Succeed())
				Expect(os.Chown(filepath.Join(baseImagePath, "groot_dir"), 1000, 1000)).To(Succeed())
			})

			It("preserves it", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "groot_dir")
				Expect(filePath).To(BeADirectory())
				stat, err := os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
			})

			Context("when there are direcotries owned by multiple users", func() {
				BeforeEach(func() {
					Expect(os.Mkdir(filepath.Join(baseImagePath, "200_dir"), 0o755)).To(Succeed())
					Expect(os.Chown(filepath.Join(baseImagePath, "200_dir"), 200, 200)).To(Succeed())

					Expect(os.Mkdir(filepath.Join(baseImagePath, "1200_dir"), 0o755)).To(Succeed())
					Expect(os.Chown(filepath.Join(baseImagePath, "1200_dir"), 1200, 1200)).To(Succeed())
				})

				It("maps their ownership", func() {
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:     stream,
						TargetPath: targetPath,
					})
					Expect(err).NotTo(HaveOccurred())

					filePath := path.Join(targetPath, "200_dir")
					Expect(filePath).To(BeADirectory())
					stat, err := os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10 + 200)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10 + 200)))

					filePath = path.Join(targetPath, "1200_dir")
					Expect(filePath).To(BeADirectory())
					stat, err = os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(2000 + 1200)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(2000 + 1200)))

					filePath = path.Join(targetPath, "groot_dir")
					Expect(filePath).To(BeADirectory())
					stat, err = os.Stat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
				})
			})
		})

		Context("when parent directories are not individual entries in the source tar", func() {
			BeforeEach(func() {
				// results in a tar of the format:
				// subdir/subdir2
				// subdir/subdir2/another_file
				filepathToTar = filepath.Join("subdir", "subdir2")
			})

			It("succeeds in creating directories", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "subdir", "subdir2")
				Expect(filePath).To(BeADirectory())
			})

			It("applies default permissions of root:root and 0755 to the parent dirs", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				parentDirPath := path.Join(targetPath, "subdir")
				Expect(parentDirPath).To(BeADirectory())

				info, err := os.Stat(parentDirPath)
				Expect(err).NotTo(HaveOccurred())

				uid := info.Sys().(*syscall.Stat_t).Uid
				gid := info.Sys().(*syscall.Stat_t).Gid

				Expect(info.Mode().String()).To(Equal("drwxr-xr-x"))
				Expect(uid).To(Equal(uint32(1000)))
				Expect(gid).To(Equal(uint32(1000)))
			})
		})

		Describe("when BaseDirectory is provided", func() {
			It("creates the files inside that directory", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:        stream,
					TargetPath:    targetPath,
					BaseDirectory: "base-dir",
				})
				Expect(err).NotTo(HaveOccurred())

				filePath := path.Join(targetPath, "base-dir", "subdir", "subdir2", "another_file")
				Expect(filePath).To(BeARegularFile())
				contents, err := os.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("goodbye-world"))
			})
		})
	})

	Describe("modification time", func() {
		var (
			fileModTime time.Time
			dirModTime  time.Time
		)

		BeforeEach(func() {
			location := time.FixedZone("foo", 0)

			fileModTime = time.Date(2014, 10, 14, 22, 8, 32, 0, location)
			filePath := path.Join(baseImagePath, "old-file")
			Expect(os.WriteFile(filePath, []byte("hello-world"), 0o600)).To(Succeed())
			Expect(os.Chtimes(filePath, time.Now(), fileModTime)).To(Succeed())

			dirModTime = time.Date(2014, 9, 3, 22, 8, 32, 0, location)
			dirPath := path.Join(baseImagePath, "old-dir")
			Expect(os.Mkdir(dirPath, 0o700)).To(Succeed())
			Expect(os.Chtimes(dirPath, time.Now(), dirModTime)).To(Succeed())
		})

		It("preserves the modtime for files", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			fi, err := os.Stat(path.Join(targetPath, "old-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(fileModTime.Unix()))
		})

		It("preserves the modtime for directories", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			fi, err := os.Stat(path.Join(targetPath, "old-dir"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(dirModTime.Unix()))
		})
	})

	Describe("permissions", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(path.Join(baseImagePath, "a_file"), []byte("hello-world"), 0o600)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "a_dir"), 0o700)).To(Succeed())

			// We have to chmod it because creat and mkdir syscalls take the umask into
			// account when applying the permissions. This means that only permissions
			// less permissive than the umask can be applied to files and directories.
			// By calling chmod we explicitly apply the permissions without being
			// subject to the umask.
			Expect(os.Chmod(path.Join(baseImagePath, "a_file"), 0o777)).To(Succeed())
			Expect(os.Chmod(path.Join(baseImagePath, "a_dir"), 0o777)).To(Succeed())
		})

		It("keeps file permissions", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(targetPath, "a_file")
			stat, err := os.Stat(filePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0o777)))
		})

		It("keeps directory permissions", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			dirPath := path.Join(targetPath, "a_dir")
			stat, err := os.Stat(dirPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0o777)))
		})
	})

	Context("when the image has links", func() {
		var aFilePath string

		BeforeEach(func() {
			aFilePath = path.Join(baseImagePath, "a_file")
			Expect(os.WriteFile(aFilePath, []byte("hello-world"), 0o600)).To(Succeed())
		})

		Describe("symlinks", func() {
			BeforeEach(func() {
				Expect(os.Symlink(aFilePath, path.Join(baseImagePath, "symlink"))).To(Succeed())
			})

			It("unpacks the symlinks", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				symlinkPath := path.Join(targetPath, "symlink")
				Expect(symlinkPath).To(BeARegularFile())

				stat, err := os.Stat(symlinkPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(stat.Mode() & os.ModeSymlink).NotTo(Equal(0))
			})

			Context("ownership", func() {
				BeforeEach(func() {
					Expect(os.Symlink("/", filepath.Join(baseImagePath, "groot_link"))).To(Succeed())
					Expect(os.Lchown(filepath.Join(baseImagePath, "groot_link"), 1000, 1000)).To(Succeed())
				})

				It("preserves it", func() {
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:     stream,
						TargetPath: targetPath,
					})
					Expect(err).NotTo(HaveOccurred())

					filePath := path.Join(targetPath, "groot_link")
					Expect(filePath).To(BeAnExistingFile())
					stat, err := os.Lstat(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
				})

				Context("when there are links owned by multiple users", func() {
					BeforeEach(func() {
						Expect(os.Symlink("/", filepath.Join(baseImagePath, "200_link"))).To(Succeed())
						Expect(os.Lchown(filepath.Join(baseImagePath, "200_link"), 200, 200)).To(Succeed())

						Expect(os.Symlink("/", filepath.Join(baseImagePath, "1200_link"))).To(Succeed())
						Expect(os.Lchown(filepath.Join(baseImagePath, "1200_link"), 1200, 1200)).To(Succeed())
					})

					It("maps their ownership", func() {
						_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
							Stream:     stream,
							TargetPath: targetPath,
						})
						Expect(err).NotTo(HaveOccurred())

						filePath := path.Join(targetPath, "200_link")
						Expect(filePath).To(BeAnExistingFile())
						stat, err := os.Lstat(filePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10 + 200)))
						Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10 + 200)))

						filePath = path.Join(targetPath, "1200_link")
						Expect(filePath).To(BeAnExistingFile())
						stat, err = os.Lstat(filePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(2000 + 1200)))
						Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(2000 + 1200)))

						filePath = path.Join(targetPath, "groot_link")
						Expect(filePath).To(BeAnExistingFile())
						stat, err = os.Lstat(filePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
						Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
					})
				})
			})

			Context("when the link name already exists", func() {
				BeforeEach(func() {
					Expect(os.WriteFile(filepath.Join(targetPath, "symlink"), []byte{}, 0o777)).To(Succeed())
				})

				It("overwrites it", func() {
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:     stream,
						TargetPath: targetPath,
					})
					Expect(err).NotTo(HaveOccurred())

					symlinkFilePath := filepath.Join(targetPath, "symlink")
					stat, err := os.Lstat(symlinkFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Mode() & os.ModeSymlink).ToNot(BeZero())
				})
			})

			Describe("when BaseDirectory is provided", func() {
				It("unpacks the symlinks inside that directory", func() {
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:        stream,
						TargetPath:    targetPath,
						BaseDirectory: "base-dir",
					})
					Expect(err).NotTo(HaveOccurred())

					symlinkPath := path.Join(targetPath, "base-dir", "symlink")
					Expect(symlinkPath).To(BeARegularFile())

					stat, err := os.Stat(symlinkPath)
					Expect(err).NotTo(HaveOccurred())

					Expect(stat.Mode() & os.ModeSymlink).NotTo(Equal(0))
				})
			})
		})

		Describe("hardlinks", func() {
			BeforeEach(func() {
				Expect(os.Link(aFilePath, path.Join(baseImagePath, "hardlink"))).To(Succeed())
			})

			It("unpacks the hardlinks", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})
				Expect(err).NotTo(HaveOccurred())

				hardLinkPath := path.Join(targetPath, "hardlink")
				Expect(hardLinkPath).To(BeAnExistingFile())

				hlStat, err := os.Stat(hardLinkPath)
				Expect(err).NotTo(HaveOccurred())

				origPath := path.Join(targetPath, "a_file")
				Expect(err).NotTo(HaveOccurred())

				origStat, err := os.Stat(origPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(os.SameFile(hlStat, origStat)).To(BeTrue())
			})

			Describe("when BaseDirectory is provided", func() {
				It("unpacks the hardlinks inside that directory", func() {
					Expect(os.MkdirAll(filepath.Join(targetPath, "hello/world"), 0o755)).To(Succeed())
					_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
						Stream:        stream,
						TargetPath:    targetPath,
						BaseDirectory: "base-dir",
					})
					Expect(err).NotTo(HaveOccurred())

					hardLinkPath := path.Join(targetPath, "base-dir", "hardlink")
					Expect(hardLinkPath).To(BeAnExistingFile())

					hlStat, err := os.Stat(hardLinkPath)
					Expect(err).NotTo(HaveOccurred())

					origPath := path.Join(targetPath, "base-dir", "a_file")
					Expect(err).NotTo(HaveOccurred())

					origStat, err := os.Stat(origPath)
					Expect(err).NotTo(HaveOccurred())

					Expect(os.SameFile(hlStat, origStat)).To(BeTrue())
				})
			})
		})
	})

	Context("setuid and setgid permissions", func() {
		BeforeEach(func() {
			setuidFilePath := filepath.Join(baseImagePath, "setuid_file")
			Expect(os.WriteFile(setuidFilePath, []byte("hello-world"), 0o755)).To(Succeed())
			Expect(os.Chmod(setuidFilePath, 0o755|os.ModeSetuid|os.ModeSetgid)).To(Succeed())
		})

		It("keeps setuid and setgid permission", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(targetPath, "setuid_file")
			stat, err := os.Stat(filePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
			Expect(stat.Mode() & os.ModeSetgid).To(Equal(os.ModeSetgid))
		})
	})

	Context("when it has whiteout files", func() {
		BeforeEach(func() {
			// Add some pre-existing files in the rootfs
			Expect(os.WriteFile(path.Join(targetPath, "b_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "a_dir"), 0o755)).To(Succeed())
			Expect(os.WriteFile(path.Join(targetPath, "a_dir", "a_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "b_dir"), 0o755)).To(Succeed())
			Expect(os.WriteFile(path.Join(targetPath, "b_dir", "a_file"), []byte(""), 0o600)).To(Succeed())

			// Add some whiteouts
			Expect(os.WriteFile(path.Join(baseImagePath, ".wh.b_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "a_dir"), 0o755)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, "a_dir", ".wh.a_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, ".wh.b_dir"), []byte(""), 0o600)).To(Succeed())
		})

		It("does not leak the whiteout files", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(path.Join(targetPath, ".wh.b_file")).NotTo(BeAnExistingFile())
			Expect(path.Join(targetPath, "a_dir", ".wh.a_file")).NotTo(BeAnExistingFile())
			Expect(path.Join(targetPath, ".wh.b_dir")).NotTo(BeAnExistingFile())
		})

		It("creates dev 0 character devices to simulate file deletions", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			bFilePath := path.Join(targetPath, "b_file")
			stat, err := os.Stat(bFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Mode()).To(Equal(os.ModeCharDevice|os.ModeDevice), "Whiteout file is not a character device")
			stat_t := stat.Sys().(*syscall.Stat_t)
			Expect(stat_t.Rdev).To(Equal(deviceNumber), "Whiteout file has incorrect device number")

			aFilePath := path.Join(targetPath, "a_dir", "a_file")
			stat, err = os.Stat(aFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Mode()).To(Equal(os.ModeCharDevice|os.ModeDevice), "Whiteout file is not a character device")
			stat_t = stat.Sys().(*syscall.Stat_t)
			Expect(stat_t.Rdev).To(Equal(deviceNumber), "Whiteout file has incorrect device number")
		})

		Context("when it fails to link the whiteout device", func() {
			BeforeEach(func() {
				var err error
				tempDir, err := os.MkdirTemp("", "")
				Expect(err).NotTo(HaveOccurred())
				storeDirFile, err = os.Open(tempDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.RemoveAll(tempDir)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})

				Expect(err).To(MatchError(ContainSubstring("failed to create whiteout node")))
			})
		})
	})

	Context("when there are opaque whiteouts", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(path.Join(baseImagePath, "whiteout_dir"), 0o755)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, "whiteout_dir", "a_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, "whiteout_dir", "b_file"), []byte(""), 0o600)).To(Succeed())
			Expect(os.WriteFile(path.Join(baseImagePath, "whiteout_dir", ".wh..wh..opq"), []byte(""), 0o600)).To(Succeed())
		})

		It("returns them in the unpack output", func() {
			unpackOutput, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(unpackOutput.OpaqueWhiteouts).To(ContainElement("whiteout_dir/.wh..wh..opq"))
		})

		It("keeps the parent directory", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(path.Join(targetPath, "whiteout_dir")).To(BeADirectory())
		})

		It("does not leak the whiteout file", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(path.Join(targetPath, "whiteout_dir", ".wh..wh..opq")).NotTo(BeAnExistingFile())
		})
	})

	Context("when it fails to untar", func() {
		JustBeforeEach(func() {
			stream = gbytes.NewBuffer()
			_, _ = stream.Write([]byte("not-a-tar"))
		})

		It("returns the error", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError("unexpected EOF"))
		})
	})
})
