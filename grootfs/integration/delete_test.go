package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

var _ = Describe("Delete", func() {
	var (
		randomImageID   string
		sourceImagePath string
		baseImagePath   string
		containerSpec   specs.Spec
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())

		randomImageID = testhelpers.NewRandomID()
		containerSpec = specs.Spec{}
		baseImagePath = ""
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		var err error
		containerSpec, err = Runner.Create(groot.CreateSpec{
			BaseImageURL: integration.String2URL(baseImagePath),
			ID:           randomImageID,
			Mount:        mountByDefault(),
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("deletes an existing image", func() {
		Expect(Runner.Delete(randomImageID)).To(Succeed())
		Expect(filepath.Dir(containerSpec.Root.Path)).NotTo(BeAnExistingFile())
	})

	Context("when there is a file in a directory groot doesn't have search permission on", func() {
		JustBeforeEach(func() {
			privateFolder := filepath.Join(containerSpec.Root.Path, "private-folder")
			Expect(os.Mkdir(privateFolder, 0600)).To(Succeed())
			f, err := os.Create(filepath.Join(privateFolder, "file"))
			Expect(f.Close()).To(Succeed())
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Chown(privateFolder, 100001, 100001)).To(Succeed())
			Expect(os.Chown(filepath.Join(privateFolder, "file"), 100001, 100001)).To(Succeed())
		})

		It("still succeeds", func() {
			Expect(Runner.Delete(randomImageID)).To(Succeed())
			Expect(filepath.Dir(containerSpec.Root.Path)).NotTo(BeAnExistingFile())
		})
	})

	Context("when the store doesn't exist", func() {
		It("logs the image path", func() {
			logBuffer := gbytes.NewBuffer()
			err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
				Delete("/path/to/random-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say(`"id":"/path/to/random-id"`))
		})
	})

	Context("when a path is provided instead of an ID", func() {
		It("deletes the image by the path", func() {
			Expect(filepath.Dir(containerSpec.Root.Path)).To(BeAnExistingFile())
			Expect(Runner.Delete(filepath.Dir(containerSpec.Root.Path))).To(Succeed())
			Expect(filepath.Dir(containerSpec.Root.Path)).NotTo(BeAnExistingFile())
		})

		Context("when a path to an image does not exist", func() {
			It("succeeds but logs a warning", func() {
				fakePath := path.Join(StorePath, "images/non_existing")
				Expect(fakePath).NotTo(BeAnExistingFile())

				outBuffer := gbytes.NewBuffer()
				err := Runner.WithStdout(outBuffer).Delete(fakePath)
				Expect(err).NotTo(HaveOccurred())

				Eventually(outBuffer).Should(gbytes.Say("Image `non_existing` not found. Skipping delete."))
			})
		})

		Context("when the path provided doesn't belong to the `--store` provided", func() {
			It("returns an error", func() {
				outBuffer := gbytes.NewBuffer()
				err := Runner.WithStdout(outBuffer).Delete("/Iamnot/in/the/storage/images/1234/rootfs")
				Expect(err).ToNot(HaveOccurred())

				Eventually(outBuffer).Should(gbytes.Say("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path"))
			})
		})
	})

	Context("when the image ID doesn't exist", func() {
		It("succeeds but logs a warning", func() {
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithStdout(outBuffer).Delete("non-existing-id")
			Expect(err).NotTo(HaveOccurred())

			Eventually(outBuffer).Should(gbytes.Say("Image `non-existing-id` not found. Skipping delete."))
		})
	})

	Context("when it fails to delete the image", func() {
		var mntPoint string

		JustBeforeEach(func() {
			mntPoint = filepath.Join(filepath.Dir(containerSpec.Root.Path), "mnt")
			Expect(os.Mkdir(mntPoint, 0700)).To(Succeed())
			Expect(unix.Mount(mntPoint, mntPoint, "none", unix.MS_BIND, "")).To(Succeed())
		})

		AfterEach(func() {
			Expect(unix.Unmount(mntPoint, unix.MNT_DETACH)).To(Succeed())
		})

		It("doesn't remove the metadata file", func() {
			metadataPath := filepath.Join(StorePath, store.MetaDirName, "dependencies", fmt.Sprintf("image:%s.json", randomImageID))
			Expect(metadataPath).To(BeAnExistingFile())
			Expect(Runner.Delete(filepath.Dir(containerSpec.Root.Path))).To(MatchError(ContainSubstring("deleting image path")))
			Expect(metadataPath).To(BeAnExistingFile())
		})
	})

	Context("when it fails to unmount the rootfs directory", func() {
		var mntPoint string

		BeforeEach(func() {
			integration.SkipIfNonRoot(GrootfsTestUid)
		})

		JustBeforeEach(func() {
			mntPoint = filepath.Join(filepath.Dir(containerSpec.Root.Path), overlayxfs.RootfsDir, "mnt")
			Expect(os.Mkdir(mntPoint, 0700)).To(Succeed())
			Expect(unix.Mount(mntPoint, mntPoint, "none", unix.MS_BIND, "")).To(Succeed())
		})

		AfterEach(func() {
			Expect(unix.Unmount(mntPoint, unix.MNT_DETACH)).To(Succeed())
		})

		It("doesn't remove the diff directory", func() {
			diffPath := filepath.Join(filepath.Dir(containerSpec.Root.Path), overlayxfs.UpperDir)
			Expect(Runner.Delete(filepath.Dir(containerSpec.Root.Path))).To(MatchError(ContainSubstring("deleting image path")))
			Expect(diffPath).To(BeADirectory())
		})

	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithStdout(outBuffer).Delete("")
			Expect(err).To(HaveOccurred())

			Eventually(outBuffer).Should(gbytes.Say("id was not specified"))
		})
	})
})
