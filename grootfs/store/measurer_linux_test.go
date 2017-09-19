package store_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/storefakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Measurer", func() {
	var (
		storePath     string
		storeMeasurer *store.StoreMeasurer
		logger        *lagertest.TestLogger
		volumeDriver  *storefakes.FakeVolumeDriver
	)

	BeforeEach(func() {
		mountPoint := fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
		var err error
		storePath, err = ioutil.TempDir(mountPoint, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.VolumesDirName), 0744,
		)).To(Succeed())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.ImageDirName), 0744,
		)).To(Succeed())

		volumeDriver = new(storefakes.FakeVolumeDriver)

		storeMeasurer = store.NewStoreMeasurer(storePath, volumeDriver)

		logger = lagertest.NewTestLogger("store-measurer")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("Usage", func() {
		It("measures space used by the volumes and images", func() {
			volPath := filepath.Join(storePath, store.VolumesDirName, "sha256:fake")
			Expect(os.MkdirAll(volPath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(volPath, "my-file"), 2048*1024)).To(Succeed())

			imagePath := filepath.Join(storePath, store.ImageDirName, "my-image")
			Expect(os.MkdirAll(imagePath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(imagePath, "my-file"), 2048*1024)).To(Succeed())

			storeSize, err := storeMeasurer.Usage(logger)
			Expect(err).NotTo(HaveOccurred())
			xfsMetadataSize := 33755136
			Expect(storeSize).To(BeNumerically("~", 4096*1024+xfsMetadataSize, 256*1024))
		})

		Context("when the store does not exist", func() {
			BeforeEach(func() {
				storeMeasurer = store.NewStoreMeasurer("/path/to/non/existent/store", volumeDriver)
			})

			It("returns a useful error", func() {
				_, err := storeMeasurer.Usage(logger)
				Expect(err).To(MatchError(ContainSubstring("/path/to/non/existent/store")))
			})
		})
	})

	Describe("Size", func() {
		It("returns the size of the store", func() {
			size, err := storeMeasurer.Size(logger)
			Expect(err).NotTo(HaveOccurred())

			// size from ci/scripts/test/utils.sh -> 1GB
			truncatedFileSize := 1024 * 1024 * 1024
			fudgeFactor := truncatedFileSize / 100
			Expect(size).To(BeNumerically("~", truncatedFileSize, fudgeFactor))
		})
	})

	Describe("CommittedSize", func() {
		var (
			image2Path string
		)

		BeforeEach(func() {
			image1Path := filepath.Join(storePath, store.ImageDirName, "my-image-1")
			Expect(os.MkdirAll(image1Path, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(image1Path, "image_quota"), []byte("1024"), 0777)).To(Succeed())

			image2Path = filepath.Join(storePath, store.ImageDirName, "my-image-2")
			Expect(os.MkdirAll(image2Path, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(image2Path, "image_quota"), []byte("1024"), 0777)).To(Succeed())
		})

		It("returns the committed size of the store", func() {
			committedSize, err := storeMeasurer.CommittedSize(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(committedSize).To(BeNumerically("==", 2048))
		})

		Context("when the imageaDir cannot be read", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName))).To(Succeed())
			})

			It("errors", func() {
				_, err := storeMeasurer.CommittedSize(logger)
				Expect(err).To(MatchError(ContainSubstring("Cannot list images")))

			})
		})

		Context("when the image_quota cannot be parsed", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(image2Path, "image_quota"), []byte("hello"), 0777)).To(Succeed())
			})

			It("ignore that image quota", func() {
				committedSize, err := storeMeasurer.CommittedSize(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("WARNING-cannot-read-image-quota-ignoring")))

				Expect(committedSize).To(BeNumerically("==", 1024))
			})
		})

		Context("when the image_quota file isn't there cannot be parsed", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(image2Path, "image_quota"))).To(Succeed())
			})

			It("ignore that image quota", func() {
				committedSize, err := storeMeasurer.CommittedSize(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("WARNING-cannot-read-image-quota-ignoring")))

				Expect(committedSize).To(BeNumerically("==", 1024))
			})
		})
	})

	Describe("Cache", func() {
		BeforeEach(func() {
			volumeDriver.VolumesReturns([]string{"sha256:fake"}, nil)
			volumeDriver.VolumeSizeReturns(2048*1024, nil)
		})

		It("measures the size of the cache (everything in the store, except images)", func() {
			imagePath := filepath.Join(storePath, store.ImageDirName, "my-image")
			Expect(os.MkdirAll(imagePath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(imagePath, "my-file"), 2048*1024)).To(Succeed())

			metaDirPath := filepath.Join(storePath, store.MetaDirName)
			Expect(os.MkdirAll(metaDirPath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(metaDirPath, "my-file"), 2048*1024)).To(Succeed())

			tempDataPath := filepath.Join(storePath, store.TempDirName)
			Expect(os.MkdirAll(tempDataPath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(tempDataPath, "my-file"), 2048*1024)).To(Succeed())

			cacheSize, err := storeMeasurer.Cache(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumeDriver.VolumesCallCount()).To(Equal(1))
			Expect(volumeDriver.VolumeSizeCallCount()).To(Equal(1))
			_, volumeId := volumeDriver.VolumeSizeArgsForCall(0)
			Expect(volumeId).To(Equal("sha256:fake"), "checked size of unexpected volume")

			Expect(cacheSize).To(BeNumerically("~", 3*2048*1024, 1024))
		})

		Context("when the store does not exist", func() {
			BeforeEach(func() {
				storeMeasurer = store.NewStoreMeasurer("/path/to/non/existent/store", volumeDriver)
			})

			It("returns a useful error", func() {
				_, err := storeMeasurer.Cache(logger)
				Expect(err).To(MatchError(ContainSubstring("/path/to/non/existent/store")))
			})
		})
	})

	Describe("PurgeableCache", func() {
		var (
			unusedVolumes []string = []string{"sha256:fake1", "sha256:fake2"}
		)

		BeforeEach(func() {
			volumeDriver.VolumeSizeReturns(1024, nil)
		})

		It("measures the size of the unused layers", func() {
			purgeableCacheSize, err := storeMeasurer.PurgeableCache(logger, unusedVolumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(purgeableCacheSize).To(BeNumerically("==", 2048))
		})
	})
})

func writeFile(path string, size int64) error {
	cmd := exec.Command(
		"dd", "if=/dev/zero", fmt.Sprintf("of=%s", path),
		"bs=1024", fmt.Sprintf("count=%d", size/1024),
	)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	sess.Wait()

	out := sess.Buffer().Contents()
	exitCode := sess.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("du failed with exit code %d: %s", exitCode, string(out))
	}

	return nil
}
