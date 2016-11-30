package unpacker_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar unpacker", func() {
	var (
		tarUnpacker   *unpacker.TarUnpacker
		logger        lager.Logger
		baseImagePath string
		stream        *gbytes.Buffer
		targetPath    string
	)

	BeforeEach(func() {
		tarUnpacker = unpacker.NewTarUnpacker()

		var err error
		targetPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test-store")
	})

	JustBeforeEach(func() {
		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", baseImagePath, "."), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(targetPath)).To(Succeed())
	})

	Describe("regular files", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(path.Join(baseImagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
		})

		It("creates regular files", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when parent dir does not have writting permission", func() {
			var folderPath string

			BeforeEach(func() {
				folderPath = path.Join(baseImagePath, "root", "that-stuff")
				Expect(os.MkdirAll(folderPath, 0777)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(folderPath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
				Expect(os.Chmod(folderPath, 0555)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.Chmod(folderPath, 0777)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})).To(MatchError(ContainSubstring("Directory '/root/that-stuff' does not give write permission to its owner. This image can only be unpacked by root.")))
			})
		})
	})

	Describe("directories", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(path.Join(baseImagePath, "subdir"), 0700)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "subdir", "subdir2"), 0777)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(baseImagePath, "subdir", "subdir2", "another_file"), []byte("goodbye-world"), 0600)).To(Succeed())
		})

		It("creates files in subdirectories", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "subdir", "subdir2", "another_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("goodbye-world"))
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
			Expect(ioutil.WriteFile(filePath, []byte("hello-world"), 0600)).To(Succeed())
			Expect(os.Chtimes(filePath, time.Now(), fileModTime)).To(Succeed())

			dirModTime = time.Date(2014, 9, 3, 22, 8, 32, 0, location)
			dirPath := path.Join(baseImagePath, "old-dir")
			Expect(os.Mkdir(dirPath, 0700)).To(Succeed())
			Expect(os.Chtimes(dirPath, time.Now(), dirModTime)).To(Succeed())
		})

		It("preserves the modtime for files", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			fi, err := os.Stat(path.Join(targetPath, "old-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(fileModTime.Unix()))
		})

		It("preserves the modtime for directories", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			fi, err := os.Stat(path.Join(targetPath, "old-dir"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(dirModTime.Unix()))
		})
	})

	Describe("permissions", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(path.Join(baseImagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "a_dir"), 0700)).To(Succeed())

			// We have to chmod it because creat and mkdir syscalls take the umask into
			// account when applying the permissions. This means that only permissions
			// less permissive than the umask can be applied to files and directories.
			// By calling chmod we explicitly apply the permissions without being
			// subject to the umask.
			Expect(os.Chmod(path.Join(baseImagePath, "a_file"), 0777)).To(Succeed())
			Expect(os.Chmod(path.Join(baseImagePath, "a_dir"), 0777)).To(Succeed())
		})

		It("keeps file permissions", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "a_file")
			stat, err := os.Stat(filePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0777)))
		})

		It("keeps directory permissions", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			dirPath := path.Join(targetPath, "a_dir")
			stat, err := os.Stat(dirPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0777)))
		})
	})

	Context("when the image has links", func() {
		BeforeEach(func() {
			aFilePath := path.Join(baseImagePath, "a_file")
			Expect(ioutil.WriteFile(aFilePath, []byte("hello-world"), 0600)).To(Succeed())
			Expect(os.Symlink(aFilePath, path.Join(baseImagePath, "symlink"))).To(Succeed())
			Expect(os.Link(aFilePath, path.Join(baseImagePath, "hardlink"))).To(Succeed())
		})

		It("unpacks the symlinks", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			symlinkPath := path.Join(targetPath, "symlink")
			Expect(symlinkPath).To(BeARegularFile())

			stat, err := os.Stat(symlinkPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode() & os.ModeSymlink).NotTo(Equal(0))
		})

		It("unpacks the hardlinks", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

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
	})

	Context("setuid and setgid permissions", func() {
		BeforeEach(func() {
			setuidFilePath := filepath.Join(baseImagePath, "setuid_file")
			Expect(ioutil.WriteFile(setuidFilePath, []byte("hello-world"), 0755)).To(Succeed())
			Expect(os.Chmod(setuidFilePath, 0755|os.ModeSetuid|os.ModeSetgid)).To(Succeed())
		})

		It("keeps setuid and setgid permission", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

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
			Expect(ioutil.WriteFile(path.Join(targetPath, "b_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "a_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(targetPath, "a_dir", "a_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "b_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(targetPath, "b_dir", "a_file"), []byte(""), 0600)).To(Succeed())

			// Add some whiteouts
			Expect(ioutil.WriteFile(path.Join(baseImagePath, ".wh.b_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(baseImagePath, "a_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(baseImagePath, "a_dir", ".wh.a_file"), []byte(""), 0600)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(baseImagePath, ".wh.b_dir"), []byte(""), 0600)).To(Succeed())
		})

		It("deletes the pre-existing files", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			Expect(path.Join(targetPath, "b_file")).NotTo(BeAnExistingFile())
			Expect(path.Join(targetPath, "a_dir", "a_file")).NotTo(BeAnExistingFile())
		})

		It("deletes the pre-existing directories", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			Expect(path.Join(targetPath, "b_dir")).NotTo(BeAnExistingFile())
		})

		It("does not leak the whiteout files", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			Expect(path.Join(targetPath, ".wh.b_file")).NotTo(BeAnExistingFile())
			Expect(path.Join(targetPath, "a_dir", ".wh.a_file")).NotTo(BeAnExistingFile())
			Expect(path.Join(targetPath, ".wh.b_dir")).NotTo(BeAnExistingFile())
		})

		Context("when there are opaque whiteouts", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(path.Join(baseImagePath, "whiteout_dir"), 0755)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(baseImagePath, "whiteout_dir", "a_file"), []byte(""), 0600)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(baseImagePath, "whiteout_dir", "b_file"), []byte(""), 0600)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(baseImagePath, "whiteout_dir", ".wh..wh..opq"), []byte(""), 0600)).To(Succeed())
			})

			It("cleans up the folder", func() {
				Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})).To(Succeed())

				Expect(path.Join(targetPath, "whiteout_dir", "a_file")).NotTo(BeAnExistingFile())
				Expect(path.Join(targetPath, "whiteout_dir", "b_file")).NotTo(BeAnExistingFile())
			})

			It("keeps the parent directory", func() {
				Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})).To(Succeed())

				Expect(path.Join(targetPath, "whiteout_dir")).To(BeADirectory())
			})

			It("does not leak the whiteout file", func() {
				Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
					Stream:     stream,
					TargetPath: targetPath,
				})).To(Succeed())

				Expect(path.Join(targetPath, "whiteout_dir", ".wh..wh..opq")).NotTo(BeAnExistingFile())
			})
		})
	})

	Context("when it fails to untar", func() {
		JustBeforeEach(func() {
			stream = gbytes.NewBuffer()
			stream.Write([]byte("not-a-tar"))
		})

		It("returns the error", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(MatchError(ContainSubstring("unexpected EOF")))
		})
	})

	Context("when creating the target directory fails", func() {
		It("returns an error", func() {
			err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: "/some-destination/images/1000",
			})

			Expect(err).To(MatchError(ContainSubstring("making destination directory")))
		})
	})

	Context("when the target does not exist", func() {
		It("still works", func() {
			Expect(os.RemoveAll(targetPath)).To(Succeed())

			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())
		})
	})
})
