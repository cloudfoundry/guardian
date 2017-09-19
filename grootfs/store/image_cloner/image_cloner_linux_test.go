package image_cloner_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	imageclonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/image_cloner/image_clonerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Image", func() {
	var (
		logger      lager.Logger
		storePath   string
		imagesPath  string
		imageCloner *imageclonerpkg.ImageCloner
		imageConfig specsv1.Image

		fakeImageDriver *image_clonerfakes.FakeImageDriver
	)

	BeforeEach(func() {
		var err error
		fakeImageDriver = new(image_clonerfakes.FakeImageDriver)

		fakeImageDriver.CreateImageStub = func(_ lager.Logger, spec imageclonerpkg.ImageDriverSpec) (groot.MountInfo, error) {
			return groot.MountInfo{
				Source:      "my-source",
				Type:        "my-type",
				Destination: "my-destination",
				Options:     []string{"my-option"},
			}, os.Mkdir(filepath.Join(spec.ImagePath, "rootfs"), 0777)
		}

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		imagesPath = filepath.Join(storePath, "images")
		timestamp := time.Now()
		imageConfig = specsv1.Image{Created: &timestamp, Config: specsv1.ImageConfig{}}

		Expect(os.Mkdir(imagesPath, 0777)).To(Succeed())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-bundler")
		imageCloner = imageclonerpkg.NewImageCloner(fakeImageDriver, storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("Create", func() {
		It("returns a image directory", func() {
			image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig})
			Expect(err).NotTo(HaveOccurred())
			Expect(image.Path).To(BeADirectory())
		})

		It("returns an image", func() {
			image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig, Mount: true})
			Expect(err).NotTo(HaveOccurred())

			Expect(image.Rootfs).To(Equal(filepath.Join(imagesPath, "some-id/rootfs")))
			Expect(image.Image.Created.Unix()).To(Equal(imageConfig.Created.Unix()))
			Expect(image.Mounts).To(BeNil())
		})

		It("keeps the images in the same image directory", func() {
			someImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig})
			Expect(err).NotTo(HaveOccurred())
			anotherImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "another-id", BaseImage: imageConfig})
			Expect(err).NotTo(HaveOccurred())

			Expect(someImage.Path).NotTo(BeEmpty())
			Expect(anotherImage.Path).NotTo(BeEmpty())

			images, err := ioutil.ReadDir(path.Join(storePath, store.ImageDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(images)).To(Equal(2))
		})

		It("creates the snapshot", func() {
			imageSpec := groot.ImageSpec{
				ID:            "some-id",
				BaseVolumeIDs: []string{"id-1", "id-2"},
				BaseImage: specsv1.Image{
					Author: "Groot",
					Config: specsv1.ImageConfig{},
				},
			}
			image, err := imageCloner.Create(logger, imageSpec)
			Expect(err).NotTo(HaveOccurred())

			_, spec := fakeImageDriver.CreateImageArgsForCall(0)
			Expect(spec.BaseVolumeIDs).To(Equal(imageSpec.BaseVolumeIDs))
			Expect(spec.ImagePath).To(Equal(image.Path))
		})

		Context("when mounting is skipped", func() {
			It("returns a image with mount information", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig, Mount: false})
				Expect(err).NotTo(HaveOccurred())

				Expect(image.Mounts).ToNot(BeNil())
				Expect(image.Mounts[0].Destination).To(Equal("my-destination"))
				Expect(image.Mounts[0].Source).To(Equal("my-source"))
				Expect(image.Mounts[0].Type).To(Equal("my-type"))
				Expect(image.Mounts[0].Options).To(ConsistOf("my-option"))
			})
		})

		Context("when the image has volumes", func() {
			var image groot.ImageInfo
			var mountSourceName string

			JustBeforeEach(func() {
				imageConfig := specsv1.Image{
					Config: specsv1.ImageConfig{
						Volumes: map[string]struct{}{"/foo": struct{}{}},
					},
				}

				var err error
				image, err = imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig, Mount: false})
				Expect(err).NotTo(HaveOccurred())

				volumeHash := sha256.Sum256([]byte("/foo"))
				mountSourceName = "vol-" + hex.EncodeToString(volumeHash[:32])
			})

			It("creates the source folders", func() {
				Expect(filepath.Join(image.Path, mountSourceName)).To(BeADirectory())
			})

			It("returns the mount information", func() {
				Expect(image.Mounts).To(ContainElement(groot.MountInfo{
					Destination: "/foo",
					Source:      filepath.Join(image.Path, mountSourceName),
					Type:        "bind",
					Options:     []string{"bind"},
				}))
			})
		})

		Describe("created files ownership", func() {
			It("will change the ownership of all artifacts it creates", func() {
				uid := 2525
				gid := 2525

				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:        "some-id",
					OwnerUID:  uid,
					OwnerGID:  gid,
					BaseImage: imageConfig,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(BeADirectory())

				imagePath, err := os.Stat(image.Path)
				Expect(err).NotTo(HaveOccurred())
				Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
				Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(gid)))

				rootfsPath, err := os.Stat(image.Rootfs)
				Expect(err).NotTo(HaveOccurred())
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(gid)))
			})

			Context("when only owner UID is 0", func() {
				It("tries to enforce ownership", func() {
					image, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:        "some-id",
						OwnerUID:  0,
						OwnerGID:  10000,
						BaseImage: imageConfig,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(image.Path).To(BeADirectory())

					imagePath, err := os.Stat(image.Path)
					Expect(err).NotTo(HaveOccurred())
					Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
					Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10000)))

					rootfsPath, err := os.Stat(image.Rootfs)
					Expect(err).NotTo(HaveOccurred())
					Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
					Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10000)))
				})
			})

			Context("when only owner GID is 0", func() {
				It("tries to enforce ownership", func() {
					image, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:        "some-id",
						OwnerUID:  50000,
						OwnerGID:  0,
						BaseImage: imageConfig,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(image.Path).To(BeADirectory())

					imagePath, err := os.Stat(image.Path)
					Expect(err).NotTo(HaveOccurred())
					Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(50000)))
					Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))

					rootfsPath, err := os.Stat(image.Rootfs)
					Expect(err).NotTo(HaveOccurred())
					Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(50000)))
					Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
				})
			})

			Context("when both owner IDs are 0", func() {
				It("doesn't enforce any ownership", func() {
					_, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:        "some-id",
						OwnerUID:  0,
						OwnerGID:  0,
						BaseImage: imageConfig,
					})

					// Because a normal user cannot change the onwership of a file to root
					// the fact that this hasn't failed proves that it didn't try
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when calling it with two different ids", func() {
			It("returns two different image paths", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig})
				Expect(err).NotTo(HaveOccurred())

				anotherImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "another-id", BaseImage: imageConfig})
				Expect(err).NotTo(HaveOccurred())

				Expect(image.Path).NotTo(Equal(anotherImage.Path))
			})
		})

		Context("when the store path does not exist", func() {
			BeforeEach(func() {
				storePath = "/non/existing/store"
			})

			It("should return an error", func() {
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig})
				Expect(err).To(MatchError(ContainSubstring("making image path")))
			})
		})

		Context("when creating the image fails", func() {
			BeforeEach(func() {
				fakeImageDriver.CreateImageReturns(groot.MountInfo{}, errors.New("failed to create image"))
			})

			It("returns an error", func() {
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id", BaseImage: imageConfig})
				Expect(err).To(MatchError(ContainSubstring("failed to create image")))
			})

			It("removes the image", func() {
				imageID := "some-id"
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: imageID, BaseImage: imageConfig})
				Expect(err).To(HaveOccurred())
				Expect(filepath.Join(imagesPath, imageID)).NotTo(BeADirectory())
			})
		})

		Context("when a disk limit is set", func() {
			It("applies the disk limit", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
					BaseImage: imageConfig,
				})
				Expect(err).NotTo(HaveOccurred())

				_, spec := fakeImageDriver.CreateImageArgsForCall(0)
				Expect(spec.ImagePath).To(Equal(image.Path))
				Expect(spec.DiskLimit).To(Equal(int64(1024)))
				Expect(spec.ExclusiveDiskLimit).To(BeFalse())
			})

			Context("when the exclusive flag is set", func() {
				It("enforces the exclusive limit", func() {
					_, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:                        "some-id",
						DiskLimit:                 int64(1024),
						ExcludeBaseImageFromQuota: true,
						BaseImage:                 imageConfig,
					})
					Expect(err).NotTo(HaveOccurred())
					_, spec := fakeImageDriver.CreateImageArgsForCall(0)
					Expect(spec.ExclusiveDiskLimit).To(BeTrue())
				})
			})
		})
	})

	Describe("Destroy", func() {
		var imagePath, imageRootFSPath string

		BeforeEach(func() {
			imagePath = path.Join(storePath, store.ImageDirName, "some-id")
			imageRootFSPath = path.Join(imagePath, "rootfs")
			Expect(os.MkdirAll(imagePath, 0755)).To(Succeed())
			Expect(os.MkdirAll(imageRootFSPath, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		})

		It("deletes an existing image", func() {
			Expect(imageCloner.Destroy(logger, "some-id")).To(Succeed())
			Expect(imagePath).NotTo(BeAnExistingFile())
		})

		Context("when image does not exist", func() {
			It("returns an error", func() {
				err := imageCloner.Destroy(logger, "cake")
				Expect(err).To(MatchError(ContainSubstring("image not found")))
			})
		})

		It("calls the image driver to remove the image", func() {
			err := imageCloner.Destroy(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())

			_, path := fakeImageDriver.DestroyImageArgsForCall(0)
			Expect(path).To(Equal(imagePath))
		})

		Context("when the image driver fails", func() {
			BeforeEach(func() {
				fakeImageDriver.DestroyImageReturns(errors.New("failed"))
			})

			It("doesnt fail", func() {
				err := imageCloner.Destroy(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())
			})

			It("still tries to delete the image path", func() {
				err := imageCloner.Destroy(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(imagePath).NotTo(BeAnExistingFile())
			})

			Context("when removing the image path also fails", func() {
				var mntPoint string

				JustBeforeEach(func() {
					mntPoint = filepath.Join(imagePath, "mnt")
					Expect(os.Mkdir(mntPoint, 0700)).To(Succeed())
					Expect(syscall.Mount(mntPoint, mntPoint, "none", syscall.MS_BIND, "")).To(Succeed())
				})

				AfterEach(func() {
					Expect(syscall.Unmount(mntPoint, syscall.MNT_DETACH)).To(Succeed())
				})

				It("returns an error", func() {
					err := imageCloner.Destroy(logger, "some-id")
					Expect(err).To(MatchError(ContainSubstring("deleting image path")))
				})
			})
		})
	})

	Describe("ImageIDs", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(filepath.Join(imagesPath, "image-a"), 0777)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(imagesPath, "image-b"), 0777)).To(Succeed())
		})

		It("returns a list with all known images", func() {
			images, err := imageCloner.ImageIDs(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(images).To(ConsistOf("image-a", "image-b"))
		})
	})

	Describe("Exists", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(filepath.Join(imagesPath, "some-id"), 0777)).To(Succeed())
		})

		It("returns true when image exists", func() {
			ok, err := imageCloner.Exists("some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		Context("when image does not exist", func() {
			It("returns false", func() {
				ok, err := imageCloner.Exists("invalid-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})
	})

	Describe("Stats", func() {
		var (
			imagePath       string
			imageRootFSPath string
			stats           groot.VolumeStats
		)

		BeforeEach(func() {
			stats = groot.VolumeStats{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     int64(1024),
					ExclusiveBytesUsed: int64(1024),
				},
			}

			imagePath = path.Join(storePath, store.ImageDirName, "some-id")
			imageRootFSPath = path.Join(imagePath, "rootfs")
			Expect(os.MkdirAll(imagePath, 0755)).To(Succeed())
			Expect(os.MkdirAll(imageRootFSPath, 0755)).To(Succeed())
		})

		It("fetches the stats", func() {
			_, err := imageCloner.Stats(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeImageDriver.FetchStatsCallCount()).To(Equal(1))
			_, receivedImagePath := fakeImageDriver.FetchStatsArgsForCall(0)
			Expect(receivedImagePath).To(Equal(imagePath))
		})

		It("returns the stats", func() {
			fakeImageDriver.FetchStatsReturns(stats, nil)

			m, err := imageCloner.Stats(logger, "some-id")

			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(stats))
		})

		Context("when image does not exist", func() {
			It("returns an error", func() {
				_, err := imageCloner.Stats(logger, "cake")
				Expect(err).To(MatchError(ContainSubstring("image not found")))
			})
		})

		Context("when the image driver fails", func() {
			It("returns an error", func() {
				fakeImageDriver.FetchStatsReturns(groot.VolumeStats{}, errors.New("failed"))

				_, err := imageCloner.Stats(logger, "some-id")
				Expect(err).To(MatchError("failed"))
			})
		})
	})
})
