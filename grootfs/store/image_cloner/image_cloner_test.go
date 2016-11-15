package image_cloner_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
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
		imageCloner *imageClonerpkg.ImageCloner

		fakeSnapshotDriver *image_clonerfakes.FakeSnapshotDriver
	)

	BeforeEach(func() {
		var err error
		fakeSnapshotDriver = new(image_clonerfakes.FakeSnapshotDriver)

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		imagesPath = filepath.Join(storePath, "images")

		Expect(os.Mkdir(imagesPath, 0777)).To(Succeed())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-bunlder")
		imageCloner = imageClonerpkg.NewImageCloner(fakeSnapshotDriver, storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("Create", func() {
		It("returns a image directory", func() {
			image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
			Expect(err).NotTo(HaveOccurred())
			Expect(image.Path).To(BeADirectory())
		})

		It("keeps the images in the same image directory", func() {
			someImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
			Expect(err).NotTo(HaveOccurred())
			anotherImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "another-id"})
			Expect(err).NotTo(HaveOccurred())

			Expect(someImage.Path).NotTo(BeEmpty())
			Expect(anotherImage.Path).NotTo(BeEmpty())

			images, err := ioutil.ReadDir(path.Join(storePath, store.IMAGES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(images)).To(Equal(2))
		})

		It("creates the snapshot", func() {
			imageSpec := groot.ImageSpec{
				ID:         "some-id",
				VolumePath: "/path/to/volume",
				BaseImage: specsv1.Image{
					Author: "Groot",
				},
			}
			image, err := imageCloner.Create(logger, imageSpec)
			Expect(err).NotTo(HaveOccurred())

			_, fromPath, toPath := fakeSnapshotDriver.SnapshotArgsForCall(0)
			Expect(fromPath).To(Equal(imageSpec.VolumePath))
			Expect(toPath).To(Equal(image.RootFSPath))
		})

		It("writes the image.json to the image", func() {
			baseImage := specsv1.Image{
				Author: "Groot",
				Config: specsv1.ImageConfig{
					User: "groot",
				},
			}

			image, err := imageCloner.Create(logger, groot.ImageSpec{
				ID:         "some-id",
				VolumePath: "/path/to/volume",
				BaseImage:  baseImage,
			})
			Expect(err).NotTo(HaveOccurred())

			imageJsonPath := filepath.Join(image.Path, "image.json")
			Expect(imageJsonPath).To(BeAnExistingFile())

			imageJsonFile, err := os.Open(imageJsonPath)
			Expect(err).NotTo(HaveOccurred())

			var imageJsonContent specsv1.Image
			Expect(json.NewDecoder(imageJsonFile).Decode(&imageJsonContent)).To(Succeed())
			Expect(imageJsonContent).To(Equal(baseImage))
		})

		Context("when calling it with two different ids", func() {
			It("returns two different image paths", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
				Expect(err).NotTo(HaveOccurred())

				anotherImage, err := imageCloner.Create(logger, groot.ImageSpec{ID: "another-id"})
				Expect(err).NotTo(HaveOccurred())

				Expect(image.Path).NotTo(Equal(anotherImage.Path))
			})
		})

		Context("when the store path does not exist", func() {
			BeforeEach(func() {
				storePath = "/non/existing/store"
			})

			It("should return an error", func() {
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("making image path")))
			})
		})

		Context("when creating the snapshot fails", func() {
			BeforeEach(func() {
				fakeSnapshotDriver.SnapshotReturns(errors.New("failed to create snapshot"))
			})

			It("returns an error", func() {
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("failed to create snapshot")))
			})

			It("removes the image", func() {
				imageID := "some-id"
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: imageID})
				Expect(err).To(HaveOccurred())
				Expect(filepath.Join(imagesPath, imageID)).NotTo(BeADirectory())
			})
		})

		Context("when writting the image.json fails", func() {
			BeforeEach(func() {
				imageClonerpkg.OF = func(name string, flag int, perm os.FileMode) (*os.File, error) {
					return nil, errors.New("permission denied: can't write stuff")
				}
			})

			AfterEach(func() {
				// needs to reasign the correct method after running the test
				imageClonerpkg.OF = os.OpenFile
			})

			It("returns an error", func() {
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("permission denied: can't write stuff")))
			})

			It("removes the image", func() {
				imageID := "some-id"
				_, err := imageCloner.Create(logger, groot.ImageSpec{ID: imageID})
				Expect(err).To(HaveOccurred())
				Expect(filepath.Join(imagesPath, imageID)).NotTo(BeADirectory())
			})
		})

		Context("when a disk limit is set", func() {
			It("applies the disk limit", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
				})
				Expect(err).NotTo(HaveOccurred())

				_, path, diskLimit, excludeBaseImageFromQuota := fakeSnapshotDriver.ApplyDiskLimitArgsForCall(0)
				Expect(path).To(Equal(image.RootFSPath))
				Expect(diskLimit).To(Equal(int64(1024)))
				Expect(excludeBaseImageFromQuota).To(BeFalse())
			})

			Context("when applying the disk limit fails", func() {
				BeforeEach(func() {
					fakeSnapshotDriver.ApplyDiskLimitReturns(errors.New("failed to apply disk limit"))
				})

				It("returns an error", func() {
					_, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:        "some-id",
						DiskLimit: int64(1024),
					})

					Expect(err).To(MatchError(ContainSubstring("failed to apply disk limit")))
				})

				It("removes the snapshot", func() {
					_, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:        "some-id",
						DiskLimit: int64(1024),
					})
					Expect(err).To(HaveOccurred())
					Expect(fakeSnapshotDriver.DestroyCallCount()).To(Equal(1))
					_, imagePath := fakeSnapshotDriver.DestroyArgsForCall(0)
					Expect(imagePath).To(Equal(imagePath))
				})
			})

			Context("when the exclusive flag is set", func() {
				It("enforces the exclusive limit", func() {
					_, err := imageCloner.Create(logger, groot.ImageSpec{
						ID:                        "some-id",
						DiskLimit:                 int64(1024),
						ExcludeBaseImageFromQuota: true,
					})
					Expect(err).NotTo(HaveOccurred())
					_, _, _, excludeBaseImageFromQuota := fakeSnapshotDriver.ApplyDiskLimitArgsForCall(0)
					Expect(excludeBaseImageFromQuota).To(BeTrue())
				})
			})
		})
	})

	Describe("Destroy", func() {
		var imagePath, imageRootFSPath string

		BeforeEach(func() {
			imagePath = path.Join(storePath, store.IMAGES_DIR_NAME, "some-id")
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

		Context("when deleting the folder fails", func() {
			BeforeEach(func() {
				Expect(os.Chmod(imagePath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				// we need to revert permissions because of the outer AfterEach
				Expect(os.Chmod(imagePath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := imageCloner.Destroy(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("deleting image path")))
			})
		})

		It("removes the snapshot", func() {
			err := imageCloner.Destroy(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())

			_, path := fakeSnapshotDriver.DestroyArgsForCall(0)
			Expect(path).To(Equal(imageRootFSPath))
		})

		Context("when removing the snapshot fails", func() {
			BeforeEach(func() {
				fakeSnapshotDriver.DestroyReturns(errors.New("failed to remove snapshot"))
			})

			It("returns an error", func() {
				err := imageCloner.Destroy(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("failed to remove snapshot")))
			})
		})
	})

	Describe("ImageIDs", func() {
		createImage := func(name string, layers []string) {
			imagePath := filepath.Join(imagesPath, name)
			Expect(os.Mkdir(imagePath, 0777)).To(Succeed())
			l := struct {
				Layers []string `json:"layers"`
			}{
				Layers: layers,
			}
			imageJson, err := json.Marshal(l)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(imagePath, "image.json"), imageJson, 0644)).To(Succeed())
		}

		BeforeEach(func() {
			createImage("image-a", []string{"sha-1", "sha-2"})
			createImage("image-b", []string{"sha-1", "sha-3", "sha-4"})
		})

		It("returns a list with all known images", func() {
			images, err := imageCloner.ImageIDs(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(images).To(ConsistOf("image-a", "image-b"))
		})

		Context("when fails to list images", func() {
			BeforeEach(func() {
				Expect(os.Chmod(imagesPath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				// we need to revert permissions because of the outer AfterEach
				Expect(os.Chmod(imagesPath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := imageCloner.ImageIDs(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to read images dir")))
			})
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

			Context("when it does not have permission to check", func() {
				JustBeforeEach(func() {
					basePath, err := ioutil.TempDir("", "")
					Expect(err).NotTo(HaveOccurred())

					noPermStorePath := filepath.Join(basePath, "no-perm-dir")
					Expect(os.Mkdir(noPermStorePath, 0000)).To(Succeed())

					imageCloner = imageClonerpkg.NewImageCloner(fakeSnapshotDriver, noPermStorePath)
				})

				It("returns an error", func() {
					ok, err := imageCloner.Exists("invalid-id")
					Expect(err).To(MatchError(ContainSubstring("stat")))
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Metrics", func() {
		var (
			imagePath, imageRootFSPath string
			metrics                    groot.VolumeMetrics
		)

		BeforeEach(func() {
			metrics = groot.VolumeMetrics{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     int64(1024),
					ExclusiveBytesUsed: int64(1024),
				},
			}

			imagePath = path.Join(storePath, store.IMAGES_DIR_NAME, "some-id")
			imageRootFSPath = path.Join(imagePath, "rootfs")
			Expect(os.MkdirAll(imagePath, 0755)).To(Succeed())
			Expect(os.MkdirAll(imageRootFSPath, 0755)).To(Succeed())
		})

		It("fetches the metrics", func() {
			_, err := imageCloner.Metrics(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeSnapshotDriver.FetchMetricsCallCount()).To(Equal(1))
		})

		It("returns the metrics", func() {
			fakeSnapshotDriver.FetchMetricsReturns(metrics, nil)

			m, err := imageCloner.Metrics(logger, "some-id")

			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(metrics))
		})

		Context("when image does not exist", func() {
			It("returns an error", func() {
				_, err := imageCloner.Metrics(logger, "cake")
				Expect(err).To(MatchError(ContainSubstring("image not found")))
			})
		})

		Context("when the snapshot driver fails", func() {
			It("returns an error", func() {
				fakeSnapshotDriver.FetchMetricsReturns(groot.VolumeMetrics{}, errors.New("failed"))

				_, err := imageCloner.Metrics(logger, "some-id")
				Expect(err).To(MatchError("failed"))
			})
		})
	})
})
