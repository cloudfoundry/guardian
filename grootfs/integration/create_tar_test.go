package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with local TAR images", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		baseImageFile   *os.File
		spec            groot.CreateSpec
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "local-image-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.MkdirAll(path.Join(sourceImagePath, "permissive-folder"), 0777)).To(Succeed())

		// we need to explicitly apply perms because mkdir is subject to umask
		Expect(os.Chmod(path.Join(sourceImagePath, "permissive-folder"), 0777)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile = integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()

		spec = groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        "random-id",
			Mount:     mountByDefault(),
		}
	})

	It("creates a root filesystem", func() {
		image, err := Runner.Create(spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(Runner.EnsureMounted(image)).To(Succeed())

		imageContentPath := path.Join(image.Rootfs, "foo")
		Expect(imageContentPath).To(BeARegularFile())

		fooContents, err := ioutil.ReadFile(imageContentPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fooContents)).To(Equal("hello-world"))
	})

	It("keeps folders original permissions", func() {
		image, err := Runner.Create(spec)
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())

		permissiveFolderPath := path.Join(image.Rootfs, "permissive-folder")
		stat, err := os.Stat(permissiveFolderPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0777)))
	})

	Context("when two rootfses are using the same image", func() {
		It("isolates them", func() {
			image1, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())

			image2, err := Runner.Create(groot.CreateSpec{
				ID:        "another-random-id",
				BaseImage: baseImagePath,
				Mount:     false,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(image1.Rootfs, "new-file"), []byte("hello-world"), 0644)).To(Succeed())
			Expect(filepath.Join(image2.Rootfs, "new-file")).NotTo(BeARegularFile())
		})
	})

	Context("timestamps", func() {
		var modTime time.Time

		BeforeEach(func() {
			location := time.FixedZone("foo", 0)
			modTime = time.Date(2014, 10, 14, 22, 8, 32, 0, location)

			oldFilePath := path.Join(sourceImagePath, "old-file")
			Expect(ioutil.WriteFile(oldFilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chtimes(oldFilePath, time.Now(), modTime)).To(Succeed())
		})

		It("preserves the timestamps", func() {
			image, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			imageOldFilePath := path.Join(image.Rootfs, "old-file")
			fi, err := os.Stat(imageOldFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(modTime.Unix()))
		})
	})

	Describe("clean up on create", func() {
		JustBeforeEach(func() {
			integration.SkipIfNonRoot(GrootfsTestUid)
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-image-1",
				BaseImage: baseImagePath,
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-image-1")).To(Succeed())
		})

		AfterEach(func() {
			Expect(Runner.Delete("my-image-3")).To(Succeed())
		})

		It("cleans up unused layers before create but not the one about to be created", func() {
			baseImage2File := integration.CreateBaseImageTar(sourceImagePath)
			baseImage2Path := baseImage2File.Name()

			createSpec := groot.CreateSpec{
				ID:        "my-image-2",
				BaseImage: baseImage2Path,
				Mount:     true,
			}
			_, err := Runner.Create(createSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.Delete("my-image-2")).To(Succeed())

			layerPath := filepath.Join(StorePath, store.VolumesDirName, integration.BaseImagePathToVolumeID(baseImage2Path))
			stat, err := os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			preLayerTimestamp := stat.ModTime()

			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(2))

			runner := Runner.WithClean()
			_, err = runner.Create(groot.CreateSpec{
				ID:        "my-image-3",
				BaseImage: baseImage2Path,
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(1))

			stat, err = os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.ModTime()).To(Equal(preLayerTimestamp))
		})

		Context("when no-clean flag is set", func() {
			It("does not clean up unused layers", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(1))

				baseImage2File := integration.CreateBaseImageTar(sourceImagePath)
				baseImage2Path := baseImage2File.Name()
				_, err = Runner.WithNoClean().Create(groot.CreateSpec{
					ID:        "my-image-3",
					BaseImage: baseImage2Path,
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(2))
			})
		})
	})

	Context("when the provided base image is a directory", func() {
		It("returns a sensible error", func() {
			tempDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = Runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: tempDir,
				Mount:     true,
			})
			Expect(err).To(MatchError("invalid base image: directory provided instead of a tar file"))
		})
	})

	Context("when required args are not provided", func() {
		It("returns an error", func() {
			_, err := Runner.Create(groot.CreateSpec{})
			Expect(err).To(MatchError(ContainSubstring("invalid arguments")))
		})
	})

	Context("when image content changes", func() {
		JustBeforeEach(func() {
			_, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uses the new content for the new image", func() {
			Expect(ioutil.WriteFile(path.Join(sourceImagePath, "bar"), []byte("this-is-a-bar-content"), 0644)).To(Succeed())
			integration.UpdateBaseImageTar(baseImagePath, sourceImagePath)

			image, err := Runner.Create(groot.CreateSpec{
				ID:        "random-id-2",
				BaseImage: baseImagePath,
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			imageContentPath := path.Join(image.Rootfs, "foo")
			Expect(imageContentPath).To(BeARegularFile())
			barImageContentPath := path.Join(image.Rootfs, "bar")
			Expect(barImageContentPath).To(BeARegularFile())
		})
	})

	Context("when the tar has files that point outside the root dir", func() {
		It("doesn't leak the file", func() {
			workDir, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			baseImagePath = fmt.Sprintf("%s/assets/hacked.tar", workDir)

			_, err = Runner.Create(groot.CreateSpec{
				ID:        "image-1",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(filepath.Join(StorePath, store.VolumesDirName, "file_outside_root")).ToNot(BeAnExistingFile())
		})
	})

	Describe("unpacked volume caching", func() {
		It("caches the unpacked image in a subvolume with snapshots", func() {
			_, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())

			volumeID := integration.BaseImagePathToVolumeID(baseImagePath)
			layerSnapshotPath := filepath.Join(StorePath, "volumes", volumeID)
			Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

			image, err := Runner.Create(groot.CreateSpec{
				ID:        "random-id-2",
				BaseImage: baseImagePath,
				Mount:     mountByDefault(),
			})
			Expect(Runner.EnsureMounted(image)).To(Succeed())
			Expect(err).NotTo(HaveOccurred())
			Expect(path.Join(image.Rootfs, "foo")).To(BeARegularFile())
			Expect(path.Join(image.Rootfs, "injected-file")).To(BeARegularFile())
		})
	})

	Context("when local image does not exist", func() {
		It("returns an error", func() {
			_, err := Runner.Create(groot.CreateSpec{
				BaseImage: "/invalid/image",
				ID:        "random-id",
				Mount:     false,
			})
			Expect(err).To(MatchError(ContainSubstring("stat /invalid/image: no such file or directory")))
		})
	})

	Context("when mappings are provided", func() {
		BeforeEach(func() {
			var spec runner.InitSpec
			spec.UIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootUID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
			spec.GIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootGID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}

			Runner.RunningAsUser(0, 0).InitStore(spec)
		})

		It("creates a store that correctly maps the user/group ids", func() {
			sourceImagePath := integration.CreateBaseImage(0, 0, GrootUID, GrootGID)
			baseImageFile := integration.CreateBaseImageTar(sourceImagePath)

			defer func() {
				Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
				Expect(os.RemoveAll(baseImageFile.Name())).To(Succeed())
			}()

			image, err := Runner.SkipInitStore().Create(groot.CreateSpec{
				BaseImage: baseImageFile.Name(),
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.EnsureMounted(image)).To(Succeed())
			grootFi, err := os.Stat(path.Join(image.Rootfs, "foo"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			rootFi, err := os.Stat(path.Join(image.Rootfs, "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})
	})

	Context("when the image has links", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(
				path.Join(sourceImagePath, "symlink-target"), []byte("hello-world"), 0644),
			).To(Succeed())
			Expect(os.Symlink(
				filepath.Join(sourceImagePath, "symlink-target"),
				filepath.Join(sourceImagePath, "symlink"),
			)).To(Succeed())
		})

		It("unpacks the symlinks", func() {
			image, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			content, err := ioutil.ReadFile(filepath.Join(image.Rootfs, "symlink"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("hello-world"))
		})

		Context("timestamps", func() {
			BeforeEach(func() {
				cmd := exec.Command("touch", "-h", "-d", "2014-01-01", path.Join(sourceImagePath, "symlink"))
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Wait()).Should(gexec.Exit(0))
			})

			It("preserves the timestamps", func() {
				image, err := Runner.Create(spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.EnsureMounted(image)).To(Succeed())

				symlinkTargetFilePath := path.Join(image.Rootfs, "symlink-target")
				symlinkTargetFi, err := os.Stat(symlinkTargetFilePath)
				Expect(err).NotTo(HaveOccurred())

				symlinkFilePath := path.Join(image.Rootfs, "symlink")
				symlinkFi, err := os.Lstat(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())

				location := time.FixedZone("foo", 0)
				modTime := time.Date(2014, 01, 01, 0, 0, 0, 0, location)
				Expect(symlinkTargetFi.ModTime().Unix()).ToNot(
					Equal(symlinkFi.ModTime().Unix()),
				)
				Expect(symlinkFi.ModTime().Unix()).To(Equal(modTime.Unix()))
			})
		})
	})
})
