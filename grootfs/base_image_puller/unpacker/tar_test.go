package unpacker_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar", func() {
	var (
		logger lager.Logger

		imgPath    string
		bundlePath string
		targetPath string
		aFilePath  string

		tarUnpacker *unpacker.TarUnpacker

		stream *gbytes.Buffer
	)

	BeforeEach(func() {
		var err error

		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		targetPath = path.Join(bundlePath, "rootfs")

		tarUnpacker = unpacker.NewTarUnpacker()

		logger = lagertest.NewTestLogger("test-store")

		imgPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		aFilePath = path.Join(imgPath, "a_file")
		Expect(ioutil.WriteFile(aFilePath, []byte("hello-world"), 0600)).To(Succeed())
		Expect(os.Mkdir(path.Join(imgPath, "subdir"), 0700)).To(Succeed())
		Expect(os.Mkdir(path.Join(imgPath, "subdir", "subdir2"), 0711)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(imgPath, "subdir", "subdir2", "another_file"), []byte("goodbye-world"), 0600)).To(Succeed())
	})

	JustBeforeEach(func() {
		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", imgPath, "."), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imgPath)).To(Succeed())
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	It("does write the image contents in the rootfs directory", func() {
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

	Context("when the image has links", func() {
		BeforeEach(func() {
			Expect(os.Symlink(aFilePath, path.Join(imgPath, "symlink"))).To(Succeed())
			Expect(os.Link(aFilePath, path.Join(imgPath, "hardlink"))).To(Succeed())
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

	It("keeps file permissions", func() {
		Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     stream,
			TargetPath: targetPath,
		})).To(Succeed())

		filePath := path.Join(targetPath, "a_file")
		stat, err := os.Stat(filePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0600)))
	})

	It("keeps directory permission", func() {
		Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     stream,
			TargetPath: targetPath,
		})).To(Succeed())

		filePath := path.Join(targetPath, "subdir", "subdir2")
		stat, err := os.Stat(filePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0711)))
	})

	Context("when there are device files", func() {
		BeforeEach(func() {
			Expect(exec.Command("sudo", "mknod", path.Join(imgPath, "somedevice"), "c", "1", "8").Run()).To(Succeed())
		})

		It("excludes them", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "somedevice")
			Expect(filePath).ToNot(BeAnExistingFile())
		})
	})

	Context("when it has whiteout files", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(targetPath, 0755)).To(Succeed())

			// Add some pre-existing files in the rootfs
			Expect(ioutil.WriteFile(path.Join(targetPath, "b_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "a_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(targetPath, "a_dir", "a_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(targetPath, "b_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(targetPath, "b_dir", "a_file"), []byte(""), 0600)).To(Succeed())

			// Add some whiteouts
			Expect(ioutil.WriteFile(path.Join(imgPath, ".wh.b_file"), []byte(""), 0600)).To(Succeed())
			Expect(os.Mkdir(path.Join(imgPath, "a_dir"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(imgPath, "a_dir", ".wh.a_file"), []byte(""), 0600)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(imgPath, ".wh.b_dir"), []byte(""), 0600)).To(Succeed())
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
				Expect(os.Mkdir(path.Join(imgPath, "whiteout_dir"), 0755)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(imgPath, "whiteout_dir", "a_file"), []byte(""), 0600)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(imgPath, "whiteout_dir", "b_file"), []byte(""), 0600)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(imgPath, "whiteout_dir", ".wh..wh..opq"), []byte(""), 0600)).To(Succeed())
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

		It("returns an error", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).NotTo(Succeed())
		})

		It("returns the command output", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(
				MatchError(ContainSubstring("tar:")),
			)
		})
	})

	Context("when creating the target directory fails", func() {
		It("returns an error", func() {
			err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: "/some-destination/bundles/1000",
			})

			Expect(err).To(MatchError(ContainSubstring("making destination directory")))
		})
	})

	Context("when the target directory exists", func() {
		It("still works", func() {
			Expect(os.Mkdir(targetPath, 0770)).To(Succeed())

			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())
		})
	})
})
