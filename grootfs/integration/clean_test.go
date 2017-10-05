package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Clean", func() {
	Context("Local Images", func() {
		var (
			baseImagePath             string
			anotherBaseImagePath      string
			yetAnotherBaseImagePath   string
			sourceImagePath           string
			anotherSourceImagePath    string
			yetAnotherSourceImagePath string
		)

		BeforeEach(func() {
			var err error
			sourceImagePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			sess, err := gexec.Start(exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(sourceImagePath, "foo")), "count=2", "bs=1M"), GinkgoWriter, nil)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
			baseImagePath = baseImageFile.Name()

			anotherSourceImagePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			sess, err = gexec.Start(exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(anotherSourceImagePath, "foo")), "count=2", "bs=1M"), GinkgoWriter, nil)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			Expect(os.MkdirAll(path.Join(anotherSourceImagePath, "prohibited-folder"), 0777)).To(Succeed())
			Expect(os.Chown(path.Join(anotherSourceImagePath, "prohibited-folder"), 4000, 4000)).To(Succeed())
			Expect(os.Chmod(path.Join(anotherSourceImagePath, "prohibited-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(anotherSourceImagePath, "prohibited-folder", "file"), []byte{}, 0700)).To(Succeed())
			anotherBaseImageFile := integration.CreateBaseImageTar(anotherSourceImagePath)
			anotherBaseImagePath = anotherBaseImageFile.Name()

			yetAnotherSourceImagePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			yetAnotherBaseImageFile := integration.CreateBaseImageTar(yetAnotherSourceImagePath)
			yetAnotherBaseImagePath = yetAnotherBaseImageFile.Name()

			_, err = Runner.Create(groot.CreateSpec{
				ID:           "my-image-1",
				BaseImageURL: integration.String2URL(baseImagePath),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(Runner.Delete("my-image-1")).To(Succeed())
		})

		Context("when the store doesn't exist", func() {
			It("logs an error message and exits successfully", func() {
				logBuffer := gbytes.NewBuffer()
				_, err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).Clean(0)
				Expect(err).ToNot(HaveOccurred())
				Expect(logBuffer).To(gbytes.Say(`"error":"no store found at /invalid-store"`))
			})
		})

		Context("when there are unused volumes", func() {
			BeforeEach(func() {
				_, err := Runner.Create(groot.CreateSpec{
					ID:           "my-image-2",
					BaseImageURL: integration.String2URL(anotherBaseImagePath),
					Mount:        mountByDefault(),
					DiskLimit:    10 * 1024 * 1024,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(Runner.Delete("my-image-2")).To(Succeed())
			})

			It("removes them", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(2))

				_, err = Runner.Clean(0)
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(1))

				afterDir := afterContents[0].Name()
				afterContentsSha := sha256.Sum256([]byte(baseImagePath))
				Expect(afterDir).To(MatchRegexp(`%s-\d+`, hex.EncodeToString(afterContentsSha[:32])))
			})

			Context("and a cache size is set", func() {
				var cacheSizeInBytes int64

				Context("but it lower than 0", func() {
					BeforeEach(func() {
						cacheSizeInBytes = -10
					})
					It("returns an error", func() {
						_, err := Runner.Clean(cacheSizeInBytes)
						Expect(err).To(MatchError("invalid argument: cache size cannot be negative"))
					})
				})

				Context("when one of the volumes has a missing metadata", func() {
					BeforeEach(func() {
						metadataFiles, err := filepath.Glob(filepath.Join(StorePath, store.MetaDirName, "volume-*"))
						Expect(err).NotTo(HaveOccurred())

						for _, metadataFile := range metadataFiles {
							Expect(os.RemoveAll(metadataFile)).To(Succeed())
						}
					})

					It("doesn't fail", func() {
						_, err := Runner.Clean(0)
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("and the size of unused layers is less than the cache size", func() {
					BeforeEach(func() {
						cacheSizeInBytes = 3 * 1024 * 1024
					})

					It("does not remove the unused volumes", func() {
						preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())

						_, err = Runner.Clean(cacheSizeInBytes)
						Expect(err).NotTo(HaveOccurred())

						afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(afterContents).To(HaveLen(len(preContents)))
					})

					It("reports that it was a no-op", func() {
						output, err := Runner.Clean(cacheSizeInBytes)
						Expect(err).NotTo(HaveOccurred())
						Expect(output).To(ContainSubstring("cache size not reached: skipping clean"))
					})
				})

				Context("and the total is more than the cache size", func() {
					BeforeEach(func() {
						cacheSizeInBytes = 1 * 1024 * 1024
					})

					It("removes the unused volumes", func() {
						preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(preContents).To(HaveLen(2))

						_, err = Runner.Clean(cacheSizeInBytes)
						Expect(err).NotTo(HaveOccurred())

						afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
						Expect(err).NotTo(HaveOccurred())
						Expect(afterContents).To(HaveLen(1))
					})
				})
			})
		})
	})

	Context("Remote Images", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:           "my-image-1",
				BaseImageURL: integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(Runner.Delete("my-image-1")).To(Succeed())
		})

		Context("when there are unused layers", func() {
			BeforeEach(func() {
				_, err := Runner.Create(groot.CreateSpec{
					ID:           "my-image-2",
					BaseImageURL: integration.String2URL("docker:///cfgarden/garden-busybox"),
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(Runner.Delete("my-image-2")).To(Succeed())
			})

			It("removes unused volumes", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(3))

				_, err = Runner.Clean(0)
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(2))
				for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
					Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
				}
			})
		})
	})
})
